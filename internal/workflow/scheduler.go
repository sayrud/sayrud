package workflow

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

const (
	// schedulePollInterval is the interval for checking due schedule and date triggers.
	schedulePollInterval = 15 * time.Second
	// dateScanInterval is the interval at which date triggers scan records.
	dateScanInterval = time.Minute
	// maxSameTimeRecords is the maximum number of records sharing the last time of a truncated scan that are added to the scan.
	maxSameTimeRecords = 5000
	// scheduleBatch is the number of due workflows handled per batch.
	scheduleBatch = 20
	// scheduleRetryDelay is the delay before retrying after a failure.
	scheduleRetryDelay = time.Minute
)

// maxDateRecords is the maximum number of records a date trigger handles per scan; the rest are handled in the next scan.
// It is a variable so that the tests can lower it.
var maxDateRecords = 500

// NotifySchedule makes the scheduler check due workflows immediately, e.g. after publishing or enabling.
func (e *Engine) NotifySchedule() {
	select {
	case e.scheduleWake <- struct{}{}:
	default:
	}
}

func (e *Engine) scheduleLoop(ctx context.Context) {
	ticker := time.NewTicker(schedulePollInterval)
	defer ticker.Stop()
	for {
		for ctx.Err() == nil {
			more, err := e.RunDue(ctx, dbutil.Now())
			if err != nil && ctx.Err() == nil {
				logrus.WithContext(ctx).WithError(err).Error("Failed to run due workflows")
			}
			if !more {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-e.scheduleWake:
		}
	}
}

// RunDue enqueues runs for due schedule and date triggers; more reports whether there may be more due workflows.
func (e *Engine) RunDue(ctx context.Context, now time.Time) (more bool, err error) {
	queued := 0
	err = e.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		list, err := db.NewWorkflowsStore(tx).LockDue(ctx, now, scheduleBatch)
		if err != nil {
			return err
		}
		more = len(list) == scheduleBatch
		for _, wf := range list {
			var n int
			// Each workflow is handled in a savepoint; on failure it is retried later without affecting other workflows.
			if err := tx.Transaction(func(sp *gorm.DB) error {
				n, err = e.fire(ctx, sp, wf, now)
				return err
			}); err != nil {
				logrus.WithContext(ctx).WithError(err).WithField("workflow", wf.UID).Error("Failed to fire workflow")
				retry := now.Add(scheduleRetryDelay)
				if err := db.NewWorkflowsStore(tx).SetSchedule(ctx, wf.ID, &retry, wf.Cursor); err != nil {
					return err
				}
				continue
			}
			queued += n
		}
		return nil
	})
	if queued > 0 {
		e.notifyWorkers()
	}
	return more && err == nil, err
}

// fire handles a due workflow, updates its next run time and returns the number of enqueued runs.
func (e *Engine) fire(ctx context.Context, tx *gorm.DB, wf *db.Workflow, now time.Time) (int, error) {
	workflows := db.NewWorkflowsStore(tx)
	spec, err := e.PublishedSpec(ctx, wf.PublishedRevisionID)
	if err != nil {
		return 0, err
	}
	opts := enqueueOptions{ProjectID: wf.ProjectID, Workflow: wf, RevisionID: wf.PublishedRevisionID, Version: wf.PublishedVersion, TriggerType: spec.Trigger.Type, Timezone: spec.FormulaTimezone(), Depth: 1}

	switch spec.Trigger.Type {
	case TriggerSchedule:
		// Missed runs are caught up only once; the next run is computed from the current time.
		opts.Triggers = []map[string]interface{}{{"time": wf.NextRunAt.UTC().Format(time.RFC3339)}}
		if _, err := e.enqueue(ctx, tx, opts); err != nil {
			return 0, err
		}
		var next *time.Time
		if t, ok := NextFire(spec.Trigger.Schedule, now); ok {
			next = &t
		}
		return 1, workflows.SetSchedule(ctx, wf.ID, next, nil)

	case TriggerDateReached:
		triggers, cursor, truncated, err := dateTriggers(ctx, tx, wf, spec.Trigger.Record, now)
		if err != nil {
			return 0, err
		}
		opts.Triggers = triggers
		if _, err := e.enqueue(ctx, tx, opts); err != nil {
			return 0, err
		}
		next := now.Add(dateScanInterval)
		if truncated {
			next = now
		}
		return len(triggers), workflows.SetSchedule(ctx, wf.ID, &next, &cursor)
	}
	return 0, workflows.SetSchedule(ctx, wf.ID, nil, nil)
}

