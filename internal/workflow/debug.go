package workflow

import (
	"context"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

// ErrNoSampleRecord means the table has no record to use when debugging a record-triggered workflow.
var ErrNoSampleRecord = errors.New("no record to debug with")

// SampleTrigger returns the trigger variables for debugging: record and date triggers use the given record, or the latest record if none is given.
func (e *Engine) SampleTrigger(ctx context.Context, project *db.Project, spec *Spec, recordUID string) (map[string]interface{}, error) {
	now := dbutil.Now().UTC().Format(time.RFC3339)
	if spec.Trigger.Type == TriggerSchedule {
		return map[string]interface{}{"time": now}, nil
	}
	r := spec.Trigger.Record
	if r == nil {
		return nil, ErrNoSampleRecord
	}
	table, err := db.NewSLTablesStore(e.db).GetByUID(ctx, r.TableUID)
	if err != nil || table.ProjectID != project.ID {
		if err == nil || errors.Is(err, db.ErrSLTableNotFound) {
			return nil, ErrNoSampleRecord
		}
		return nil, err
	}
	fields, err := db.NewSLFieldsStore(e.db).ListByTableID(ctx, table.ID)
	if err != nil {
		return nil, err
	}
	info := newTableInfo(table, fields)

	records := db.NewSLRecordsStore(e.db)
	var record *db.SLRecord
	if recordUID != "" {
		record, err = records.GetByUID(ctx, recordUID)
		if err == nil && record.SLTableID != table.ID {
			err = db.ErrSLRecordNotFound
		}
	} else {
		var list []*db.SLRecord
		if list, _, err = records.Query(ctx, table.ID, db.QuerySLRecordsOptions{Limit: 1}); err == nil {
			if len(list) == 0 {
				err = db.ErrSLRecordNotFound
			} else {
				record = list[0]
			}
		}
	}
	if err != nil {
		if errors.Is(err, db.ErrSLRecordNotFound) {
			return nil, ErrNoSampleRecord
		}
		return nil, err
	}

	data := map[string]interface{}{"record": recordOutput(info, record), "table": tableData(table)}
	switch spec.Trigger.Type {
	case TriggerRecordUpdated:
		data["previous"], data["changedFields"] = data["record"], []interface{}{}
	case TriggerRecordMatched:
		data["previous"] = nil
	case TriggerDateReached:
		data["time"] = now
		if t, ok := recordTime(record, r.DateFieldUID); ok {
			data["time"] = t.Add(time.Duration(r.Offset) * time.Minute).UTC().Format(time.RFC3339)
		}
	}
	return normalize(data).(map[string]interface{}), nil
}

// DebugInput is the input for debugging a workflow that is not triggered by an API.
type DebugInput struct {
	Project  *db.Project
	Workflow *db.Workflow
	Spec     *Spec
	Trigger  map[string]interface{}
	UserID   int64
	// DryRun runs the main flow in a rolled-back transaction and only previews emails and HTTP requests; otherwise a real background run is enqueued.
	DryRun bool
}

// Debug debugs the draft of a workflow that is not triggered by an API.
func (e *Engine) Debug(ctx context.Context, in DebugInput) (*RunResult, error) {
	start := time.Now()
	result := &RunResult{Trace: []TraceStep{}, Errors: []ParamError{}, Jobs: []PlannedJob{}, Input: in.Trigger}
	if !in.DryRun {
		runs, err := e.enqueue(ctx, e.db, enqueueOptions{
			ProjectID: in.Project.ID, Workflow: in.Workflow, TriggerType: in.Spec.Trigger.Type, Timezone: in.Spec.FormulaTimezone(), Source: db.WorkflowRunTriggerDebug,
			Depth: 1, Triggers: []map[string]interface{}{in.Trigger}, Flow: &AsyncFlow{ID: MainFlow, Steps: in.Spec.Steps}, UserID: in.UserID,
		})
		if err != nil {
			return nil, err
		}
		e.notifyWorkers()
		result.RunUID, result.Status = runs[0].UID, db.WorkflowRunQueued
		return result, nil
	}

	tables := make([]*db.SLTable, 0)
	for _, uid := range in.Spec.WriteTables() {
		table, err := db.NewSLTablesStore(e.db).GetByUID(ctx, uid)
		if err == nil && table.ProjectID == in.Project.ID {
			tables = append(tables, table)
		} else if err != nil && !errors.Is(err, db.ErrSLTableNotFound) {
			return nil, err
		}
	}
	scope := Scope{
		"trigger": in.Trigger,
		"run":     map[string]interface{}{"id": "dry", "time": dbutil.Now().UTC().Format(time.RFC3339), "trigger": string(db.WorkflowRunTriggerDebug), "timezone": in.Spec.FormulaTimezone()},
		"steps":   map[string]interface{}{},
	}
	result.RunUID = "dry"
	origin := collab.Origin{WorkflowID: in.Workflow.ID, Depth: 1}
	err := e.hub.Batch(ctx, in.Project, tables, collab.BatchOptions{DryRun: true, Origin: origin}, func(b *collab.Batch) error {
		x := &execution{ctx: ctx, db: e.db, hub: e.hub, project: in.Project, scope: scope, tables: map[string]*tableInfo{}, batch: b, origin: origin, preview: true}
		_, err := x.runSteps(in.Spec.Steps)
		result.Trace = x.trace
		return err
	})
	result.DurationMs = time.Since(start).Milliseconds()
	result.Status = db.WorkflowRunSucceeded
	if err != nil {
		var stepErr *StepError
		if !errors.As(err, &stepErr) {
			logrus.WithContext(ctx).WithError(err).WithField("workflow", in.Workflow.UID).Error("Failed to debug workflow")
			stepErr = &StepError{Code: ErrCodeInternal, Status: http.StatusInternalServerError, Err: err}
		}
		result.Status = db.WorkflowRunFailed
		body := ErrorBody(stepErr.Code, publicError(stepErr), map[string]string{"step": stepErr.StepID})
		result.Response = &Response{Status: stepErr.Status, Headers: map[string]string{}, Body: body}
	}
	return result, nil
}
