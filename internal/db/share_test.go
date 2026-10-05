package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestSharePersistenceAndAttachmentScope(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t, &SLTable{}, &SLField{}, &SLRecord{})
	tables, records := NewSLTablesStore(database), NewSLRecordsStore(database)
	table, err := tables.Create(ctx, 1, CreateSLTableOptions{Name: "Shared"})
	require.NoError(t, err)

	token, hash, sealed, enabled, children := "random-token", "hashed-password", "encrypted-password", true, true
	require.NoError(t, tables.Update(ctx, table.ID, UpdateSLTableOptions{
		ShareToken: &token, SharePasswordHash: &hash, SharePasswordSealed: &sealed, ShareEnabled: &enabled, ShareIncludeChildren: &children,
	}))
	got, err := tables.GetByShareToken(ctx, token)
	require.NoError(t, err)
	require.Equal(t, hash, got.SharePasswordHash)
	require.Equal(t, sealed, got.SharePasswordSealed)
	require.True(t, got.ShareIncludeChildren)

	field := &SLField{UID: "fldAttachment", SLTableID: table.ID, Label: "File", Type: AttachmentFieldType,
		Metadata: datatypes.NewJSONType[SLFieldMetadata](map[string]interface{}{})}
	require.NoError(t, database.Create(field).Error)
	record := &SLRecord{SLTableID: table.ID, Data: datatypes.JSON(`{"fldAttachment":[{"uid":"filOne"}],"fldText":[{"uid":"filPrivate"}]}`)}
	require.NoError(t, database.Create(record).Error)
	found, err := records.HasAttachment(ctx, table.ID, "filOne")
	require.NoError(t, err)
	require.True(t, found)
	for _, uid := range []string{"filPrivate", "missing", "' OR true --"} {
		found, err = records.HasAttachment(ctx, table.ID, uid)
		require.NoError(t, err)
		require.False(t, found)
	}
	found, err = records.HasAttachment(ctx, table.ID+1, "filOne")
	require.NoError(t, err)
	require.False(t, found)
	require.NoError(t, database.Delete(field).Error)
	found, err = records.HasAttachment(ctx, table.ID, "filOne")
	require.NoError(t, err)
	require.False(t, found)

	enabled = false
	require.NoError(t, tables.Update(ctx, table.ID, UpdateSLTableOptions{ShareEnabled: &enabled}))
	_, err = tables.GetByShareToken(ctx, token)
	require.ErrorIs(t, err, ErrSLTableNotFound)
	enabled = true
	require.NoError(t, tables.Update(ctx, table.ID, UpdateSLTableOptions{ShareEnabled: &enabled}))
	require.NoError(t, tables.DeleteByID(ctx, table.ID))
	_, err = tables.GetByShareToken(ctx, token)
	require.ErrorIs(t, err, ErrSLTableNotFound)
}
