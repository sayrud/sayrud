package db

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func navigationFixture(t *testing.T) (*gorm.DB, *Project, []string) {
	t.Helper()
	ctx := context.Background()
	database := newTestDB(t, &Project{}, &SLTable{}, &Workflow{})
	project, err := NewProjectsStore(database).Create(ctx, CreateProjectOptions{Name: "Navigation"})
	require.NoError(t, err)

	var uids []string
	for i := range 2 {
		table, err := NewSLTablesStore(database).Create(ctx, project.ID, CreateSLTableOptions{Name: fmt.Sprint(i)})
		require.NoError(t, err)

		workflow, err := NewWorkflowsStore(database).Create(ctx, CreateWorkflowOptions{ProjectID: project.ID, Draft: []byte(`{}`)})
		require.NoError(t, err)

		require.Equal(t, i*2, table.Position)
		require.Equal(t, i*2+1, workflow.Position)
		uids = append(uids, table.UID, workflow.UID)
	}

	return database, project, uids
}

func navigationUIDs(t *testing.T, database *gorm.DB, projectID int64) []string {
	t.Helper()
	items, err := listProjectNavigation(context.Background(), database, projectID)
	require.NoError(t, err)

	uids := make([]string, len(items))
	for i, item := range items {
		uids[i] = item.UID
	}

	return uids
}

func TestMoveProjectNavigation(t *testing.T) {
	database, project, uids := navigationFixture(t)
	ctx := context.Background()
	store := NewProjectsStore(database)

	for _, tc := range []struct {
		name         string
		from, target int
		after        bool
		want         []int
	}{
		{"move workflow to first", 3, 0, false, []int{3, 0, 1, 2}},
		{"move table to last", 0, 3, true, []int{1, 2, 3, 0}},
		{"move down before target", 0, 2, false, []int{1, 0, 2, 3}},
		{"move up after target", 3, 1, true, []int{0, 1, 3, 2}},
		{"adjacent before is unchanged", 0, 1, false, []int{0, 1, 2, 3}},
		{"same item is unchanged", 1, 1, true, []int{0, 1, 2, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items, err := listProjectNavigation(ctx, database, project.ID)
			require.NoError(t, err)

			for i := range items {
				for position, uid := range uids {
					if items[i].UID == uid {
						items[i].Position = position
					}
				}
			}
			require.NoError(t, writeNavigationPositions(ctx, database, items))

			got, err := store.MoveNavigation(ctx, project.ID, uids[tc.from], uids[tc.target], tc.after)
			require.NoError(t, err)

			want := make([]string, len(tc.want))
			for i, index := range tc.want {
				want[i] = uids[index]
				require.Equal(t, NavigationPosition{UID: want[i], Position: i}, got[i])
			}
			require.Equal(t, want, navigationUIDs(t, database, project.ID))
		})
	}

	before := navigationUIDs(t, database, project.ID)
	foreign := &SLTable{UID: "tblForeign", ProjectID: project.ID + 1}
	deleted := &Workflow{UID: "wflDeleted", ProjectID: project.ID, Draft: datatypes.JSON(`{}`)}
	require.NoError(t, database.Create(foreign).Error)
	require.NoError(t, database.Create(deleted).Error)
	require.NoError(t, database.Delete(deleted).Error)

	for _, pair := range [][2]string{{"missing", uids[0]}, {uids[0], "missing"}, {foreign.UID, uids[0]}, {uids[0], foreign.UID}, {deleted.UID, uids[0]}} {
		_, err := store.MoveNavigation(ctx, project.ID, pair[0], pair[1], false)
		require.ErrorIs(t, err, ErrNavigationItemNotFound)
		require.Equal(t, before, navigationUIDs(t, database, project.ID))
	}

	_, err := store.MoveNavigation(ctx, project.ID+100, uids[0], uids[1], false)
	require.ErrorIs(t, err, ErrProjectNotFound)
}

func TestProjectNavigationMovesUseLatestOrderAndRollbackTogether(t *testing.T) {
	database, project, uids := navigationFixture(t)
	ctx := context.Background()
	store := NewProjectsStore(database)

	_, err := store.MoveNavigation(ctx, project.ID, uids[3], uids[1], false)
	require.NoError(t, err)

	_, err = store.MoveNavigation(ctx, project.ID, uids[0], uids[2], true)
	require.NoError(t, err)
	require.Equal(t, []string{uids[3], uids[1], uids[2], uids[0]}, navigationUIDs(t, database, project.ID))

	// A failure on the second table must roll back the first table's position updates.
	require.NoError(t, database.Exec("ALTER TABLE workflows ADD CONSTRAINT keep_workflow_positions CHECK (position IN (0, 1))").Error)
	before, err := listProjectNavigation(ctx, database, project.ID)
	require.NoError(t, err)

	_, err = store.MoveNavigation(ctx, project.ID, uids[3], uids[0], true)
	require.Error(t, err)

	after, err := listProjectNavigation(ctx, database, project.ID)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestProjectNavigationConcurrentCreatesAndMoves(t *testing.T) {
	database, project, uids := navigationFixture(t)
	ctx := context.Background()
	const count = 12
	var wg sync.WaitGroup
	errors := make(chan error, count)

	for i := range count {
		wg.Go(func() {
			if i%2 == 0 {
				_, err := NewSLTablesStore(database).Create(ctx, project.ID, CreateSLTableOptions{})
				errors <- err
			} else {
				_, err := NewWorkflowsStore(database).Create(ctx, CreateWorkflowOptions{ProjectID: project.ID, Draft: []byte(`{}`)})
				errors <- err
			}
		})
	}
	wg.Wait()
	for range count {
		require.NoError(t, <-errors)
	}

	for i := range count {
		wg.Go(func() {
			_, err := NewProjectsStore(database).MoveNavigation(ctx, project.ID, uids[i%len(uids)], uids[(i+1)%len(uids)], i%2 == 0)
			errors <- err
		})
	}
	wg.Wait()
	for range count {
		require.NoError(t, <-errors)
	}

	items, err := listProjectNavigation(ctx, database, project.ID)
	require.NoError(t, err)
	require.Len(t, items, count+len(uids))
	for i, item := range items {
		require.Equal(t, i, item.Position)
	}

	// Deleting an item leaves a valid gap; the next create appends beyond both item types.
	for _, item := range items[:len(items)-1] {
		if item.Kind == 0 {
			require.NoError(t, NewSLTablesStore(database).DeleteByID(ctx, item.ID))
			break
		}
	}

	created, err := NewSLTablesStore(database).Create(ctx, project.ID, CreateSLTableOptions{})
	require.NoError(t, err)
	require.Equal(t, count+len(uids), created.Position)
}
