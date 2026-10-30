package workflow

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

const (
	// maxTriggerDepth is the maximum depth of triggers chained by workflow writes; beyond it, workflow writes no longer trigger other workflows.
	maxTriggerDepth = 3
	// maxTriggerRuns is the maximum number of runs enqueued per workflow by a single write.
	maxTriggerRuns = 1000
)

var _ collab.WorkflowHooks = (*Engine)(nil)

// recordTriggerTypes maps each kind of record change to the trigger types it may fire.
var recordTriggerTypes = map[string][]string{
	collab.RecordCreated: {TriggerRecordCreated, TriggerRecordMatched},
	collab.RecordUpdated: {TriggerRecordUpdated, TriggerRecordMatched},
	collab.RecordDeleted: {TriggerRecordDeleted},
}

// RecordsChanged enqueues, within the write transaction, the workflow runs triggered by record changes; a workflow is never triggered by its own writes.
func (e *Engine) RecordsChanged(ctx context.Context, tx *gorm.DB, project *db.Project, table *db.SLTable, origin collab.Origin, events []collab.RecordEvent) (int, error) {
	if origin.Depth >= maxTriggerDepth {
		return 0, nil
	}
	var types []string
	for _, ev := range events {
		types = append(types, recordTriggerTypes[ev.Kind]...)
	}
	list, err := db.NewWorkflowsStore(tx).ListByTriggerTable(ctx, table.ID, lo.Uniq(types))
	if err != nil {
		return 0, errors.Wrap(err, "list triggered workflows")
	}
	if len(list) == 0 {
		return 0, nil
	}
	fields, err := db.NewSLFieldsStore(tx).ListByTableID(ctx, table.ID)
	if err != nil {
		return 0, errors.Wrap(err, "list fields")
	}
	info := newTableInfo(table, fields)

	total := 0
	for _, wf := range list {
		if wf.ID == origin.WorkflowID {
			continue
		}
		log := logrus.WithContext(ctx).WithField("workflow", wf.UID)
		spec, err := e.PublishedSpec(ctx, wf.PublishedRevisionID)
		if err != nil {
			log.WithError(err).Error("Failed to load workflow revision")
			continue
		}
		var matched []collab.RecordEvent
		// Conditions are evaluated in a savepoint so that an invalid configuration only skips this workflow without affecting the write.
		if err := tx.Transaction(func(sp *gorm.DB) error {
			matched, err = matchRecordEvents(ctx, sp, info, &spec.Trigger, events)
			return err
		}); err != nil {
			log.WithError(err).Warn("Failed to match workflow trigger")
			continue
		}
		if len(matched) > maxTriggerRuns {
			log.WithField("count", len(matched)).Warn("Too many records trigger the workflow, the rest are skipped")
			matched = matched[:maxTriggerRuns]
		}
		triggers := make([]map[string]interface{}, 0, len(matched))
		for _, ev := range matched {
			triggers = append(triggers, recordTriggerData(info, spec.Trigger.Type, ev, formulaLocation(spec.FormulaTimezone())))
		}
		if _, err := e.enqueue(ctx, tx, enqueueOptions{
			ProjectID: project.ID, Workflow: wf, RevisionID: wf.PublishedRevisionID, Version: wf.PublishedVersion,
			TriggerType: spec.Trigger.Type, Timezone: spec.FormulaTimezone(), Depth: origin.Depth + 1, Triggers: triggers,
		}); err != nil {
			return 0, err
		}
		total += len(triggers)
	}
	return total, nil
}

// RunsQueued wakes up the workers to process newly enqueued runs.
func (e *Engine) RunsQueued() {
	e.notifyWorkers()
}

// matchRecordEvents returns the changes that satisfy the record trigger's type, watched fields and conditions; record_matched only fires when a record goes from not matching to matching the conditions.
func matchRecordEvents(ctx context.Context, tx *gorm.DB, info *tableInfo, t *Trigger, events []collab.RecordEvent) ([]collab.RecordEvent, error) {
	r := t.Record
	if r == nil {
		return nil, nil
	}
	var candidates []collab.RecordEvent
	for _, ev := range events {
		ok := false
		switch t.Type {
		case TriggerRecordCreated:
			ok = ev.Kind == collab.RecordCreated
		case TriggerRecordUpdated:
			ok = ev.Kind == collab.RecordUpdated && (len(r.Fields) == 0 || len(lo.Intersect(r.Fields, ev.Fields)) > 0)
		case TriggerRecordMatched:
			ok = ev.Kind == collab.RecordCreated || ev.Kind == collab.RecordUpdated
		case TriggerRecordDeleted:
			ok = ev.Kind == collab.RecordDeleted
		}
		if ok {
			candidates = append(candidates, ev)
		}
	}
	if len(candidates) == 0 || t.Type == TriggerRecordDeleted {
		return candidates, nil
	}
	filters, err := buildFilters(info, Scope{}, r.Filters)
	if err != nil {
		return nil, err
	}
	if len(filters) == 0 {
		if t.Type == TriggerRecordMatched {
			return nil, nil
		}
		return candidates, nil
	}

	records := db.NewSLRecordsStore(tx)
	after, before := map[string]json.RawMessage{}, map[string]json.RawMessage{}
	for i, ev := range candidates {
		key := strconv.Itoa(i)
		after[key] = json.RawMessage(ev.Record.Data)
		if t.Type == TriggerRecordMatched && ev.Kind == collab.RecordUpdated {
			before[key] = json.RawMessage(ev.Previous)
		}
	}
	matchAny := r.Match == "any"
	afterKeys, err := records.MatchData(ctx, info.table.ID, filters, matchAny, after)
	if err != nil {
		return nil, err
	}
	beforeKeys, err := records.MatchData(ctx, info.table.ID, filters, matchAny, before)
	if err != nil {
		return nil, err
	}
	out := make([]collab.RecordEvent, 0, len(afterKeys))
	for i, ev := range candidates {
		key := strconv.Itoa(i)
		if lo.Contains(afterKeys, key) && !lo.Contains(beforeKeys, key) {
			out = append(out, ev)
		}
	}
	return out, nil
}

