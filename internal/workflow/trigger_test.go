package workflow

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
)

func constOf(v interface{}) *Value {
	raw, _ := json.Marshal(v)
	return &Value{Kind: ValueConst, Const: raw}
}

// write writes to the leads table as a user in a single transaction.
func (f *leadFixture) write(t *testing.T, fn func(b *collab.Batch)) {
	t.Helper()
	require.NoError(t, f.hub.Batch(context.Background(), f.project, []*db.SLTable{f.table}, collab.BatchOptions{}, func(b *collab.Batch) error {
		fn(b)
		return nil
	}))
}

func (f *leadFixture) create(t *testing.T, values map[string]interface{}) string {
	t.Helper()
	var uid string
	f.write(t, func(b *collab.Batch) {
		record, err := b.CreateRecord(f.table, values)
		require.NoError(t, err)
		uid = record.UID
	})
	return uid
}

func (f *leadFixture) update(t *testing.T, uid string, values map[string]interface{}) {
	t.Helper()
	f.write(t, func(b *collab.Batch) {
		_, err := b.UpdateRecord(f.table, uid, func(current map[string]interface{}) map[string]interface{} {
			for k, v := range values {
				current[k] = v
			}
			return current
		})
		require.NoError(t, err)
	})
}

func (f *leadFixture) runs(t *testing.T, wf *db.Workflow) []*db.WorkflowRun {
	t.Helper()
	var runs []*db.WorkflowRun
	require.NoError(t, f.db.Where("workflow_id = ?", wf.ID).Order("id ASC").Find(&runs).Error)
	return runs
}

func TestRecordTriggers(t *testing.T) {
	f := newLeadFixture(t)
	created := f.publish(t, "On web lead", &Spec{
		Trigger: Trigger{Type: TriggerRecordCreated, Record: &RecordTrigger{TableUID: f.table.UID, Filters: []Filter{{FieldUID: f.source.UID, Op: "eq", Value: constOf("Web")}}}},
		Steps: []Step{{ID: "mark", Type: StepUpdate, TableUID: f.table.UID, RecordUID: vp("trigger.record.uid"), Fields: []FieldValue{
			{FieldUID: f.name.UID, Value: Value{Kind: ValueTemplate, Template: "{{trigger.record.fields." + f.name.UID + "}} (web)"}},
		}}},
	})
	updated := f.publish(t, "On email change", &Spec{
		Trigger: Trigger{Type: TriggerRecordUpdated, Record: &RecordTrigger{TableUID: f.table.UID, Fields: []string{f.email.UID}}},
		Steps:   []Step{{ID: "find", Type: StepQuery, TableUID: f.table.UID, Limit: 1}},
	})
	matched := f.publish(t, "Became ads", &Spec{
		Trigger: Trigger{Type: TriggerRecordMatched, Record: &RecordTrigger{TableUID: f.table.UID, Filters: []Filter{{FieldUID: f.source.UID, Op: "eq", Value: constOf("Ads")}}}},
		Steps:   []Step{{ID: "find", Type: StepQuery, TableUID: f.table.UID, Limit: 1}},
	})
	renamed := f.publish(t, "On rename", &Spec{
		Trigger: Trigger{Type: TriggerRecordUpdated, Record: &RecordTrigger{TableUID: f.table.UID, Fields: []string{f.name.UID}}},
		Steps:   []Step{{ID: "find", Type: StepQuery, TableUID: f.table.UID, Limit: 1}},
	})

	web := f.create(t, map[string]interface{}{f.name.UID: "Ann", f.email.UID: "ann@example.com", f.source.UID: "optWeb"})
	f.create(t, map[string]interface{}{f.name.UID: "Bob", f.source.UID: "optAds"})
	require.Len(t, f.runs(t, created), 1, "only the record matching the conditions triggers")
	assert.Equal(t, db.WorkflowRunQueued, f.runs(t, created)[0].Status)
	require.Len(t, f.runs(t, matched), 1, "a new record matching the conditions triggers")

	f.update(t, web, map[string]interface{}{f.name.UID: "Ann Lee"})
	assert.Empty(t, f.runs(t, updated), "changes of the fields not watched are ignored")
	f.update(t, web, map[string]interface{}{f.email.UID: "ann@example.org"})
	require.Len(t, f.runs(t, updated), 1)
	event := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(f.runs(t, updated)[0].Event, &event))
	assert.Equal(t, []interface{}{"Email"}, event["changedFields"])
	assert.Equal(t, "ann@example.com", event["previous"].(map[string]interface{})["fields"].(map[string]interface{})[f.email.UID])

	f.update(t, web, map[string]interface{}{f.source.UID: "optAds"})
	f.update(t, web, map[string]interface{}{f.email.UID: "ann@example.net"})
	assert.Len(t, f.runs(t, matched), 2, "only the change into the conditions triggers, not the later edits")

	ctx, cancel := context.WithCancel(context.Background())
	f.engine.Start(ctx)
	defer func() {
		cancel()
		_ = f.engine.Shutdown(context.Background(), context.Background())
	}()
	require.Eventually(t, func() bool {
		runs := f.runs(t, created)
		return runs[0].Status == db.WorkflowRunSucceeded
	}, 15*time.Second, 100*time.Millisecond)
	var record db.SLRecord
	require.NoError(t, f.db.Where("uid = ?", web).First(&record).Error)
	data := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(record.Data, &data))
	assert.Equal(t, "Ann (web)", data[f.name.UID])
	assert.Len(t, f.runs(t, created), 1, "the workflow is not triggered by its own writes")
	require.Eventually(t, func() bool { return len(f.runs(t, renamed)) == 2 }, 5*time.Second, 50*time.Millisecond, "the writes of a workflow trigger the others")
}

