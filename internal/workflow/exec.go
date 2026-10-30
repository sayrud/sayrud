package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

// Step trace statuses.
const (
	TraceSucceeded = "succeeded"
	TraceFailed    = "failed"
	TraceSkipped   = "skipped"
	TracePlanned   = "planned"
)

// TraceStep is the execution result of a single step.
type TraceStep struct {
	StepID     string `json:"stepID"`
	Type       string `json:"type"`
	Name       string `json:"name,omitempty"`
	Status     string `json:"status" enums:"succeeded,failed,skipped,planned"`
	DurationMs int64  `json:"durationMs"`
	// Branch is the branch a branching node entered, then or else.
	Branch string `json:"branch,omitempty"`
	// Output is the truncated step output.
	Output json.RawMessage `json:"output,omitempty" swaggertype:"object"`
	Error  string          `json:"error,omitempty"`
	// Logs are the lines printed by a plugin node.
	Logs []string `json:"logs,omitempty"`
	// Reused means an async retry reused the output of the previous successful attempt.
	Reused bool `json:"reused,omitempty"`
} // @name WorkflowTraceStep

// Response is the HTTP response built by a sync flow.
type Response struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body" swaggertype:"object"`
} // @name WorkflowResponse

// StepError is a step execution failure; Status is the response code for sync requests.
type StepError struct {
	StepID string
	Code   string
	Status int
	Err    error
	// Transient means the async job may be retried.
	Transient bool
	// Uncertain means the outcome of an external action is unknown, so it is not retried automatically.
	Uncertain bool
}

func (e *StepError) Error() string {
	return fmt.Sprintf("step %s: %v", e.StepID, e.Err)
}

func (e *StepError) Unwrap() error {
	return e.Err
}

// Error codes of sync failures.
const (
	ErrCodeInvalidFieldValue = "invalid_field_value"
	ErrCodeRecordNotFound    = "record_not_found"
	ErrCodeNoResponse        = "no_response"
	ErrCodeConfig            = "invalid_config"
	ErrCodeInternal          = "internal_error"
	ErrCodeAction            = "action_failed"
	ErrCodeFormula           = "formula_error"
)

// maxTraceOutput is the number of bytes of a step's output kept in the trace.
const maxTraceOutput = 4 << 10

func traceOutput(v interface{}) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	if len(raw) > maxTraceOutput {
		truncated, _ := json.Marshal(map[string]interface{}{"truncated": true, "preview": string(raw[:maxTraceOutput])})
		return truncated
	}
	return raw
}

// Actions performs operations with external side effects.
type Actions interface {
	// SendEmail sends with the SMTP server resolved from the credential referenced by the step.
	SendEmail(ctx context.Context, server SMTPServer, mail Mail) error
	Fetch(ctx context.Context, req HTTPRequest) (*HTTPResult, error)
}

// execution is the state of a single flow execution.
type execution struct {
	ctx     context.Context
	db      *gorm.DB
	hub     *collab.Hub
	project *db.Project
	scope   Scope
	trace   []TraceStep
	tables  map[string]*tableInfo

	// batch is the shared transaction of a sync flow with its tables locked in advance; it is nil for background flows, where each write step opens its own transaction.
	batch *collab.Batch
	// origin is the source of the flow's writes, used to trigger other workflows.
	origin collab.Origin
	// actions is nil when external actions must not run, e.g. in simulated debugging.
	actions Actions
	// preview marks simulated debugging: external actions only compute their parameters and are not executed.
	preview bool
	// done holds the outputs of steps already completed in an async retry, keyed by step ID.
	done map[string]json.RawMessage
	// persist saves an async step's output after it succeeds; when tx is non-nil, it must save within that transaction.
	persist func(tx *gorm.DB, stepID string, output interface{}) error

	response *Response
	// logs are the lines printed by the running plugin step, moved into its trace.
	logs []string
}

