package shortcut

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
)

func TestEngineShutdownDrainsOrReleasesOwnedJobs(t *testing.T) {
	for _, mode := range []string{"finish", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			gormDB := newTestDB(t)
			ctx := context.Background()
			project := &db.Project{Name: "Test"}
			require.NoError(t, gormDB.Create(project).Error)
			table, err := db.NewSLTablesStore(gormDB).Create(ctx, project.ID, db.CreateSLTableOptions{Name: "Test"})
			require.NoError(t, err)
			code := "function execute() { return 'done'; }"
			if mode == "cancel" {
				code = "function execute() { while (true) {} }"
			}
			custom := &db.CustomFieldShortcut{UID: "fscAAAAAAA", Name: "Test", ResultType: db.TextFieldType, Code: code, Enabled: true, TimeoutSeconds: 10,
				FormItems: datatypes.NewJSONType([]db.ShortcutFormItem{}), Domains: datatypes.NewJSONType([]string{}), Credentials: datatypes.NewJSONType([]db.ShortcutCredential{})}
			require.NoError(t, gormDB.Create(custom).Error)
			field, err := db.NewSLFieldsStore(gormDB).Create(ctx, db.CreateSLFieldOptions{SLTableID: table.ID, Label: "Result", Type: db.TextFieldType, Metadata: map[string]interface{}{}})
			require.NoError(t, err)
			require.NoError(t, db.NewSLFieldsStore(gormDB).SetShortcut(ctx, field.ID, &db.FieldShortcut{ID: custom.UID, Inputs: map[string]interface{}{}}))
			record, err := db.NewSLRecordsStore(gormDB).Create(ctx, table.ID, []byte("{}"))
			require.NoError(t, err)
			other, err := db.NewSLRecordsStore(gormDB).Create(ctx, table.ID, []byte("{}"))
			require.NoError(t, err)
			store := db.NewSLShortcutJobsStore(gormDB)
			_, err = store.Enqueue(ctx, table.ID, field.UID, []string{record.UID, other.UID})
			require.NoError(t, err)
			hub := collab.NewHub(gormDB)
			engine := NewEngine(gormDB, hub, 1)
			started, proceed := make(chan struct{}), make(chan struct{})
			resume := sync.OnceFunc(func() { close(proceed) })
			defer resume()
			var once sync.Once
			// A running row can precede the post-claim cancellation check; pause inside process instead.
			require.NoError(t, gormDB.Callback().Query().Before("gorm:query").Register("test:job_started", func(tx *gorm.DB) {
				if tx.Statement.Table == "sl_tables" {
					once.Do(func() {
						close(started)
						<-proceed
					})
				}
			}))
			engine.Start(ctx)
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("worker did not start processing a job")
			}
			engine.Stop()
			resume()
			budget := 2 * time.Second
			if mode == "cancel" {
				budget = 20 * time.Millisecond
			}
			shutdown, cancel := context.WithTimeout(ctx, budget)
			cleanup, cancelCleanup := context.WithTimeout(ctx, 2*time.Second)
			err = engine.Shutdown(shutdown, cleanup)
			cancelCleanup()
			cancel()
			require.NoError(t, err)
			jobs, err := store.ListByTableID(ctx, table.ID)
			require.NoError(t, err)
			count := 1
			if mode == "cancel" {
				count = 2
			}
			require.Len(t, jobs, count)
			for _, job := range jobs {
				require.Equal(t, db.ShortcutJobPending, job.Status)
				require.Zero(t, job.Attempts)
				require.Nil(t, job.LockedAt)
			}
			require.NoError(t, hub.WaitBackground(ctx))
			hub.CancelBackground()
		})
	}
}
