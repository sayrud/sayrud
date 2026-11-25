package collab

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/importer"
)

// recordChanges returns the changes of the record the newest first.
func recordChanges(t *testing.T, gormDB *gorm.DB, table *db.SLTable, recordUID string) []*db.SLRecordChange {
	t.Helper()
	records, err := db.NewSLRecordsStore(gormDB.Unscoped()).ListByUIDs(context.Background(), table.ID, []string{recordUID})
	require.NoError(t, err)
	require.Len(t, records, 1)
	changes, err := db.NewSLRecordChangesStore(gormDB).ListByRecord(context.Background(), records[0].ID, 0, 100)
	require.NoError(t, err)
	return changes
}

func TestHistoryOfWritePaths(t *testing.T) {
	ctx := context.Background()
	gormDB := clusterTestDB(t)
	hub := NewHub(gormDB)
	project, table, user := testProject(t, gormDB, hub)
	assert.Empty(t, recordChanges(t, gormDB, table, "recAAAAAAAAAAA"), "the writes of the server are not recorded")

	t.Run("websocket", func(t *testing.T) {
		conn := connectCluster(t, hub, project, table, user)
		require.NoError(t, conn.WriteJSON(Message{Type: MessageUserChanges, ReqID: 2, Data: json.RawMessage(`{"tableUID":"` + table.UID + `","signature":"ws-1","operations":[{"command":"SetRecord","actions":[` +
			`{"action":"record.add","recordUID":"recWSWSWSWSWSW","values":{"fldAAAAAAA":"new"}},` +
			`{"action":"record.set","recordUID":"recAAAAAAAAAAA","values":{"fldAAAAAAA":"a","fldBBBBBBB":"b"}}]}]}`)}))
		readClusterMessage(t, conn, MessageAcceptCommit)

		changes := recordChanges(t, gormDB, table, "recAAAAAAAAAAA")
		require.Len(t, changes, 2)
		for _, c := range changes {
			assert.Equal(t, db.SLRecordChangeUpdated, c.Kind)
			assert.Equal(t, db.SLRecordWriter{UserID: user.ID}, c.Writer())
		}
		created := recordChanges(t, gormDB, table, "recWSWSWSWSWSW")
		require.Len(t, created, 1)
		assert.Equal(t, db.SLRecordChangeCreated, created[0].Kind)

		changesets, err := db.NewSLChangesetsStore(gormDB).ListSince(ctx, table.ID, 0, 100)
		require.NoError(t, err)
		assert.Equal(t, db.SLRecordWriter{UserID: user.ID}, changesets[len(changesets)-1].Actor(), "the changeset keeps its writer for the catch-up")
		assert.True(t, changesets[0].Actor().IsZero())

		require.NoError(t, conn.WriteJSON(Message{Type: MessageUserChanges, ReqID: 3, Data: json.RawMessage(`{"tableUID":"` + table.UID + `","signature":"ws-2","operations":[{"command":"DeleteRecords","actions":[` +
			`{"action":"record.delete","recordUIDs":["recWSWSWSWSWSW"]}]}]}`)}))
		readClusterMessage(t, conn, MessageAcceptCommit)
		deleted := recordChanges(t, gormDB, table, "recWSWSWSWSWSW")
		require.Len(t, deleted, 2)
		assert.Equal(t, db.SLRecordChangeDeleted, deleted[0].Kind)
	})

	t.Run("server writes", func(t *testing.T) {
		before := len(recordChanges(t, gormDB, table, "recAAAAAAAAAAA"))
		ok, err := hub.CommitServer(ctx, project, table, []Operation{{Actions: []Action{
			{Action: ActionSetRecord, RecordUID: "recAAAAAAAAAAA", Values: map[string]interface{}{"fldBBBBBBB": "generated"}},
		}}}, nil)
		require.NoError(t, err)
		require.True(t, ok)
		number := "number"
		require.NoError(t, hub.Commit(ctx, project, table, nil, "convert", []Operation{{Actions: []Action{
			{Action: ActionSetFieldType, FieldUID: "fldBBBBBBB", Field: &FieldAttrs{Type: &number}},
		}}}, func(CommitResult) {}))
		assert.Len(t, recordChanges(t, gormDB, table, "recAAAAAAAAAAA"), before, "field shortcuts and type conversions are not recorded")
	})

	t.Run("import", func(t *testing.T) {
		n, err := hub.Import(ctx, project, table, ImportOptions{
			Rows:    [][]*importer.Cell{textCells("x"), textCells("y")},
			Columns: []ImportColumn{{Index: 0, FieldUID: "fldAAAAAAA"}},
			Origin:  Origin{UserID: user.ID},
		})
		require.NoError(t, err)
		require.Equal(t, 2, n)
		var count int64
		require.NoError(t, gormDB.Model(&db.SLRecordChange{}).Where("sl_table_id = ? AND kind = ? AND user_id = ?", table.ID, db.SLRecordChangeCreated, user.ID).Count(&count).Error)
		assert.Equal(t, int64(3), count, "each imported record has a single created change")
	})

	t.Run("batch", func(t *testing.T) {
		origin := Origin{WorkflowID: 42, Depth: 1}
		var created *db.SLRecord
		require.NoError(t, hub.Batch(ctx, project, []*db.SLTable{table}, BatchOptions{Origin: origin}, func(b *Batch) error {
			var err error
			if created, err = b.CreateRecord(table, map[string]interface{}{"fldAAAAAAA": "wf"}); err != nil {
				return err
			}
			_, err = b.UpdateRecord(table, "recAAAAAAAAAAA", func(current map[string]interface{}) map[string]interface{} {
				current["fldAAAAAAA"] = "by workflow"
				return current
			})
			return err
		}))
		changes := recordChanges(t, gormDB, table, "recAAAAAAAAAAA")
		assert.Equal(t, db.SLRecordWriter{WorkflowID: 42}, changes[0].Writer())
		assert.Equal(t, "A", changes[0].FieldLabel)
		created2 := recordChanges(t, gormDB, table, created.UID)
		require.Len(t, created2, 1)
		assert.Equal(t, db.SLRecordWriter{WorkflowID: 42}, created2[0].Writer())

		// A dry run rolls back the history with the records.
		total := func() int64 {
			var n int64
			require.NoError(t, gormDB.Model(&db.SLRecordChange{}).Count(&n).Error)
			return n
		}
		before := total()
		require.NoError(t, hub.Batch(ctx, project, []*db.SLTable{table}, BatchOptions{Origin: origin, DryRun: true}, func(b *Batch) error {
			_, err := b.CreateRecord(table, map[string]interface{}{"fldAAAAAAA": "dry"})
			return err
		}))
		assert.Equal(t, before, total())
	})
}