func TestRecordDeletedTrigger(t *testing.T) {
	f := newLeadFixture(t)
	deleted := f.publish(t, "On delete", &Spec{
		Trigger: Trigger{Type: TriggerRecordDeleted, Record: &RecordTrigger{TableUID: f.table.UID}},
		Steps:   []Step{{ID: "find", Type: StepQuery, TableUID: f.table.UID, Limit: 1}},
	})
	uid := f.create(t, map[string]interface{}{f.name.UID: "Ann"})
	require.NoError(t, f.hub.MutateRecords(context.Background(), f.project, f.table, func(tx *gorm.DB) ([]collab.RecordEvent, error) {
		record, err := db.NewSLRecordsStore(tx).GetByUID(context.Background(), uid)
		if err != nil {
			return nil, err
		}
		return []collab.RecordEvent{{Kind: collab.RecordDeleted, Record: record}}, db.NewSLRecordsStore(tx).DeleteByID(context.Background(), record.ID)
	}, func() collab.DirtyScope { return collab.DirtyScope{Records: []string{uid}} }))
	runs := f.runs(t, deleted)
	require.Len(t, runs, 1)
	event := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(runs[0].Event, &event))
	assert.Equal(t, "Ann", event["record"].(map[string]interface{})["fields"].(map[string]interface{})[f.name.UID])
}

