package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

const (
	// apiDepth is the trigger depth of runs started by API requests.
	apiDepth        = 1
	maxBodySize     = 1 << 20
	maxSnapshotBody = 64 << 10
)

// Engine handles API requests, record triggers and schedule triggers of workflows, and runs jobs in the background.
type Engine struct {
	db      *gorm.DB
	hub     *collab.Hub
	redis   *redis.Client
	actions Actions

	specs     sync.Map
	specCount int64
	specMu    sync.Mutex

	workers         int
	wake            chan struct{}
	scheduleWake    chan struct{}
	startOnce       sync.Once
	stopMu          sync.Mutex
	stopClaims      context.CancelFunc
	cancelExecution context.CancelFunc
	wg              sync.WaitGroup
	done            chan struct{}
	releaseErr      error
}

// NewEngine returns an engine whose background jobs and schedule triggers run after Start; rate limiting is disabled when redis is nil.
func NewEngine(gormDB *gorm.DB, hub *collab.Hub, client *redis.Client, workers int) *Engine {
	workers = max(workers, 1)
	return &Engine{
		db:           gormDB,
		hub:          hub,
		redis:        client,
		actions:      &DefaultActions{},
		workers:      workers,
		wake:         make(chan struct{}, workers),
		scheduleWake: make(chan struct{}, 1),
		done:         make(chan struct{}),
	}
}

// SetActions replaces the implementation of external actions, for tests.
func (e *Engine) SetActions(actions Actions) {
	e.actions = actions
}

// PublishedSpec returns the Spec of a published revision; revisions are immutable and cached by ID.
func (e *Engine) PublishedSpec(ctx context.Context, revisionID int64) (*Spec, error) {
	if v, ok := e.specs.Load(revisionID); ok {
		return v.(*Spec), nil
	}
	revision, err := db.NewWorkflowRevisionsStore(e.db).GetByID(ctx, revisionID)
	if err != nil {
		return nil, err
	}
	spec, err := ParseSpec(revision.Spec)
	if err != nil {
		return nil, errors.Wrap(err, "parse spec")
	}
	e.specMu.Lock()
	if e.specCount > 1000 {
		e.specs.Range(func(k, _ interface{}) bool { e.specs.Delete(k); return true })
		e.specCount = 0
	}
	e.specCount++
	e.specMu.Unlock()
	e.specs.Store(revisionID, spec)
	return spec, nil
}

// RequestSnapshot is a bounded request snapshot saved in runs and events, without credentials.
type RequestSnapshot struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	IP      string            `json:"ip"`
	Query   map[string]string `json:"query"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body" swaggertype:"object"`
} // @name WorkflowRequestSnapshot

// sensitiveHeaders are not written to snapshots.
var sensitiveHeaders = map[string]bool{"authorization": true, "cookie": true, "x-api-key": true, "proxy-authorization": true}

// NewSnapshot builds a request snapshot; oversized request bodies are not saved.
func NewSnapshot(method, path, ip string, query map[string][]string, header http.Header, body []byte) RequestSnapshot {
	s := RequestSnapshot{Method: method, Path: path, IP: ip, Query: map[string]string{}, Headers: map[string]string{}}
	for k, v := range query {
		if len(v) > 0 {
			s.Query[k] = v[0]
		}
	}
	for k, v := range header {
		lower := strings.ToLower(k)
		if sensitiveHeaders[lower] || len(v) == 0 {
			continue
		}
		s.Headers[lower] = v[0]
	}
	if len(body) > 0 && len(body) <= maxSnapshotBody {
		var parsed interface{}
		if json.Unmarshal(body, &parsed) == nil {
			s.Body = parsed
		}
	}
	return s
}

// RunInput is the input of a run.
type RunInput struct {
	Project    *db.Project
	Workflow   *db.Workflow
	RevisionID int64
	Version    int
	Spec       *Spec
	Snapshot   RequestSnapshot
	Raw        RawInput
	Trigger    db.WorkflowRunTrigger
	UserID     int64
	// DryRun previews external actions and rolls back writes without saving the run or creating async jobs.
	DryRun bool
}