func (x *execution) addTrace(step *Step, status string, start time.Time, output interface{}, err error) {
	t := TraceStep{StepID: step.ID, Type: step.Type, Name: step.Name, Status: status, DurationMs: time.Since(start).Milliseconds()}
	if output != nil {
		t.Output = traceOutput(output)
	}
	if err != nil {
		t.Error = publicError(err)
	}
	t.Logs, x.logs = x.logs, nil
	x.trace = append(x.trace, t)
}

// publicError returns an error description that is safe to show to the workflow author.
func publicError(err error) string {
	var fieldErr *FieldValueError
	if errors.As(err, &fieldErr) {
		return fieldErr.Error()
	}
	var stepErr *StepError
	if errors.As(err, &stepErr) {
		err = stepErr.Err
	}
	switch {
	case errors.Is(err, routeutil.ErrFieldTypeMismatch):
		return "the value mismatches the field type"
	case errors.Is(err, routeutil.ErrInvalidAttachment):
		return "invalid attachment reference"
	case errors.Is(err, db.ErrSLRecordNotFound):
		return "the record does not exist"
	case errors.Is(err, db.ErrSLFieldNotFound):
		return "a referenced field does not exist"
	case errors.Is(err, errTableNotFound):
		return "the table does not exist"
	}
	var formulaErr *FormulaError
	if errors.As(err, &formulaErr) {
		return "formula error: " + formulaErr.Message
	}
	var actionErr *ActionError
	if errors.As(err, &actionErr) {
		return actionErr.Message
	}
	return "internal error"
}

var errTableNotFound = errors.New("table not found")

// table returns a table in the project; a table that does not belong to the project is treated as nonexistent.
func (x *execution) table(uid string) (*tableInfo, error) {
	if info, ok := x.tables[uid]; ok {
		return info, nil
	}
	table, err := db.NewSLTablesStore(x.db).GetByUID(x.ctx, uid)
	if err != nil {
		if errors.Is(err, db.ErrSLTableNotFound) {
			return nil, errTableNotFound
		}
		return nil, err
	}
	if table.ProjectID != x.project.ID {
		return nil, errTableNotFound
	}
	var fields []*db.SLField
	if x.batch != nil {
		if fields, err = x.batch.Fields(table); errors.Is(err, collab.ErrBatchTableNotLocked) {
			fields, err = db.NewSLFieldsStore(x.batch.Tx()).ListByTableID(x.ctx, table.ID)
		}
	} else {
		fields, err = db.NewSLFieldsStore(x.db).ListByTableID(x.ctx, table.ID)
	}
	if err != nil {
		return nil, err
	}
	info := newTableInfo(table, fields)
	x.tables[uid] = info
	return info, nil
}

// reader returns the connection for reading records: sync flows read from the shared transaction so they see writes from earlier steps.
func (x *execution) reader() *gorm.DB {
	if x.batch != nil {
		return x.batch.Tx()
	}
	return x.db
}

// runSteps runs the steps in order; stop is true once a response node is reached.
func (x *execution) runSteps(steps []Step) (bool, error) {
	for i := range steps {
		stop, err := x.runStep(&steps[i])
		if err != nil || stop {
			return stop, err
		}
	}
	return false, nil
}