func TestCheckTriggerScopes(t *testing.T) {
	f := newLeadFixture(t)
	spec := &Spec{
		Trigger: Trigger{Type: TriggerRecordCreated, Record: &RecordTrigger{TableUID: f.table.UID}},
		Steps: []Step{
			{ID: "find", Type: StepQuery, TableUID: f.table.UID, Filters: []Filter{{FieldUID: f.email.UID, Op: "eq", Value: vp("input.email")}}},
			{ID: "mail", Type: StepEmail, To: vp("trigger.record.fields." + f.email.UID), Subject: &Value{Kind: ValueTemplate, Template: "{{trigger.previous.uid}}"}},
			{ID: "respond", Type: StepResponse, Status: 200},
		},
	}
	issues, err := Check(context.Background(), f.db, f.project, spec)
	require.NoError(t, err)
	got := map[string]bool{}
	for _, i := range issues {
		got[i.Code] = true
	}
	assert.True(t, got["value.unknown_var"], "input and trigger.previous are not available")
	assert.True(t, got["step.response_api_only"])
	assert.False(t, got["step.async_only"], "background flows can send emails")
	assert.False(t, got["flow.missing_response"])

	blank, err := ParseSpec(nil)
	require.NoError(t, err)
	assert.Empty(t, blank.Trigger.Type, "a new workflow chooses its trigger later")
	issues, err = Check(context.Background(), f.db, f.project, blank)
	require.NoError(t, err)
	assert.Contains(t, issues, Issue{Code: "trigger.unset", Target: "trigger"})

	spec.Trigger = Trigger{Type: TriggerRecordMatched, Record: &RecordTrigger{TableUID: f.table.UID}}
	issues, err = Check(context.Background(), f.db, f.project, spec)
	require.NoError(t, err)
	assert.Contains(t, issues, Issue{Code: "trigger.no_conditions", Target: "trigger"})
}

func TestNextFire(t *testing.T) {
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err)
		return v
	}
	cases := []struct {
		name  string
		s     ScheduleTrigger
		after string
		want  string
	}{
		{"once in the future", ScheduleTrigger{Start: "2026-10-12T09:00", Timezone: "Asia/Shanghai", Repeat: RepeatNone}, "2026-10-11T00:00:00Z", "2026-10-12T01:00:00Z"},
		{"daily after today", ScheduleTrigger{Start: "2026-10-01T09:00", Timezone: "Asia/Shanghai", Repeat: RepeatDaily}, "2026-10-11T02:00:00Z", "2026-10-12T01:00:00Z"},
		{"every 15 minutes", ScheduleTrigger{Start: "2026-10-01T09:00", Timezone: "UTC", Repeat: RepeatMinutes, Interval: 15}, "2026-10-11T10:07:00Z", "2026-10-11T10:15:00Z"},
		{"workdays skip weekend", ScheduleTrigger{Start: "2026-10-01T09:00", Timezone: "UTC", Repeat: RepeatWorkdays}, "2026-10-09T10:00:00Z", "2026-10-12T09:00:00Z"},
		{"weekly on wednesday", ScheduleTrigger{Start: "2026-10-01T18:30", Timezone: "UTC", Repeat: RepeatWeekly, Weekdays: []int{3}}, "2026-10-11T00:00:00Z", "2026-10-14T18:30:00Z"},
		{"monthly clamps to the month end", ScheduleTrigger{Start: "2026-01-31T08:00", Timezone: "UTC", Repeat: RepeatMonthly}, "2026-02-01T00:00:00Z", "2026-02-28T08:00:00Z"},
	}
	for _, c := range cases {
		got, ok := NextFire(&c.s, at(c.after))
		require.True(t, ok, c.name)
		assert.Equal(t, at(c.want), got.UTC(), c.name)
	}
	_, ok := NextFire(&ScheduleTrigger{Start: "2026-10-01T09:00", Timezone: "UTC", Repeat: RepeatNone}, at("2026-10-02T00:00:00Z"))
	assert.False(t, ok, "a passed one-time schedule does not run")
	assert.Equal(t, "trigger.invalid_interval", checkSchedule(&ScheduleTrigger{Start: "2026-10-01T09:00", Timezone: "UTC", Repeat: RepeatMinutes, Interval: 1}))
}