// PlannedJob is an async flow triggered by a run.
type PlannedJob struct {
	FlowID string `json:"flowID"`
	Name   string `json:"name"`
	Event  string `json:"event"`
	// Queued is false for a dry run, which only lists the flows that would be triggered.
	Queued bool `json:"queued"`
} // @name WorkflowPlannedJob

// RunResult is the result of a run.
type RunResult struct {
	RunUID     string               `json:"runUID"`
	Status     db.WorkflowRunStatus `json:"status" enums:"queued,succeeded,failed,rejected"`
	Response   *Response            `json:"response"`
	Trace      []TraceStep          `json:"trace"`
	Errors     []ParamError         `json:"errors"`
	Input      interface{}          `json:"input" swaggertype:"object"`
	Jobs       []PlannedJob         `json:"jobs"`
	DurationMs int64                `json:"durationMs"`
} // @name WorkflowRunResult

// ErrorBody returns a runtime error response body.
func ErrorBody(code, message string, details interface{}) map[string]interface{} {
	e := map[string]interface{}{"code": code, "message": message}
	if details != nil {
		e["details"] = details
	}
	return map[string]interface{}{"error": e}
}

// jobPayload is the event snapshot saved with a background job.
type jobPayload struct {
	ProjectID int64 `json:"projectID"`
	// Flow is the flow when debugging a draft; when empty, it is read from the job's revision.
	Flow  *AsyncFlow `json:"flow,omitempty"`
	Scope Scope      `json:"scope"`
	// Depth is the trigger depth of the run; writes made by the flow trigger other workflows with it.
	Depth int `json:"depth"`
}

func (e *Engine) newJobs(in *RunInput, run *db.WorkflowRun, event string, scope Scope) ([]*db.WorkflowJob, []PlannedJob, error) {
	var jobs []*db.WorkflowJob
	var planned []PlannedJob
	for i := range in.Spec.Async {
		flow := in.Spec.Async[i]
		if flow.Event != event || len(flow.Steps) == 0 {
			continue
		}
		planned = append(planned, PlannedJob{FlowID: flow.ID, Name: flow.Name, Event: flow.Event, Queued: !in.DryRun})
		if in.DryRun {
			continue
		}
		payload := jobPayload{ProjectID: in.Project.ID, Scope: scope, Depth: apiDepth}
		if in.RevisionID == 0 {
			payload.Flow = &flow
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, errors.Wrap(err, "encode payload")
		}
		jobs = append(jobs, &db.WorkflowJob{WorkflowID: in.Workflow.ID, RunID: run.ID, RevisionID: in.RevisionID, FlowID: flow.ID, Event: flow.Event, Payload: datatypes.JSON(raw)})
	}
	return jobs, planned, nil
}

func marshalJSON(v interface{}) []byte {
	raw, err := json.Marshal(v)
	if err != nil {
		return []byte("null")
	}
	return raw
}