// dateTriggers returns the records whose trigger time (field time plus offset) falls in (cursor, now], and the new scan position.
func dateTriggers(ctx context.Context, tx *gorm.DB, wf *db.Workflow, r *RecordTrigger, now time.Time) ([]map[string]interface{}, time.Time, bool, error) {
	cursor := now
	if wf.Cursor != nil {
		cursor = *wf.Cursor
	}
	if r == nil || !cursor.Before(now) {
		return nil, now, false, nil
	}
	table, err := db.NewSLTablesStore(tx).GetByID(ctx, wf.TriggerTableID)
	if err != nil {
		return nil, now, false, errors.Wrap(err, "get table")
	}
	fields, err := db.NewSLFieldsStore(tx).ListByTableID(ctx, table.ID)
	if err != nil {
		return nil, now, false, errors.Wrap(err, "list fields")
	}
	info := newTableInfo(table, fields)
	if _, ok := info.byUID[r.DateFieldUID]; !ok {
		return nil, now, false, nil
	}

	offset := time.Duration(r.Offset) * time.Minute
	window := []db.QuerySLRecordsFilter{
		{FieldUID: r.DateFieldUID, Operation: db.FilterOperationGreaterThan, Value: cursor.Add(-offset).UTC().Format(time.RFC3339)},
		{FieldUID: r.DateFieldUID, Operation: db.FilterOperationLessThanOrEqual, Value: now.Add(-offset).UTC().Format(time.RFC3339)},
	}
	records := db.NewSLRecordsStore(tx)
	list, _, err := records.Query(ctx, table.ID, db.QuerySLRecordsOptions{
		Filter: window, Order: []db.QuerySLRecordsSort{{FieldUID: r.DateFieldUID, Order: "asc"}}, Limit: maxDateRecords,
	})
	if err != nil {
		return nil, now, false, errors.Wrap(err, "query records")
	}
	truncated := len(list) == maxDateRecords
	next := now
	if truncated {
		if t, ok := recordTime(list[len(list)-1], r.DateFieldUID); ok {
			next = t.Add(offset)
			// The next scan starts after the last time, so the records sharing it that were cut off by the limit are added now.
			same, _, err := records.Query(ctx, table.ID, db.QuerySLRecordsOptions{
				Filter: []db.QuerySLRecordsFilter{{FieldUID: r.DateFieldUID, Operation: db.FilterOperationEqual, Value: t.UTC().Format(time.RFC3339)}},
				Limit:  maxSameTimeRecords,
			})
			if err != nil {
				return nil, now, false, errors.Wrap(err, "query records at the last time")
			}
			seen := make(map[string]bool, len(list))
			for _, rec := range list {
				seen[rec.UID] = true
			}
			for _, rec := range same {
				if !seen[rec.UID] {
					list = append(list, rec)
				}
			}
		}
	}

	matched := list
	if filters, err := buildFilters(info, Scope{}, r.Filters); err != nil {
		return nil, now, false, err
	} else if len(filters) > 0 {
		data := make(map[string]json.RawMessage, len(list))
		for _, rec := range list {
			data[rec.UID] = json.RawMessage(rec.Data)
		}
		keys, err := records.MatchData(ctx, table.ID, filters, r.Match == "any", data)
		if err != nil {
			return nil, now, false, err
		}
		keep := map[string]bool{}
		for _, k := range keys {
			keep[k] = true
		}
		matched = matched[:0:0]
		for _, rec := range list {
			if keep[rec.UID] {
				matched = append(matched, rec)
			}
		}
	}

	triggers := make([]map[string]interface{}, 0, len(matched))
	for _, rec := range matched {
		data := map[string]interface{}{"record": recordOutput(info, rec), "table": tableData(table)}
		if t, ok := recordTime(rec, r.DateFieldUID); ok {
			data["time"] = t.Add(offset).UTC().Format(time.RFC3339)
		}
		triggers = append(triggers, normalize(data).(map[string]interface{}))
	}
	return triggers, next, truncated, nil
}

// recordTime returns the time of the record's date field.
func recordTime(record *db.SLRecord, fieldUID string) (time.Time, bool) {
	data := map[string]interface{}{}
	_ = json.Unmarshal(record.Data, &data)
	s, _ := data[fieldUID].(string)
	t, err := time.Parse(time.RFC3339, s)
	return t, err == nil
}

// ScheduleState returns the schedule state on publishing or enabling: schedule triggers start from the first fire time after now, and date triggers start scanning from now;
// current is the currently published workflow, whose scan position is kept when the date trigger configuration is unchanged.
func ScheduleState(spec *Spec, current *db.Workflow, now time.Time) db.WorkflowSchedule {
	switch spec.Trigger.Type {
	case TriggerSchedule:
		if t, ok := NextFire(spec.Trigger.Schedule, now); ok {
			return db.WorkflowSchedule{NextRunAt: &t}
		}
	case TriggerDateReached:
		cursor := now
		if current != nil && current.TriggerType == TriggerDateReached && current.Enabled && current.Cursor != nil {
			cursor = *current.Cursor
		}
		return db.WorkflowSchedule{NextRunAt: &now, Cursor: &cursor}
	}
	return db.WorkflowSchedule{}
}
