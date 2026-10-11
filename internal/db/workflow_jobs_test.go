package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestWorkflowJobsResetStale(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t, &WorkflowJob{})
	store := NewWorkflowJobsStore(gormDB)

	jobs := []*WorkflowJob{
		{WorkflowID: 1, RunID: 1, FlowID: "main", Event: "record_created", Payload: datatypes.JSON(`{}`)},
		{WorkflowID: 1, RunID: 2, FlowID: "main", Event: "record_created", Payload: datatypes.JSON(`{}`)},
	}
	require.NoError(t, store.Create(ctx, jobs))
	for range jobs {
		claimed, err := store.Claim(ctx)
		require.NoError(t, err)
		require.NotNil(t, claimed)
	}
	// The second job has used up its attempts before its lease expires.
	require.NoError(t, gormDB.Model(&WorkflowJob{}).Where("id = ?", jobs[1].ID).Update("attempts", 3).Error)

	n, failed, err := store.ResetStale(ctx, time.Minute, 3)
	require.NoError(t, err)
	require.Zero(t, n)
	require.Empty(t, failed)

	n, failed, err = store.ResetStale(ctx, -time.Minute, 3)
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	require.Len(t, failed, 1)
	require.Equal(t, jobs[1].ID, failed[0].ID)
	require.Equal(t, int64(2), failed[0].RunID)

	var stored []WorkflowJob
	require.NoError(t, gormDB.Order("id ASC").Find(&stored).Error)
	require.Equal(t, WorkflowJobPending, stored[0].Status)
	require.Equal(t, WorkflowJobFailed, stored[1].Status)
	require.True(t, stored[1].Uncertain)
	require.Equal(t, WorkflowJobInterruptedMessage, stored[1].Error)
	require.NotNil(t, stored[1].FinishedAt)
}