// Run saves the run and the request received event, validates the parameters, then runs the synchronous flow and saves the result and the succeeded event in a single transaction.
func (e *Engine) Run(ctx context.Context, in RunInput) (*RunResult, error) {
	SyncPlugins(ctx, false)
	start := time.Now()
	result := &RunResult{Trace: []TraceStep{}, Errors: []ParamError{}, Jobs: []PlannedJob{}}
	runInfo := map[string]interface{}{"time": dbutil.Now().UTC().Format(time.RFC3339), "trigger": string(in.Trigger), "timezone": in.Spec.FormulaTimezone()}
	baseScope := Scope{"request": normalize(in.Snapshot), "run": runInfo}

	run := &db.WorkflowRun{
		WorkflowID: in.Workflow.ID, RevisionID: in.RevisionID, Version: in.Version, Trigger: in.Trigger, Status: db.WorkflowRunRunning,
		IP: in.Snapshot.IP, UserID: in.UserID, Request: marshalJSON(in.Snapshot),
	}
	if in.DryRun {
		run.UID = "dry"
	} else {
		if err := run.BeforeCreate(nil); err != nil {
			return nil, err
		}
	}
	runInfo["id"] = run.UID
	result.RunUID = run.UID

	if !in.DryRun {
		err := e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := db.NewWorkflowRunsStore(tx).Create(ctx, run); err != nil {
				return err
			}
			jobs, planned, err := e.newJobs(&in, run, EventRequestReceived, baseScope)
			if err != nil {
				return err
			}
			result.Jobs = append(result.Jobs, planned...)
			return db.NewWorkflowJobsStore(tx).Create(ctx, jobs)
		})
		if err != nil {
			return nil, err
		}
		if len(result.Jobs) > 0 {
			e.notifyWorkers()
		}
	} else {
		_, planned, _ := e.newJobs(&in, run, EventRequestReceived, baseScope)
		result.Jobs = append(result.Jobs, planned...)
	}

	finish := func(status db.WorkflowRunStatus, resp *Response, errBody interface{}) {
		result.Status = status
		result.Response = resp
		result.DurationMs = time.Since(start).Milliseconds()
		if in.DryRun || status == db.WorkflowRunSucceeded {
			return
		}
		if err := db.NewWorkflowRunsStore(e.db).Finish(context.WithoutCancel(ctx), run.ID, db.FinishWorkflowRunOptions{
			Status: status, StatusCode: resp.Status, DurationMs: result.DurationMs,
			Response: marshalJSON(resp), Trace: marshalJSON(result.Trace), Error: marshalJSON(errBody),
		}); err != nil {
			logrus.WithContext(ctx).WithError(err).Error("Failed to finish workflow run")
		}
	}

	input, paramErrs := ValidateParams(in.Spec.Params(), in.Raw)
	result.Input = input
	if len(paramErrs) > 0 {
		result.Errors = paramErrs
		body := ErrorBody("invalid_params", paramErrs[0].Message, paramErrs)
		resp := renderErrorResponse(in.Spec.Trigger.API.ParamErrorResponse, paramErrorScope(paramErrs, run.UID))
		if resp == nil {
			resp = &Response{Status: http.StatusBadRequest, Headers: map[string]string{}, Body: body}
		}
		finish(db.WorkflowRunRejected, resp, body["error"])
		return result, nil
	}

	tables := make([]*db.SLTable, 0)
	for _, uid := range in.Spec.WriteTables() {
		table, err := db.NewSLTablesStore(e.db).GetByUID(ctx, uid)
		if err == nil && table.ProjectID != in.Project.ID {
			err = db.ErrSLTableNotFound
		}
		if err != nil {
			if !errors.Is(err, db.ErrSLTableNotFound) {
				return nil, err
			}
			body := ErrorBody(ErrCodeConfig, "a table written by the API does not exist", nil)
			finish(db.WorkflowRunFailed, &Response{Status: http.StatusInternalServerError, Headers: map[string]string{}, Body: body}, body["error"])
			return result, nil
		}
		tables = append(tables, table)
	}

	scope := Scope{"input": input, "request": baseScope["request"], "run": runInfo, "steps": map[string]interface{}{}}
	var resp *Response
	origin := collab.Origin{WorkflowID: in.Workflow.ID, Depth: apiDepth}
	err := e.hub.Batch(ctx, in.Project, tables, collab.BatchOptions{DryRun: in.DryRun, Origin: origin}, func(b *collab.Batch) error {
		// External plugin calls are awaited within the transaction; their effects cannot be rolled back, so this batch must not be retried.
		x := &execution{ctx: ctx, db: e.db, hub: e.hub, project: in.Project, scope: scope, tables: map[string]*tableInfo{}, batch: b, origin: origin, preview: in.DryRun}
		if !in.DryRun {
			x.actions = e.actions
		}

		stop, err := x.runSteps(in.Spec.Steps)
		result.Trace = x.trace
		if err != nil {
			return err
		}
		if !stop || x.response == nil {
			return &StepError{Code: ErrCodeNoResponse, Status: http.StatusInternalServerError, Err: errors.New("the flow ended without a response")}
		}
		resp = x.response

		eventScope := Scope{"input": input, "request": scope["request"], "run": runInfo, "steps": scope["steps"], "response": normalize(resp)}
		jobs, planned, err := e.newJobs(&in, run, EventSucceeded, eventScope)
		if err != nil {
			return err
		}
		result.Jobs = append(result.Jobs, planned...)
		if in.DryRun {
			return nil
		}
		result.DurationMs = time.Since(start).Milliseconds()
		if err := db.NewWorkflowRunsStore(b.Tx()).Finish(ctx, run.ID, db.FinishWorkflowRunOptions{
			Status: db.WorkflowRunSucceeded, StatusCode: resp.Status, DurationMs: result.DurationMs,
			Response: marshalJSON(resp), Trace: marshalJSON(result.Trace),
		}); err != nil {
			return errors.Wrap(err, "finish run")
		}
		return db.NewWorkflowJobsStore(b.Tx()).Create(ctx, jobs)
	})
	if err != nil {
		var stepErr *StepError
		if !errors.As(err, &stepErr) {
			logrus.WithContext(ctx).WithError(err).WithField("workflow", in.Workflow.UID).Error("Failed to run workflow")
			stepErr = &StepError{Code: ErrCodeInternal, Status: http.StatusInternalServerError, Err: err}
		}
		// After the rollback the planned succeeded events are no longer valid.
		result.Jobs = filterJobs(result.Jobs, EventRequestReceived)
		message := publicError(stepErr)
		if stepErr.Code == ErrCodeNoResponse {
			message = "the flow ended without a response"
		}
		body := ErrorBody(stepErr.Code, message, map[string]string{"step": stepErr.StepID})
		finish(db.WorkflowRunFailed, &Response{Status: stepErr.Status, Headers: map[string]string{}, Body: body}, body["error"])
		return result, nil
	}

	finish(db.WorkflowRunSucceeded, resp, nil)
	if !in.DryRun && len(result.Jobs) > 0 {
		e.notifyWorkers()
	}
	return result, nil
}

