package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectAppearancePartialUpdates(t *testing.T) {
	ctx := context.Background()
	store := NewProjectsStore(newTestDB(t, &Project{}))

	project, err := store.Create(ctx, CreateProjectOptions{OwnerUserID: 1, Name: "Original"})
	require.NoError(t, err)

	require.Empty(t, project.Icon)
	require.Empty(t, project.Color)

	icon, color, renamed, empty := "calendar", "purple", "Renamed", ""
	require.NoError(t, store.Update(ctx, project.ID, UpdateProjectOptions{Icon: &icon, Color: &color}))
	got, err := store.GetByUID(ctx, project.UID)
	require.NoError(t, err)

	require.Equal(t, "Original", got.Name)
	require.Equal(t, icon, got.Icon)
	require.Equal(t, color, got.Color)

	require.NoError(t, store.Update(ctx, project.ID, UpdateProjectOptions{Name: &renamed}))
	got, err = store.GetByID(ctx, project.ID)
	require.NoError(t, err)

	require.Equal(t, renamed, got.Name)
	require.Equal(t, icon, got.Icon)
	require.Equal(t, color, got.Color)

	color = "teal"
	require.NoError(t, store.Update(ctx, project.ID, UpdateProjectOptions{Color: &color}))
	got, err = store.GetByID(ctx, project.ID)
	require.NoError(t, err)

	require.Equal(t, renamed, got.Name)
	require.Equal(t, icon, got.Icon)
	require.Equal(t, color, got.Color)

	require.NoError(t, store.Update(ctx, project.ID, UpdateProjectOptions{Icon: &empty, Color: &empty}))
	got, err = store.GetByID(ctx, project.ID)
	require.NoError(t, err)

	require.Equal(t, renamed, got.Name)
	require.Empty(t, got.Icon)
	require.Empty(t, got.Color)
}

func TestTableAppearancePartialUpdates(t *testing.T) {
	ctx := context.Background()
	store := NewSLTablesStore(newTestDB(t, &SLTable{}))

	table, err := store.Create(ctx, 1, CreateSLTableOptions{Name: "Original"})
	require.NoError(t, err)

	require.Empty(t, table.Icon)
	require.Empty(t, table.Color)

	icon, color, renamed, empty := "calendar", "purple", "Renamed", ""
	require.NoError(t, store.Update(ctx, table.ID, UpdateSLTableOptions{Icon: &icon, Color: &color}))
	got, err := store.GetByUID(ctx, table.UID)
	require.NoError(t, err)

	require.Equal(t, "Original", got.Name)
	require.Equal(t, icon, got.Icon)
	require.Equal(t, color, got.Color)

	require.NoError(t, store.Update(ctx, table.ID, UpdateSLTableOptions{Name: &renamed}))
	got, err = store.GetByID(ctx, table.ID)
	require.NoError(t, err)

	require.Equal(t, renamed, got.Name)
	require.Equal(t, icon, got.Icon)
	require.Equal(t, color, got.Color)

	color = "teal"
	require.NoError(t, store.Update(ctx, table.ID, UpdateSLTableOptions{Color: &color}))
	got, err = store.GetByID(ctx, table.ID)
	require.NoError(t, err)

	require.Equal(t, renamed, got.Name)
	require.Equal(t, icon, got.Icon)
	require.Equal(t, color, got.Color)

	require.NoError(t, store.Update(ctx, table.ID, UpdateSLTableOptions{Icon: &empty, Color: &empty}))
	got, err = store.GetByID(ctx, table.ID)
	require.NoError(t, err)

	require.Equal(t, renamed, got.Name)
	require.Empty(t, got.Icon)
	require.Empty(t, got.Color)
}
