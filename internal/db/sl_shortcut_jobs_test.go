package db

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/thanhpk/randstr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestDB connects to the PostgreSQL of the PG* environment variables in a temporary schema, and skips the test if not configured.
func newTestDB(t *testing.T, models ...interface{}) *gorm.DB {
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
	require.NoError(t, gormDB.AutoMigrate(models...))
	return gormDB
}

func TestSLShortcutJobs(t *testing.T) {
	ctx := context.Background()
	store := NewSLShortcutJobsStore(newTestDB(t, &SLShortcutJob{}))

	jobs, err := store.Enqueue(ctx, 1, "fldAAAAAAA", []string{"rec1", "rec2", "rec1"})
	require.NoError(t, err)
	require.Len(t, jobs, 2)

	claimed, err := store.Claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	require.Equal(t, ShortcutJobRunning, claimed.Status)
	require.Equal(t, 1, claimed.Attempts)
	require.Equal(t, int64(1), claimed.Token)

	// Re-enqueueing the running cell invalidates the running execution.
	_, err = store.Enqueue(ctx, 1, "fldAAAAAAA", []string{claimed.RecordUID})
	require.NoError(t, err)
	finished, err := store.Finish(ctx, claimed)
	require.NoError(t, err)
	require.False(t, finished)
	failed, err := store.Fail(ctx, claimed, ShortcutJobError{Key: "shortcut::invalid_output"})
	require.NoError(t, err)
	require.False(t, failed)

	// Both cells are pending, the re-enqueued one has a new token.
	second, err := store.Claim(ctx)
	require.NoError(t, err)
	third, err := store.Claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.NotNil(t, third)
	none, err := store.Claim(ctx)
	require.NoError(t, err)
	require.Nil(t, none)

	reclaimed := second
	if reclaimed.RecordUID != claimed.RecordUID {
		reclaimed = third
	}
	require.Equal(t, int64(2), reclaimed.Token)
	finished, err = store.Finish(ctx, reclaimed)
	require.NoError(t, err)
	require.True(t, finished)

	other := otherJob(second, third, reclaimed)
	retried, err := store.Retry(ctx, other, time.Now().Add(time.Hour), ShortcutJobError{Key: "shortcut::request_failed"})
	require.NoError(t, err)
	require.True(t, retried)
	none, err = store.Claim(ctx)
	require.NoError(t, err)
	require.Nil(t, none, "the retried job is not due yet")

	list, err := store.ListByTableID(ctx, 1)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, ShortcutJobPending, list[0].Status)
	require.Equal(t, "shortcut::request_failed", list[0].Error.Data().Key)

	require.NoError(t, store.DeleteByRecords(ctx, 1, []string{other.RecordUID}))
	list, err = store.ListByTableID(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, list)
}

func TestSLShortcutJobsResetStale(t *testing.T) {
	ctx := context.Background()
	store := NewSLShortcutJobsStore(newTestDB(t, &SLShortcutJob{}))

	_, err := store.Enqueue(ctx, 1, "fldAAAAAAA", []string{"rec1"})
	require.NoError(t, err)
	claimed, err := store.Claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, claimed)

	n, err := store.ResetStale(ctx, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	require.Zero(t, n)
	n, err = store.ResetStale(ctx, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	again, err := store.Claim(ctx)
	require.NoError(t, err)
	require.NotNil(t, again)
	require.Equal(t, 2, again.Attempts)
}

func TestFieldShortcutColumn(t *testing.T) {
	ctx := context.Background()
	store := NewSLFieldsStore(newTestDB(t, &SLField{}))

	field, err := store.Create(ctx, CreateSLFieldOptions{SLTableID: 1, Label: "Category", Type: SingleSelectFieldType, Metadata: map[string]interface{}{}})
	require.NoError(t, err)
	got, err := store.GetByID(ctx, field.ID)
	require.NoError(t, err)
	require.Nil(t, got.Shortcut)

	shortcut := &FieldShortcut{ID: "ai_classify", Inputs: map[string]interface{}{"source": "fldBBBBBBB"}, AutoUpdate: true}
	require.NoError(t, store.SetShortcut(ctx, field.ID, shortcut))
	got, err = store.GetByID(ctx, field.ID)
	require.NoError(t, err)
	require.Equal(t, shortcut, got.Shortcut)

	require.NoError(t, store.SetShortcut(ctx, field.ID, nil))
	got, err = store.GetByID(ctx, field.ID)
	require.NoError(t, err)
	require.Nil(t, got.Shortcut)
}

// otherJob returns the job of a and b which is not c.
func otherJob(a, b, c *SLShortcutJob) *SLShortcutJob {
	if a.ID == c.ID {
		return b
	}
	return a
}