func filterJobs(jobs []PlannedJob, event string) []PlannedJob {
	out := []PlannedJob{}
	for _, j := range jobs {
		if j.Event == event {
			out = append(out, j)
		}
	}
	return out
}

// WriteResponse writes a JSON response.
func WriteResponse(w http.ResponseWriter, resp *Response, runUID string) {
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	if runUID != "" {
		w.Header().Set("X-Request-Id", runUID)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(resp.Status)
	if resp.Status == http.StatusNoContent || resp.Status == http.StatusNotModified {
		return
	}
	_ = json.NewEncoder(w).Encode(resp.Body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	WriteResponse(w, &Response{Status: status, Body: ErrorBody(code, message, nil)}, "")
}

func writeSecurityError(w http.ResponseWriter, response *ErrorResponse, status int, code, message string) {
	if response != nil && response.validStatus() && json.Valid([]byte(response.Body)) {
		WriteResponse(w, &Response{Status: response.Status, Body: json.RawMessage(response.Body)}, "")
		return
	}

	writeError(w, status, code, message)
}

func apiKeyOf(r *http.Request) string {
	if key := r.Header.Get("X-API-Key"); key != "" {
		return key
	}
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

// AllowRate counts requests in a fixed window and allows them when Redis is unavailable.
func (e *Engine) AllowRate(ctx context.Context, apiID int64, ip string, rl RateLimit) (bool, int) {
	if e.redis == nil || !rl.Enabled {
		return true, 0
	}
	window := int64(rl.Window)
	now := time.Now().Unix()
	key := "api_rate:" + strconv.FormatInt(apiID, 10) + ":" + ip + ":" + strconv.FormatInt(now/window, 10)
	// INCR and EXPIRE run in one transaction so that a counter never lives without a TTL.
	var incr *redis.IntCmd
	if _, err := e.redis.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		incr = pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, time.Duration(window)*time.Second)
		return nil
	}); err != nil {
		logrus.WithContext(ctx).WithError(err).Warn("Failed to count api rate")
		return true, 0
	}
	return incr.Val() <= int64(rl.Limit), int(window - now%window)
}