func (x *execution) runStep(step *Step) (bool, error) {
	start := time.Now()
	if raw, ok := x.done[step.ID]; ok && step.Type != StepCondition {
		var output interface{}
		_ = json.Unmarshal(raw, &output)
		branch, err := pluginBranch(step, output)
		if err != nil {
			x.addTrace(step, TraceFailed, start, nil, err)
			return false, classify(step, err)
		}

		x.scope.SetStep(step.ID, output)
		x.trace = append(x.trace, TraceStep{StepID: step.ID, Type: step.Type, Name: step.Name, Status: TraceSucceeded, Output: traceOutput(output), Reused: true, Branch: branch})
		return x.runBranch(step, branch)
	}

	switch step.Type {
	case StepCondition:
		matched, err := x.evalConditions(step)
		if err != nil {
			x.addTrace(step, TraceFailed, start, nil, err)
			return false, &StepError{StepID: step.ID, Code: ErrCodeInternal, Status: http.StatusInternalServerError, Err: err}
		}
		branch, steps := "else", step.Else
		if matched {
			branch, steps = "then", step.Then
		}
		x.trace = append(x.trace, TraceStep{StepID: step.ID, Type: step.Type, Name: step.Name, Status: TraceSucceeded, Branch: branch, DurationMs: time.Since(start).Milliseconds()})
		return x.runSteps(steps)

	case StepResponse:
		resp, err := x.buildResponse(step)
		if err != nil {
			x.addTrace(step, TraceFailed, start, nil, err)
			return false, &StepError{StepID: step.ID, Code: ErrCodeInternal, Status: http.StatusInternalServerError, Err: err}
		}
		x.response = resp
		x.addTrace(step, TraceSucceeded, start, resp, nil)
		return true, nil
	}

	impl, ok := moduleImpls[step.Type]
	if !ok {
		impl, ok = pluginImpl(step.Type)
	}
	if !ok || impl.exec == nil {
		err := &StepError{StepID: step.ID, Code: ErrCodeConfig, Status: http.StatusInternalServerError, Err: errors.Newf("unsupported step type %q", step.Type)}
		x.addTrace(step, TraceFailed, start, nil, err)
		return false, err
	}

	if x.preview && impl.preview != nil {
		output, err := impl.preview(x, step)
		if err != nil {
			x.addTrace(step, TraceFailed, start, nil, err)
			return false, classify(step, err)
		}
		output = normalize(output)
		branch, err := pluginBranch(step, output)
		if err != nil {
			x.addTrace(step, TraceFailed, start, nil, err)
			return false, classify(step, err)
		}

		x.scope.SetStep(step.ID, output)
		x.addTrace(step, TracePlanned, start, output, nil)
		x.trace[len(x.trace)-1].Branch = branch
		return x.runBranch(step, branch)
	}

	output, err := impl.exec(x, step)
	persisted := impl.ownTx && x.batch == nil
	if err != nil {
		x.addTrace(step, TraceFailed, start, nil, err)
		return false, classify(step, err)
	}

	output = normalize(output)
	branch, err := pluginBranch(step, output)
	if err != nil {
		x.addTrace(step, TraceFailed, start, nil, err)
		return false, classify(step, err)
	}

	if x.persist != nil && !persisted {
		if err := x.persist(nil, step.ID, output); err != nil {
			x.addTrace(step, TraceFailed, start, nil, err)
			return false, &StepError{StepID: step.ID, Code: ErrCodeInternal, Status: http.StatusInternalServerError, Err: err, Transient: true}
		}
	}
	x.scope.SetStep(step.ID, output)
	x.addTrace(step, TraceSucceeded, start, output, nil)
	x.trace[len(x.trace)-1].Branch = branch
	return x.runBranch(step, branch)
}

// classify maps a step error to a response code and whether it is retryable.
func classify(step *Step, err error) error {
	var stepErr *StepError
	if errors.As(err, &stepErr) {
		if stepErr.StepID == "" {
			stepErr.StepID = step.ID
		}
		return stepErr
	}
	var fieldErr *FieldValueError
	switch {
	case errors.As(err, &fieldErr), errors.Is(err, routeutil.ErrFieldTypeMismatch), errors.Is(err, routeutil.ErrInvalidAttachment):
		return &StepError{StepID: step.ID, Code: ErrCodeInvalidFieldValue, Status: http.StatusUnprocessableEntity, Err: err}
	case errors.Is(err, db.ErrSLRecordNotFound):
		return &StepError{StepID: step.ID, Code: ErrCodeRecordNotFound, Status: http.StatusNotFound, Err: err}
	case errors.Is(err, errTableNotFound), errors.Is(err, db.ErrSLFieldNotFound), errors.Is(err, db.ErrUnsupportedFilterOperation),
		errors.Is(err, db.ErrInvalidFilterValue), errors.Is(err, db.ErrSLFieldNotQueryable):
		return &StepError{StepID: step.ID, Code: ErrCodeConfig, Status: http.StatusInternalServerError, Err: err}
	}
	var formulaErr *FormulaError
	if errors.As(err, &formulaErr) {
		return &StepError{StepID: step.ID, Code: ErrCodeFormula, Status: http.StatusUnprocessableEntity, Err: err}
	}
	var actionErr *ActionError
	if errors.As(err, &actionErr) {
		return &StepError{StepID: step.ID, Code: ErrCodeAction, Status: http.StatusBadGateway, Err: err, Transient: actionErr.Transient, Uncertain: actionErr.Uncertain}
	}
	return &StepError{StepID: step.ID, Code: ErrCodeInternal, Status: http.StatusInternalServerError, Err: err, Transient: true}
}

