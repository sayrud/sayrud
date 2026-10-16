package db

import (
	"context"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

func TestWorkflowRunsListFilters(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t, &WorkflowRun{})
	store := NewWorkflowRunsStore(gormDB)

	base := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	finished := base.Add(time.Hour)
	runs := []*WorkflowRun{
		{WorkflowID: 1, Trigger: WorkflowRunTriggerHTTP, Status: WorkflowRunSucceeded, DurationMs: 200, CreatedAt: base, FinishedAt: &finished},
		{WorkflowID: 1, Trigger: WorkflowRunTriggerHTTP, Status: WorkflowRunFailed, DurationMs: 3000, CreatedAt: base.Add(24 * time.Hour), FinishedAt: &finished},
		{WorkflowID: 1, Trigger: WorkflowRunTriggerHTTP, Status: WorkflowRunRunning, CreatedAt: base.Add(48 * time.Hour)},
		{WorkflowID: 2, Trigger: WorkflowRunTriggerHTTP, Status: WorkflowRunSucceeded, DurationMs: 200, CreatedAt: base, FinishedAt: &finished},
	}
	require.NoError(t, store.Create(ctx, runs...))

	list := func(opts ListWorkflowRunsOptions) []int64 {
		got, err := store.List(ctx, 1, opts)
		require.NoError(t, err)
		return lo.Map(got, func(r *WorkflowRun, _ int) int64 { return r.ID })
	}

	require.Equal(t, []int64{runs[2].ID, runs[1].ID, runs[0].ID}, list(ListWorkflowRunsOptions{}))
	require.Equal(t, []int64{runs[2].ID, runs[0].ID}, list(ListWorkflowRunsOptions{Statuses: []WorkflowRunStatus{WorkflowRunSucceeded, WorkflowRunRunning}}))
	// The duration filters match finished runs only.
	require.Equal(t, []int64{runs[0].ID}, list(ListWorkflowRunsOptions{MaxDurationMs: 1000}))
	require.Equal(t, []int64{runs[1].ID}, list(ListWorkflowRunsOptions{MinDurationMs: 1000}))
	require.Equal(t, []int64{runs[1].ID}, list(ListWorkflowRunsOptions{CreatedFrom: base.Add(24 * time.Hour), CreatedTo: base.Add(48 * time.Hour)}))
}