// Serve handles requests to /api/{projectUID}/{path}, i.e. API-triggered workflows.
func (e *Engine) Serve(w http.ResponseWriter, r *http.Request, projectUID, rawPath, ip string) {
	ctx := r.Context()
	path, ok := NormalizePath(rawPath)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "no API matches the request")
		return
	}
	project, err := db.NewProjectsStore(e.db).GetByUID(ctx, projectUID)
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "no API matches the request")
			return
		}
		logrus.WithContext(ctx).WithError(err).Error("Failed to get project")
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		return
	}
	wf, err := db.NewWorkflowsStore(e.db).GetByRoute(ctx, project.ID, r.Method, path)
	if err != nil {
		if errors.Is(err, db.ErrWorkflowNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "no API matches the request")
			return
		}
		logrus.WithContext(ctx).WithError(err).Error("Failed to get workflow")
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		return
	}
	if !wf.Enabled {
		writeError(w, http.StatusServiceUnavailable, "api_disabled", "the API is disabled")
		return
	}
	spec, err := e.PublishedSpec(ctx, wf.PublishedRevisionID)
	if err != nil || !spec.IsAPI() {
		logrus.WithContext(ctx).WithError(err).Error("Failed to load workflow revision")
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		return
	}

	if spec.Trigger.API.Auth.Type == AuthAPIKey {
		ok, err := db.NewWorkflowKeysStore(e.db).Verify(ctx, wf.ID, apiKeyOf(r))
		if err != nil {
			logrus.WithContext(ctx).WithError(err).Error("Failed to verify api key")
			writeError(w, http.StatusInternalServerError, ErrCodeInternal, "internal error")
			return
		}
		if !ok {
			writeSecurityError(w, spec.Trigger.API.Auth.ErrorResponse, http.StatusUnauthorized, "unauthorized", "a valid API key is required")
			return
		}
	}
	if allowed, retry := e.AllowRate(ctx, wf.ID, ip, spec.Trigger.API.RateLimit); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(retry))
		writeSecurityError(w, spec.Trigger.API.RateLimit.ErrorResponse, http.StatusTooManyRequests, "rate_limited", "too many requests")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "failed to read the request body")
		return
	}
	if len(body) > maxBodySize {
		writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "the request body is too large")
		return
	}
	raw := RawInput{Query: r.URL.Query(), Header: r.Header}
	if len(bytes.TrimSpace(body)) > 0 {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(body, &obj); err != nil || obj == nil {
			const message = "the request body must be a JSON object"
			errs := []ParamError{{Path: SourceBody, Code: CodeInvalidBody, Message: message}}
			if resp := renderErrorResponse(spec.Trigger.API.ParamErrorResponse, paramErrorScope(errs, "")); resp != nil {
				WriteResponse(w, resp, "")
				return
			}
			writeError(w, http.StatusBadRequest, CodeInvalidBody, message)
			return
		}
		raw.Body = obj
	}

	in := RunInput{
		Project: project, Workflow: wf, RevisionID: wf.PublishedRevisionID, Version: wf.PublishedVersion, Spec: spec,
		Snapshot: NewSnapshot(r.Method, path, ip, r.URL.Query(), r.Header, body), Raw: raw, Trigger: db.WorkflowRunTriggerHTTP,
	}
	result, err := e.Run(ctx, in)
	if err != nil {
		logrus.WithContext(ctx).WithError(err).Error("Failed to run workflow")
		writeError(w, http.StatusInternalServerError, ErrCodeInternal, "internal error")
		return
	}
	WriteResponse(w, result.Response, result.RunUID)
}

// DB returns the database connection used by the engine.
func (e *Engine) DB() *gorm.DB {
	return e.db
}