func (x *execution) query(step *Step) (interface{}, error) {
	cfg := configOf[QueryConfig](step)
	info, err := x.table(cfg.TableUID)
	if err != nil {
		return nil, err
	}
	filters, err := buildFilters(info, x.scope, cfg.Filters)
	if err != nil {
		return nil, err
	}
	limit := cfg.Limit
	if limit <= 0 {
		limit = DefaultQueryLimit
	}
	limit = min(limit, MaxQueryLimit)
	sorts := make([]db.QuerySLRecordsSort, 0, len(cfg.Sort))
	for _, s := range cfg.Sort {
		sorts = append(sorts, db.QuerySLRecordsSort{FieldUID: s.FieldUID, Order: s.Order})
	}

	records, total, err := db.NewSLRecordsStore(x.reader()).Query(x.ctx, info.table.ID, db.QuerySLRecordsOptions{Filter: filters, MatchAny: cfg.Match == "any", Order: sorts, Limit: limit})
	if err != nil {
		return nil, err
	}
	list := make([]interface{}, 0, len(records))
	for _, r := range records {
		list = append(list, recordOutput(info, r, x.scope.formulaLocation()))
	}
	var first interface{}
	if len(list) > 0 {
		first = list[0]
	}
	return map[string]interface{}{"records": list, "first": first, "count": len(list), "total": total, "found": len(list) > 0}, nil
}

// cellValues computes the field values mapped by the step, keyed by field UID; nil means clear the field.
func (x *execution) cellValues(info *tableInfo, cfg *WriteConfig) (map[string]interface{}, error) {
	values := make(map[string]interface{}, len(cfg.Fields))
	for i := range cfg.Fields {
		fv := &cfg.Fields[i]
		field, ok := info.byUID[fv.FieldUID]
		if !ok {
			return nil, errors.Wrapf(db.ErrSLFieldNotFound, "field %q", fv.FieldUID)
		}
		if !WritableField(field) {
			return nil, &FieldValueError{Field: field.Label, Message: "is not writable"}
		}
		v, err := x.scope.Resolve(&fv.Value)
		if err != nil {
			return nil, err
		}
		cell, err := toCellValue(field, v)
		if err != nil {
			return nil, err
		}
		values[field.UID] = cell
	}
	return values, nil
}

