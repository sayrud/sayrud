package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestSLRecordsCountByTableIDs(t *testing.T) {
	ctx := context.Background()
	database := newTestDB(t, &SLRecord{})
	store := NewSLRecordsStore(database)
	records := []*SLRecord{
		{SLTableID: 1, Data: datatypes.JSON(`{}`)},
		{SLTableID: 1, Data: datatypes.JSON(`{}`)},
		{SLTableID: 1, Data: datatypes.JSON(`{}`)},
		{SLTableID: 2, Data: datatypes.JSON(`{}`)},
		{SLTableID: 3, Data: datatypes.JSON(`{}`)},
	}
	require.NoError(t, database.Create(&records).Error)
	require.NoError(t, database.Delete(records[2]).Error)
	require.NoError(t, database.Delete(records[3]).Error)

	counts, err := store.CountByTableIDs(ctx, []int64{1, 1, 2, 99})
	require.NoError(t, err)
	require.Equal(t, map[int64]int64{1: 2}, counts, "count only live records within the requested tables")
	require.Zero(t, counts[2], "tables with only deleted records must have zero records")
	require.Zero(t, counts[99], "missing tables must have zero records")

	counts, err = NewSLRecordsStore(nil).CountByTableIDs(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, counts, "an empty table list must not query the database")

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	counts, err = store.CountByTableIDs(canceled, []int64{1})
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, counts)
}