// tableData returns the trigger.table variable.
func tableData(table *db.SLTable) map[string]interface{} {
	return map[string]interface{}{"uid": table.UID, "name": table.Name}
}

// recordTriggerData returns the trigger variables of a record trigger: record and table, plus previous and changedFields (field names) for updates.
func recordTriggerData(info *tableInfo, triggerType string, ev collab.RecordEvent, loc *time.Location) map[string]interface{} {
	data := map[string]interface{}{"record": recordOutput(info, ev.Record, loc), "table": tableData(info.table)}
	if ev.Kind == collab.RecordUpdated && (triggerType == TriggerRecordUpdated || triggerType == TriggerRecordMatched) {
		previous := *ev.Record
		previous.Data = ev.Previous
		data["previous"] = recordOutput(info, &previous, loc)
		if triggerType == TriggerRecordUpdated {
			changed := make([]interface{}, 0, len(ev.Fields))
			for _, uid := range ev.Fields {
				if f, ok := info.byUID[uid]; ok {
					changed = append(changed, f.Label)
				}
			}
			data["changedFields"] = changed
		}
	}
	return normalize(data).(map[string]interface{})
}

// enqueueOptions are the options for enqueuing background runs.
type enqueueOptions struct {
	ProjectID  int64
	Workflow   *db.Workflow
	RevisionID int64
	Version    int
	// TriggerType is the workflow's trigger type; Source is the run source and defaults to the trigger type when empty.
	TriggerType string
	// Timezone is the formula time zone recorded as run.timezone.
	Timezone string
	Source   db.WorkflowRunTrigger
	Depth    int
	// Triggers holds the trigger variables of each run.
	Triggers []map[string]interface{}
	// Flow is the main flow to run when debugging a draft; when empty, it is read from the revision.
	Flow   *AsyncFlow
	UserID int64
}

// enqueue creates a queued run and a job running the main flow for each set of trigger data.
func (e *Engine) enqueue(ctx context.Context, tx *gorm.DB, opts enqueueOptions) ([]*db.WorkflowRun, error) {
	if len(opts.Triggers) == 0 {
		return nil, nil
	}
	source := opts.Source
	if source == "" {
		source = db.WorkflowRunTrigger(opts.TriggerType)
	}
	runs := make([]*db.WorkflowRun, 0, len(opts.Triggers))
	for _, t := range opts.Triggers {
		runs = append(runs, &db.WorkflowRun{
			WorkflowID: opts.Workflow.ID, RevisionID: opts.RevisionID, Version: opts.Version, Trigger: source,
			Status: db.WorkflowRunQueued, UserID: opts.UserID, Event: datatypes.JSON(marshalJSON(t)),
		})
	}
	if err := db.NewWorkflowRunsStore(tx).Create(ctx, runs...); err != nil {
		return nil, errors.Wrap(err, "create runs")
	}

	now := dbutil.Now().UTC().Format(time.RFC3339)
	jobs := make([]*db.WorkflowJob, 0, len(runs))
	for i, run := range runs {
		scope := Scope{
			"trigger": opts.Triggers[i],
			"run":     map[string]interface{}{"id": run.UID, "time": now, "trigger": string(source), "timezone": opts.Timezone},
			"steps":   map[string]interface{}{},
		}
		raw, err := json.Marshal(jobPayload{ProjectID: opts.ProjectID, Flow: opts.Flow, Scope: scope, Depth: opts.Depth})
		if err != nil {
			return nil, errors.Wrap(err, "encode payload")
		}
		jobs = append(jobs, &db.WorkflowJob{
			WorkflowID: opts.Workflow.ID, RunID: run.ID, RevisionID: opts.RevisionID, FlowID: MainFlow, Event: opts.TriggerType, Payload: datatypes.JSON(raw),
		})
	}
	if err := db.NewWorkflowJobsStore(tx).Create(ctx, jobs); err != nil {
		return nil, errors.Wrap(err, "create jobs")
	}
	return runs, nil
}