func TestScheduleAndDateTriggers(t *testing.T) {
	f := newLeadFixture(t)
	ctx := context.Background()
	fields := db.NewSLFieldsStore(f.db)
	due, err := fields.Create(ctx, db.CreateSLFieldOptions{SLTableID: f.table.ID, Label: "Due", Type: db.DateTimeFieldType, Metadata: map[string]interface{}{}})
	require.NoError(t, err)

	now := time.Now().UTC().Truncate(time.Second)
	scheduled := f.publish(t, "Daily report", &Spec{
		Trigger: Trigger{Type: TriggerSchedule, Schedule: &ScheduleTrigger{Start: now.Add(-time.Hour).Format(ScheduleLayout), Timezone: "UTC", Repeat: RepeatHours, Interval: 1}},
		Steps:   []Step{{ID: "find", Type: StepQuery, TableUID: f.table.UID, Limit: 1}},
	})
	require.NotNil(t, scheduled.NextRunAt)
	assert.True(t, scheduled.NextRunAt.After(now), "the schedule starts after publishing")

	reminder := f.publish(t, "Remind before due", &Spec{
		Trigger: Trigger{Type: TriggerDateReached, Record: &RecordTrigger{TableUID: f.table.UID, DateFieldUID: due.UID, Offset: -60}},
		Steps:   []Step{{ID: "find", Type: StepQuery, TableUID: f.table.UID, Limit: 1}},
	})
	soon := f.create(t, map[string]interface{}{f.name.UID: "Soon", due.UID: now.Add(90 * time.Minute).Format(time.RFC3339)})
	f.create(t, map[string]interface{}{f.name.UID: "Later", due.UID: now.Add(5 * time.Hour).Format(time.RFC3339)})

	later := now.Add(2 * time.Hour)
	for more := true; more; {
		more, err = f.engine.RunDue(ctx, later)
		require.NoError(t, err)
	}
	runs := f.runs(t, scheduled)
	require.Len(t, runs, 1)
	assert.Equal(t, db.WorkflowRunTrigger(TriggerSchedule), runs[0].Trigger)
	wf, err := db.NewWorkflowsStore(f.db).GetByID(ctx, scheduled.ID)
	require.NoError(t, err)
	assert.True(t, wf.NextRunAt.After(later), "the missed times are skipped")

	runs = f.runs(t, reminder)
	require.Len(t, runs, 1, "only the record whose time minus the offset has passed triggers")
	event := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(runs[0].Event, &event))
	assert.Equal(t, soon, event["record"].(map[string]interface{})["uid"])
	wf, err = db.NewWorkflowsStore(f.db).GetByID(ctx, reminder.ID)
	require.NoError(t, err)
	assert.Equal(t, later.Unix(), wf.Cursor.Unix())
}

func TestDateTriggerKeepsRecordsAtTruncatedTime(t *testing.T) {
	f := newLeadFixture(t)
	ctx := context.Background()
	due, err := db.NewSLFieldsStore(f.db).Create(ctx, db.CreateSLFieldOptions{SLTableID: f.table.ID, Label: "Due", Type: db.DateTimeFieldType, Metadata: map[string]interface{}{}})
	require.NoError(t, err)
	defer func(n int) { maxDateRecords = n }(maxDateRecords)
	maxDateRecords = 2

	now := time.Now().UTC().Truncate(time.Second)
	reminder := f.publish(t, "Remind", &Spec{
		Trigger: Trigger{Type: TriggerDateReached, Record: &RecordTrigger{TableUID: f.table.UID, DateFieldUID: due.UID}},
		Steps:   []Step{{ID: "find", Type: StepQuery, TableUID: f.table.UID, Limit: 1}},
	})
	at := now.Add(30 * time.Minute).Format(time.RFC3339)
	for _, name := range []string{"A", "B", "C"} {
		f.create(t, map[string]interface{}{f.name.UID: name, due.UID: at})
	}
	f.create(t, map[string]interface{}{f.name.UID: "D", due.UID: now.Add(40 * time.Minute).Format(time.RFC3339)})

	// A truncated scan is due again right away, the polls are simulated by calling RunDue repeatedly.
	later := now.Add(time.Hour)
	for range 3 {
		_, err = f.engine.RunDue(ctx, later)
		require.NoError(t, err)
	}
	assert.Len(t, f.runs(t, reminder), 4, "the records sharing the time at the scan limit all trigger")
}