func (x *execution) write(step *Step) (interface{}, error) {
	cfg := configOf[WriteConfig](step)
	info, err := x.table(cfg.TableUID)
	if err != nil {
		return nil, err
	}
	values, err := x.cellValues(info, cfg)
	if err != nil {
		return nil, err
	}
	var recordUID string
	if step.Type == StepUpdate {
		v, err := x.scope.Resolve(cfg.RecordUID)
		if err != nil {
			return nil, err
		}
		recordUID = Text(v)
		if recordUID == "" {
			return nil, db.ErrSLRecordNotFound
		}
	}

	apply := func(b *collab.Batch) (interface{}, error) {
		var record *db.SLRecord
		var err error
		if step.Type == StepCreate {
			info.applyDefaults(values)
			record, err = b.CreateRecord(info.table, values)
		} else {
			record, err = b.UpdateRecord(info.table, recordUID, func(current map[string]interface{}) map[string]interface{} {
				for k, v := range values {
					current[k] = v
				}
				return current
			})
		}
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{"record": recordOutput(info, record, x.scope.formulaLocation())}, nil
	}

	if x.batch != nil {
		return apply(x.batch)
	}
	var output interface{}
	err = x.hub.Batch(x.ctx, x.project, []*db.SLTable{info.table}, collab.BatchOptions{Origin: x.origin}, func(b *collab.Batch) error {
		out, err := apply(b)
		if err != nil {
			return err
		}
		output = normalize(out)
		if x.persist != nil {
			return x.persist(b.Tx(), step.ID, output)
		}
		return nil
	})
	return output, err
}

func (x *execution) evalConditions(step *Step) (bool, error) {
	cfg := configOf[ConditionConfig](step)
	if len(cfg.Conditions) == 0 {
		return true, nil
	}
	matchAny := cfg.Match == "any"
	for i := range cfg.Conditions {
		c := &cfg.Conditions[i]
		left, err := x.scope.Resolve(&c.Left)
		if err != nil {
			return false, err
		}
		right, err := x.scope.Resolve(c.Right)
		if err != nil {
			return false, err
		}
		ok := compare(left, c.Op, right)
		if matchAny && ok {
			return true, nil
		}
		if !matchAny && !ok {
			return false, nil
		}
	}
	return !matchAny, nil
}

func isEmpty(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []interface{}:
		return len(t) == 0
	case map[string]interface{}:
		return len(t) == 0
	case bool:
		return !t
	}
	return false
}

func compare(left interface{}, op string, right interface{}) bool {
	switch op {
	case "empty":
		return isEmpty(left)
	case "not_empty":
		return !isEmpty(left)
	case "eq":
		return equal(left, right)
	case "neq":
		return !equal(left, right)
	case "contains", "not_contains":
		var found bool
		switch l := left.(type) {
		case string:
			found = strings.Contains(l, Text(right))
		case []interface{}:
			for _, item := range l {
				if equal(item, right) {
					found = true
					break
				}
			}
		}
		return found == (op == "contains")
	case "gt", "gte", "lt", "lte":
		if left == nil || right == nil {
			return false
		}
		// Numbers and numeric strings compare numerically, other values compare as text, e.g. RFC 3339 times.
		var c int
		lf, lok := toFloat(left)
		rf, rok := toFloat(right)
		if lok && rok {
			c = cmpFloat(lf, rf)
		} else {
			c = strings.Compare(Text(left), Text(right))
		}
		switch op {
		case "gt":
			return c > 0
		case "gte":
			return c >= 0
		case "lt":
			return c < 0
		default:
			return c <= 0
		}
	}
	return false
}

// toFloat converts a number or a string that is entirely a finite number, rejecting strings with trailing text such as "12abc".
func toFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, false
		}
		return f, true
	}
	return 0, false
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// equal compares values: a number equals its numeric string and a boolean equals "true"/"false", to ease comparison with constants.
func equal(a, b interface{}) bool {
	if a == nil || b == nil {
		return (a == nil || a == "") && (b == nil || b == "")
	}
	if af, ok := a.(float64); ok {
		if bf, ok := toFloat(b); ok {
			return af == bf
		}
	}
	if bf, ok := b.(float64); ok {
		if af, ok := toFloat(a); ok {
			return af == bf
		}
	}
	return Text(a) == Text(b)
}

func (x *execution) buildResponse(step *Step) (*Response, error) {
	cfg := configOf[ResponseConfig](step)
	status := cfg.Status
	if status == 0 {
		status = http.StatusOK
	}
	body, err := x.scope.BuildBody(cfg.Body)
	if err != nil {
		return nil, err
	}
	// Response headers cannot be set by the flow.
	return &Response{Status: status, Headers: map[string]string{}, Body: body}, nil
}
