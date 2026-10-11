package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
)

// Issue is a problem in the configuration; Code is translated by the frontend, and Target points to the offending object: trigger, route, auth, rate_limit, param:<id>, step:<id> or async:<id>.
type Issue struct {
	Code   string            `json:"code"`
	Target string            `json:"target"`
	Args   map[string]string `json:"args,omitempty"`
} // @name WorkflowIssue

// Methods lists the supported request methods.
var Methods = []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

var (
	pathSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9._~\-]+$`)
	paramKeyPattern    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_\-]{0,63}$`)
	stepIDPattern      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,31}$`)
)

// Configuration size limits.
const (
	maxParams = 100
	maxSteps  = 100
	maxPath   = 200
	// maxDateOffset is the maximum offset of a date trigger in minutes, i.e. one year.
	maxDateOffset = 366 * 24 * 60
)

// NormalizePath returns the normalized static path: it starts with /, has no trailing /, and each segment contains only URL-safe characters.
func NormalizePath(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimRight(path, "/")
	if path == "" || len(path) > maxPath {
		return "", false
	}
	for _, seg := range strings.Split(path[1:], "/") {
		if seg == "" || seg == "." || seg == ".." || !pathSegmentPattern.MatchString(seg) {
			return "", false
		}
	}
	return path, true
}

// ValidMethod reports whether the request method is supported.
func ValidMethod(method string) bool {
	return contains(Methods, method)
}

// filterOps lists the filter operators supported by each field type.
var filterOps = map[db.SLFieldType][]string{
	db.TextFieldType:         {"eq", "neq", "like", "in", "nin", "empty", "not_empty"},
	db.NumberFieldType:       {"eq", "neq", "gt", "gte", "lt", "lte", "in", "nin", "empty", "not_empty"},
	db.SingleSelectFieldType: {"eq", "neq", "in", "nin", "empty", "not_empty"},
	db.MultiSelectFieldType:  {"eq", "neq", "in", "nin", "empty", "not_empty"},
	db.DateTimeFieldType:     {"eq", "neq", "gt", "gte", "lt", "lte", "empty", "not_empty"},
	db.CheckboxFieldType:     {"eq", "neq"},
	db.AttachmentFieldType:   {"empty", "not_empty"},
}

var conditionOps = []string{"eq", "neq", "gt", "gte", "lt", "lte", "contains", "not_contains", "empty", "not_empty"}

// Flow execution modes.
const (
	// flowSync is the main flow of an API trigger: it runs synchronously, every path must return a response, and it cannot perform external actions.
	flowSync = iota
	// flowAsync is an async flow of an API trigger: it runs in the background and cannot return a response.
	flowAsync
	// flowBackground is the main flow of other triggers: it runs in the background and cannot return a response.
	flowBackground
)

type checker struct {
	ctx     context.Context
	db      *gorm.DB
	project *db.Project
	spec    *Spec
	issues  []Issue
	tables  map[string]*tableInfo
	stepIDs map[string]*Step
	// inputKeys are the declared parameter names; input.<key> may only reference them.
	inputKeys map[string]bool
}

func (c *checker) add(code, target string, args ...string) {
	issue := Issue{Code: code, Target: target}
	if len(args) > 0 {
		issue.Args = map[string]string{}
		for i := 0; i+1 < len(args); i += 2 {
			issue.Args[args[i]] = args[i+1]
		}
	}
	c.issues = append(c.issues, issue)
}

func (c *checker) table(uid string) (*tableInfo, error) {
	if info, ok := c.tables[uid]; ok {
		return info, nil
	}
	table, err := db.NewSLTablesStore(c.db).GetByUID(c.ctx, uid)
	if err != nil && !errors.Is(err, db.ErrSLTableNotFound) {
		return nil, err
	}
	if err != nil || table.ProjectID != c.project.ID {
		c.tables[uid] = nil
		return nil, nil
	}
	fields, err := db.NewSLFieldsStore(c.db).ListByTableID(c.ctx, table.ID)
	if err != nil {
		return nil, err
	}
	info := newTableInfo(table, fields)
	c.tables[uid] = info
	return info, nil
}

// Check returns the issues that must be fixed before publishing, or an empty list if there are none.
func Check(ctx context.Context, gormDB *gorm.DB, project *db.Project, spec *Spec) ([]Issue, error) {
	c := &checker{ctx: ctx, db: gormDB, project: project, spec: spec, tables: map[string]*tableInfo{}, stepIDs: map[string]*Step{}, inputKeys: map[string]bool{}}
	if err := c.checkTrigger(); err != nil {
		return nil, err
	}

	total := 0
	spec.AllSteps(func(step *Step) {
		total++
		if !stepIDPattern.MatchString(step.ID) {
			c.add("step.invalid_id", "step:"+step.ID)
		} else if _, ok := c.stepIDs[step.ID]; ok {
			c.add("step.duplicate_id", "step:"+step.ID, "id", step.ID)
		}
		c.stepIDs[step.ID] = step
	})
	if total > maxSteps {
		c.add("flow.too_many_steps", "flow", "max", strconv.Itoa(maxSteps))
	}

	if !spec.IsAPI() {
		if err := c.checkFlow(spec.Steps, c.baseScope(false), flowBackground); err != nil {
			return nil, err
		}
		if len(spec.Steps) == 0 {
			c.add("flow.empty", "flow")
		}
		return c.result(), nil
	}

	if err := c.checkFlow(spec.Steps, c.baseScope(false), flowSync); err != nil {
		return nil, err
	}
	flowIDs := map[string]bool{}
	for i := range spec.Async {
		flow := &spec.Async[i]
		target := "async:" + flow.ID
		if flow.ID == "" || flow.ID == MainFlow || flowIDs[flow.ID] {
			c.add("async.invalid_id", target)
		}
		flowIDs[flow.ID] = true
		if flow.Event != EventRequestReceived && flow.Event != EventSucceeded {
			c.add("async.invalid_event", target)
		}
		if len(flow.Steps) == 0 {
			c.add("async.empty", target)
		}
		scope := c.baseScope(flow.Event == EventRequestReceived)
		if flow.Event == EventSucceeded {
			// The succeeded event may reference all steps of the sync flow and its response.
			WalkSteps(spec.Steps, func(step *Step) { scope.steps[step.ID] = step })
			scope.response = true
		}
		if err := c.checkFlow(flow.Steps, scope, flowAsync); err != nil {
			return nil, err
		}
	}
	return c.result(), nil
}

func (c *checker) result() []Issue {
	if c.issues == nil {
		return []Issue{}
	}
	return c.issues
}

func (c *checker) checkTrigger() error {
	t := &c.spec.Trigger
	switch {
	case t.Type == TriggerAPI:
		c.checkRoute()
		c.checkParams()
	case IsRecordTrigger(t.Type), t.Type == TriggerDateReached:
		return c.checkRecordTrigger()
	case t.Type == TriggerSchedule:
		if code := checkSchedule(t.Schedule); code != "" {
			c.add(code, "trigger")
		}
	case t.Type == "":
		c.add("trigger.unset", "trigger")
	default:
		c.add("trigger.invalid_type", "trigger")
	}
	return nil
}

// triggerTable returns the table watched by record and date triggers, or nil if it does not exist.
func (c *checker) triggerTable() (*tableInfo, error) {
	r := c.spec.Trigger.Record
	if r == nil || r.TableUID == "" {
		return nil, nil
	}
	return c.table(r.TableUID)
}

func (c *checker) checkRecordTrigger() error {
	t := &c.spec.Trigger
	r := t.Record
	info, err := c.triggerTable()
	if err != nil {
		return err
	}
	if info == nil {
		c.add("trigger.table_missing", "trigger")
		return nil
	}
	if t.Type == TriggerRecordUpdated {
		for _, uid := range r.Fields {
			if _, ok := info.byUID[uid]; !ok {
				c.add("trigger.field_missing", "trigger")
			}
		}
	}
	if t.Type == TriggerDateReached {
		field, ok := info.byUID[r.DateFieldUID]
		if !ok || field.Type != db.DateTimeFieldType {
			c.add("trigger.date_field_missing", "trigger")
		}
		if r.Offset < -maxDateOffset || r.Offset > maxDateOffset {
			c.add("trigger.invalid_offset", "trigger")
		}
	}
	if t.Type == TriggerRecordMatched && len(r.Filters) == 0 {
		c.add("trigger.no_conditions", "trigger")
	}
	if r.Match != "" && r.Match != "all" && r.Match != "any" {
		c.add("trigger.invalid_condition", "trigger")
	}
	if t.Type == TriggerRecordDeleted && len(r.Filters) > 0 {
		c.add("trigger.invalid_condition", "trigger")
	}
	for i := range r.Filters {
		f := &r.Filters[i]
		field, ok := info.byUID[f.FieldUID]
		if !ok {
			c.add("trigger.field_missing", "trigger")
			continue
		}
		if !contains(filterOps[field.Type], f.Op) {
			c.add("step.invalid_operator", "trigger", "field", field.Label)
			continue
		}
		if f.Value == nil || f.Op == "empty" || f.Op == "not_empty" {
			continue
		}
		if f.Value.Kind != ValueConst {
			c.add("trigger.invalid_condition", "trigger")
			continue
		}
		if _, err := buildFilters(info, Scope{}, []Filter{*f}); err != nil {
			c.add("trigger.invalid_condition", "trigger")
		}
	}
	return nil
}

func (c *checker) checkRoute() {
	a := c.spec.Trigger.API
	if !ValidMethod(a.Method) {
		c.add("route.invalid_method", "route")
	}
	if _, ok := NormalizePath(a.Path); !ok {
		c.add("route.invalid_path", "route")
	}
	if a.Auth.Type != AuthNone && a.Auth.Type != AuthAPIKey {
		c.add("auth.invalid_type", "auth")
	}
	if rl := a.RateLimit; rl.Enabled && (rl.Limit < 1 || rl.Limit > 100000 || rl.Window < 1 || rl.Window > 86400) {
		c.add("rate_limit.invalid", "rate_limit")
	}
}

var paramTypes = []string{TypeString, TypeNumber, TypeInteger, TypeBoolean, TypeDate, TypeDateTime, TypeEnum, TypeArray}

func (c *checker) checkParams() {
	a := c.spec.Trigger.API
	if len(a.Params) > maxParams {
		c.add("param.too_many", "params", "max", strconv.Itoa(maxParams))
	}
	ids := map[string]bool{}
	for i := range a.Params {
		p := &a.Params[i]
		target := "param:" + p.ID
		if p.ID == "" || ids[p.ID] {
			c.add("param.invalid_id", target)
		}
		ids[p.ID] = true
		if !paramKeyPattern.MatchString(p.Key) {
			c.add("param.invalid_key", target, "key", p.Key)
		} else if c.inputKeys[p.Key] {
			c.add("param.duplicate_key", target, "key", p.Key)
		}
		c.inputKeys[p.Key] = true
		if p.Source != SourceQuery && p.Source != SourceHeader && p.Source != SourceBody {
			c.add("param.invalid_source", target, "key", p.Key)
		}
		if p.Source == SourceBody && a.Method == http.MethodGet {
			c.add("param.body_not_allowed", target, "key", p.Key)
		}
		if !contains(paramTypes, p.Type) {
			c.add("param.invalid_type", target, "key", p.Key)
			continue
		}
		if p.Type == TypeArray && p.ItemType != TypeString && p.ItemType != TypeNumber {
			c.add("param.invalid_type", target, "key", p.Key)
		}
		if p.Type == TypeEnum && len(p.Options) == 0 {
			c.add("param.enum_options", target, "key", p.Key)
		}
		if p.Pattern != "" {
			if _, err := regexp.Compile(p.Pattern); err != nil {
				c.add("param.invalid_pattern", target, "key", p.Key)
			}
		}
		if p.MinLength != nil && p.MaxLength != nil && *p.MinLength > *p.MaxLength || p.Min != nil && p.Max != nil && *p.Min > *p.Max {
			c.add("param.invalid_range", target, "key", p.Key)
		}
		if len(p.Default) > 0 && string(p.Default) != "null" {
			v, err := decodeTyped(p, p.Default)
			if err == nil {
				err = checkRules(p, v)
			}
			if err != nil {
				c.add("param.invalid_default", target, "key", p.Key)
			}
		}
	}
}

// flowScope holds the variables available at a given position when checking variable references.
type flowScope struct {
	// requestOnly marks the request_received event, which may only reference request and run.
	requestOnly bool
	steps       map[string]*Step
	response    bool
	item        bool
}

func (c *checker) baseScope(requestOnly bool) *flowScope {
	return &flowScope{requestOnly: requestOnly, steps: map[string]*Step{}}
}

func (s *flowScope) clone() *flowScope {
	steps := make(map[string]*Step, len(s.steps))
	for k, v := range s.steps {
		steps[k] = v
	}
	return &flowScope{requestOnly: s.requestOnly, steps: steps, response: s.response, item: s.item}
}

// checkFlow checks a flow; every path of a sync flow must end with a response node.
func (c *checker) checkFlow(steps []Step, scope *flowScope, kind int) error {
	terminated, err := c.checkSequence(steps, scope, kind)
	if err != nil {
		return err
	}
	if kind == flowSync && !terminated {
		target := "flow"
		if len(steps) > 0 {
			target = "step:" + steps[len(steps)-1].ID
		}
		c.add("flow.missing_response", target)
	}
	return nil
}

// checkSequence checks a sequence of steps; terminated means every path has reached a response node.
// Branches that do not return a response continue after the condition node, and outputs of steps inside a branch cannot be referenced after the condition node.
func (c *checker) checkSequence(steps []Step, scope *flowScope, kind int) (bool, error) {
	for i := range steps {
		step := &steps[i]
		if err := c.checkStep(step, scope, kind); err != nil {
			return false, err
		}
		terminated := false
		switch step.Type {
		case StepResponse:
			terminated = kind == flowSync
		case StepCondition:
			thenDone, err := c.checkSequence(step.Then, scope.clone(), kind)
			if err != nil {
				return false, err
			}
			elseDone, err := c.checkSequence(step.Else, scope.clone(), kind)
			if err != nil {
				return false, err
			}
			terminated = thenDone && elseDone
		default:
			scope.steps[step.ID] = step
		}
		if terminated {
			if i < len(steps)-1 {
				c.add("step.unreachable", "step:"+steps[i+1].ID)
			}
			return true, nil
		}
	}
	return false, nil
}

func (c *checker) checkStep(step *Step, scope *flowScope, kind int) error {
	target := "step:" + step.ID
	switch step.Type {
	case StepQuery, StepCreate, StepUpdate:
		info, err := c.table(step.TableUID)
		if err != nil {
			return err
		}
		if info == nil {
			c.add("step.table_missing", target)
			break
		}
		if step.Type == StepQuery {
			if step.Limit < 0 || step.Limit > MaxQueryLimit {
				c.add("step.invalid_limit", target, "max", strconv.Itoa(MaxQueryLimit))
			}
			if step.Match != "" && step.Match != "all" && step.Match != "any" {
				c.add("step.invalid_match", target)
			}
			for _, f := range step.Filters {
				field, ok := info.byUID[f.FieldUID]
				if !ok {
					c.add("step.field_missing", target)
					continue
				}
				if !contains(filterOps[field.Type], f.Op) {
					c.add("step.invalid_operator", target, "field", field.Label)
				}
				c.checkValue(f.Value, scope, target)
			}
			for _, s := range step.Sort {
				field, ok := info.byUID[s.FieldUID]
				if !ok || field.Type == db.FormulaFieldType {
					c.add("step.field_missing", target)
				}
			}
			break
		}
		if len(step.Fields) == 0 {
			c.add("step.no_fields", target)
		}
		seen := map[string]bool{}
		for i := range step.Fields {
			fv := &step.Fields[i]
			field, ok := info.byUID[fv.FieldUID]
			if !ok {
				c.add("step.field_missing", target)
				continue
			}
			if !WritableField(field) {
				c.add("step.field_not_writable", target, "field", field.Label)
			}
			if seen[field.UID] {
				c.add("step.duplicate_field", target, "field", field.Label)
			}
			seen[field.UID] = true
			c.checkValue(&fv.Value, scope, target)
		}
		if step.Type == StepUpdate {
			if step.RecordUID == nil || step.RecordUID.Kind == "" {
				c.add("step.record_required", target)
			}
			c.checkValue(step.RecordUID, scope, target)
		}

	case StepCondition:
		if len(step.Conditions) == 0 {
			c.add("step.no_conditions", target)
		}
		for i := range step.Conditions {
			cond := &step.Conditions[i]
			if !contains(conditionOps, cond.Op) {
				c.add("step.invalid_operator", target)
			}
			c.checkValue(&cond.Left, scope, target)
			c.checkValue(cond.Right, scope, target)
		}

	case StepResponse:
		switch kind {
		case flowAsync:
			c.add("step.response_in_async", target)
		case flowBackground:
			c.add("step.response_api_only", target)
		}
		if step.Status < 100 || step.Status > 599 {
			c.add("step.invalid_status", target)
		}
		c.checkBody(step.Body, scope, target)

	case StepEmail:
		if kind == flowSync {
			c.add("step.async_only", target)
		}
		if step.To == nil || step.To.Kind == "" {
			c.add("step.email_to_required", target)
		}
		if step.Subject == nil || step.Subject.Kind == "" {
			c.add("step.email_subject_required", target)
		}
		c.checkValue(step.To, scope, target)
		c.checkValue(step.CC, scope, target)
		c.checkValue(step.Subject, scope, target)
		c.checkValue(step.Content, scope, target)

	case StepHTTP:
		if kind == flowSync {
			c.add("step.async_only", target)
		}
		if step.URL == nil || step.URL.Kind == "" {
			c.add("step.url_required", target)
		}
		if step.HTTPMethod != "" && !ValidMethod(strings.ToUpper(step.HTTPMethod)) {
			c.add("step.invalid_method", target)
		}
		if step.Timeout < 0 || step.Timeout > 30 {
			c.add("step.invalid_timeout", target)
		}
		c.checkValue(step.URL, scope, target)
		for i := range step.Headers {
			c.checkValue(&step.Headers[i].Value, scope, target)
		}
		c.checkBody(step.Body, scope, target)

	default:
		c.add("step.invalid_type", target)
	}
	return nil
}

func (c *checker) checkBody(b *Body, scope *flowScope, target string) {
	if b == nil {
		return
	}
	switch b.Type {
	case BodyObject:
		keys := map[string]bool{}
		for i := range b.Fields {
			f := &b.Fields[i]
			if f.Key == "" || keys[f.Key] {
				c.add("step.invalid_body_key", target, "key", f.Key)
			}
			keys[f.Key] = true
			c.checkBody(&f.Value, scope, target)
		}
	case BodyValue:
		c.checkValue(b.Value, scope, target)
	case BodyMap:
		c.checkValue(b.Source, scope, target)
		child := scope.clone()
		child.item = true
		c.checkBody(b.Item, child, target)
	default:
		c.add("step.invalid_body", target)
	}
}

func (c *checker) checkValue(v *Value, scope *flowScope, target string) {
	if v == nil {
		return
	}
	switch v.Kind {
	case ValueConst:
		if len(v.Const) > 0 && !json.Valid(v.Const) {
			c.add("value.invalid_const", target)
		}
	case ValueVar:
		c.checkVar(v.Var, scope, target)
	case ValueTemplate:
		for _, m := range TemplatePattern.FindAllStringSubmatch(v.Template, -1) {
			c.checkVar(m[1], scope, target)
		}
	case ValueFormula:
		if strings.TrimSpace(v.Formula) == "" {
			return
		}
		f, err := ParseFormula(v.Formula)
		if err != nil {
			c.add("value.invalid_formula", target)
			return
		}
		for _, ref := range f.Refs {
			c.checkVar(ref, scope, target)
		}
	case "":
	default:
		c.add("value.invalid_kind", target)
	}
}

// stepOutputs lists the output properties of each step type.
var stepOutputs = map[string][]string{
	StepQuery:  {"records", "first", "count", "total", "found"},
	StepCreate: {"record"},
	StepUpdate: {"record"},
	StepEmail:  {"to", "cc", "subject", "messageID"},
	StepHTTP:   {"status", "headers", "body"},
}

// TriggerOutputs lists the properties each trigger type provides through the trigger variable.
var TriggerOutputs = map[string][]string{
	TriggerRecordCreated: {"record", "table"},
	TriggerRecordUpdated: {"record", "previous", "changedFields", "table"},
	TriggerRecordMatched: {"record", "previous", "table"},
	TriggerRecordDeleted: {"record", "table"},
	TriggerDateReached:   {"record", "table", "time"},
	TriggerSchedule:      {"time"},
}

// checkFieldRef checks that a fields.<uid> reference in parts points to a field that still exists in the table.
func (c *checker) checkFieldRef(parts []string, from int, tableUID string) bool {
	for i := from; i+1 < len(parts); i++ {
		if parts[i] != "fields" {
			continue
		}
		if info, _ := c.table(tableUID); info != nil {
			if _, ok := info.byUID[parts[i+1]]; !ok {
				return false
			}
		}
		break
	}
	return true
}

func (c *checker) checkVar(path string, scope *flowScope, target string) {
	parts := strings.Split(path, ".")
	unknown := func() { c.add("value.unknown_var", target, "var", path) }
	api := c.spec.IsAPI()
	switch parts[0] {
	case "run":
		return
	case "request":
		if !api {
			unknown()
		}
		return
	case "trigger":
		if api || len(parts) < 2 || !contains(TriggerOutputs[c.spec.Trigger.Type], parts[1]) {
			unknown()
			return
		}
		if (parts[1] == "record" || parts[1] == "previous") && c.spec.Trigger.Record != nil && !c.checkFieldRef(parts, 2, c.spec.Trigger.Record.TableUID) {
			unknown()
		}
		return
	case "item":
		if !scope.item {
			unknown()
		}
		return
	case "response":
		if !scope.response {
			unknown()
		}
		return
	case "input":
		if !api || scope.requestOnly || len(parts) < 2 || !c.inputKeys[parts[1]] {
			unknown()
		}
		return
	case "steps":
		if scope.requestOnly || len(parts) < 2 {
			unknown()
			return
		}
		step, ok := scope.steps[parts[1]]
		if !ok {
			unknown()
			return
		}
		if len(parts) >= 3 && !contains(stepOutputs[step.Type], parts[2]) {
			unknown()
			return
		}
		if !c.checkFieldRef(parts, 3, step.TableUID) {
			unknown()
		}
		return
	}
	unknown()
}
