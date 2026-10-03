package shortcut

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thanhpk/randstr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
)

// newTestDB connects to the PostgreSQL of the PG* environment variables in a temporary schema, and skips the test if not configured.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("PGHOST") == "" {
		t.Skip("PGHOST is not set")
	}
	admin, err := gorm.Open(postgres.New(postgres.Config{DSN: "", PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	schema := "test_" + strings.ToLower(randstr.String(10))
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	gormDB, err := gorm.Open(postgres.New(postgres.Config{DSN: "search_path=" + schema, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := gormDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, gormDB.AutoMigrate(&db.Project{}, &db.SLTable{}, &db.SLField{}, &db.SLRecord{}, &db.SLView{}, &db.SLChangeset{},
		&db.CustomFieldShortcut{}, &db.SLShortcutJob{}))
	db.SetDatabaseStore(gormDB)
	return gormDB
}

func strPtr(s string) *string { return &s }

func TestEngine(t *testing.T) {
	gormDB := newTestDB(t)
	ctx := context.Background()

	project := &db.Project{Name: "Test"}
	require.NoError(t, gormDB.Create(project).Error)
	table, err := db.NewSLTablesStore(gormDB).Create(ctx, project.ID, db.CreateSLTableOptions{Name: "Feedback"})
	require.NoError(t, err)

	hub := collab.NewHub(gormDB)
	ai := &fakeAI{reply: "Product"}
	engine := NewEngine(gormDB, hub, &Executor{AI: func(context.Context) (AIClient, error) { return ai, nil }}, 1)
	jobs := db.NewSLShortcutJobsStore(gormDB)

	const source, category = "fldSourceX", "fldCategry"
	commit := func(signature string, actions ...collab.Action) {
		t.Helper()
		require.NoError(t, hub.Commit(ctx, project, table, nil, signature, []collab.Operation{{Command: "Test", Actions: actions}}, func(collab.CommitResult) {}))
	}
	waitJobs := func(check func([]*db.SLShortcutJob) bool) []*db.SLShortcutJob {
		t.Helper()
		var list []*db.SLShortcutJob
		require.Eventually(t, func() bool {
			list, err = jobs.ListByTableID(ctx, table.ID)
			require.NoError(t, err)
			return check(list)
		}, 5*time.Second, 20*time.Millisecond)
		return list
	}
	cell := func(recordUID string) interface{} {
		t.Helper()
		record, err := db.NewSLRecordsStore(gormDB).GetByUID(ctx, recordUID)
		require.NoError(t, err)
		data := map[string]interface{}{}
		require.NoError(t, json.Unmarshal(record.Data, &data))
		return data[category]
	}
	processNext := func() *db.SLShortcutJob {
		t.Helper()
		job, err := jobs.Claim(ctx)
		require.NoError(t, err)
		require.NotNil(t, job)
		engine.process(ctx, job)
		return job
	}

	// An invalid shortcut rejects the whole changeset.
	err = hub.Commit(ctx, project, table, nil, "invalid", []collab.Operation{{Command: "AddField", Actions: []collab.Action{{
		Action: collab.ActionAddField, FieldUID: category,
		Field: &collab.FieldAttrs{Label: strPtr("Category"), Type: strPtr("single_select"), Shortcut: &db.FieldShortcut{ID: "ai_classify", Inputs: map[string]interface{}{}}},
	}}}}, func(collab.CommitResult) {})
	var operationErr *collab.OperationError
	require.ErrorAs(t, err, &operationErr)
	require.Equal(t, "shortcut::input_required", operationErr.Key)

	commit("setup",
		collab.Action{Action: collab.ActionAddField, FieldUID: source, Field: &collab.FieldAttrs{Label: strPtr("Feedback"), Type: strPtr("text")}},
		collab.Action{Action: collab.ActionAddField, FieldUID: category, Field: &collab.FieldAttrs{
			Label: strPtr("Category"), Type: strPtr("single_select"),
			Metadata: map[string]interface{}{"options": []interface{}{
				map[string]interface{}{"uid": "optProduct", "name": "Product", "color": 0},
				map[string]interface{}{"uid": "optService", "name": "Service", "color": 1},
			}},
			Shortcut: &db.FieldShortcut{ID: "ai_classify", Inputs: map[string]interface{}{"source": source}, AutoUpdate: true},
		}},
		collab.Action{Action: collab.ActionAddRecord, RecordUID: "recAAAAAAAAAAA", Values: map[string]interface{}{source: "The app crashes"}},
		collab.Action{Action: collab.ActionAddRecord, RecordUID: "recBBBBBBBBBBB", Values: map[string]interface{}{}},
	)

	// Adding the shortcut queues the whole column.
	waitJobs(func(list []*db.SLShortcutJob) bool { return len(list) == 2 })
	processNext()
	processNext()
	require.Equal(t, "optProduct", cell("recAAAAAAAAAAA"))
	require.Nil(t, cell("recBBBBBBBBBBB"))
	require.Equal(t, 1, ai.calls, "the empty record does not call the model")
	waitJobs(func(list []*db.SLShortcutJob) bool { return len(list) == 0 })

	// Changing the input regenerates the cell.
	ai.reply = "Service"
	commit("edit1", collab.Action{Action: collab.ActionSetRecord, RecordUID: "recBBBBBBBBBBB", Values: map[string]interface{}{source: "Slow support"}})
	waitJobs(func(list []*db.SLShortcutJob) bool { return len(list) == 1 })
	claimed, err := jobs.Claim(ctx)
	require.NoError(t, err)

	// The input changes again while running, so the stale result is discarded.
	commit("edit2", collab.Action{Action: collab.ActionSetRecord, RecordUID: "recBBBBBBBBBBB", Values: map[string]interface{}{source: "Rude support"}})
	waitJobs(func(list []*db.SLShortcutJob) bool { return len(list) == 1 && list[0].Token == 2 })
	engine.process(ctx, claimed)
	require.Nil(t, cell("recBBBBBBBBBBB"))

	processNext()
	require.Equal(t, "optService", cell("recBBBBBBBBBBB"))

	// Editing other fields does not regenerate.
	commit("edit3", collab.Action{Action: collab.ActionSetRecord, RecordUID: "recBBBBBBBBBBB", Values: map[string]interface{}{category: "optProduct"}})
	time.Sleep(100 * time.Millisecond)
	list, err := jobs.ListByTableID(ctx, table.ID)
	require.NoError(t, err)
	require.Empty(t, list)

	// Only toggling the auto update keeps the values.
	commit("toggle", collab.Action{Action: collab.ActionSetFieldShortcut, FieldUID: category, Shortcut: &db.FieldShortcut{ID: "ai_classify", Inputs: map[string]interface{}{"source": source}}})
	time.Sleep(100 * time.Millisecond)
	list, err = jobs.ListByTableID(ctx, table.ID)
	require.NoError(t, err)
	require.Empty(t, list)
	field, err := db.NewSLFieldsStore(gormDB).GetByUID(ctx, category)
	require.NoError(t, err)
	require.False(t, field.Shortcut.AutoUpdate)

	// Invalid outputs fail without retrying.
	ai.reply = "Unknown"
	_, err = engine.Run(ctx, project, table, &db.SLField{UID: category}, RunScopeRecords, []string{"recAAAAAAAAAAA"})
	require.NoError(t, err)
	processNext()
	list = waitJobs(func(list []*db.SLShortcutJob) bool { return len(list) == 1 })
	require.Equal(t, db.ShortcutJobFailed, list[0].Status)
	require.Equal(t, "shortcut::invalid_output", list[0].Error.Data().Key)
	require.Equal(t, "optProduct", cell("recAAAAAAAAAAA"))

	// Removing the shortcut drops its jobs.
	commit("remove", collab.Action{Action: collab.ActionSetFieldShortcut, FieldUID: category})
	list, err = jobs.ListByTableID(ctx, table.ID)
	require.NoError(t, err)
	require.Empty(t, list)
	field, err = db.NewSLFieldsStore(gormDB).GetByUID(ctx, category)
	require.NoError(t, err)
	require.Nil(t, field.Shortcut)

	// Changing the type of a shortcut field regenerates the converted values if the shortcut supports the new type.
	const amount = "fldAmountX"
	commit("extract", collab.Action{Action: collab.ActionAddField, FieldUID: amount, Field: &collab.FieldAttrs{
		Label: strPtr("Amount"), Type: strPtr("text"),
		Shortcut: &db.FieldShortcut{ID: "ai_extract", Inputs: map[string]interface{}{"source": source, "target": "the amount"}},
	}})
	waitJobs(func(list []*db.SLShortcutJob) bool { return len(list) == 2 })
	processNext()
	processNext()
	commit("toNumber", collab.Action{Action: collab.ActionSetFieldType, FieldUID: amount, Field: &collab.FieldAttrs{Type: strPtr("number"), Metadata: map[string]interface{}{"format": "0"}}})
	waitJobs(func(list []*db.SLShortcutJob) bool { return len(list) == 2 })
	field, err = db.NewSLFieldsStore(gormDB).GetByUID(ctx, amount)
	require.NoError(t, err)
	require.NotNil(t, field.Shortcut)

	// A type the shortcut does not support removes the shortcut along with its jobs.
	commit("toCheckbox", collab.Action{Action: collab.ActionSetFieldType, FieldUID: amount, Field: &collab.FieldAttrs{Type: strPtr("checkbox")}})
	field, err = db.NewSLFieldsStore(gormDB).GetByUID(ctx, amount)
	require.NoError(t, err)
	require.Nil(t, field.Shortcut)
	list, err = jobs.ListByTableID(ctx, table.ID)
	require.NoError(t, err)
	require.Empty(t, list)
}
