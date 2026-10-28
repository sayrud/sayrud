package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TestProjectNavigationFreshDatabase(t *testing.T) {
	database := newTestDB(t)
	require.NoError(t, database.Transaction(func(tx *gorm.DB) error {
		if err := initializeProjectNavigationPositions(tx); err != nil {
			return err
		}

		return tx.AutoMigrate(&Project{})
	}))

	ctx := context.Background()
	project, err := NewProjectsStore(database).Create(ctx, CreateProjectOptions{})
	require.NoError(t, err)

	table, err := NewSLTablesStore(database).Create(ctx, project.ID, CreateSLTableOptions{})
	require.NoError(t, err)

	workflow, err := NewWorkflowsStore(database).Create(ctx, CreateWorkflowOptions{ProjectID: project.ID, Draft: []byte(`{}`)})
	require.NoError(t, err)

	require.Equal(t, 0, table.Position)
	require.Equal(t, 1, workflow.Position)
}

func TestInitializeProjectNavigationPositions(t *testing.T) {
	database := newTestDB(t, &Project{}, &SLTable{}, &Workflow{})
	ctx := context.Background()
	project, err := NewProjectsStore(database).Create(ctx, CreateProjectOptions{Name: "Existing"})
	require.NoError(t, err)

	other, err := NewProjectsStore(database).Create(ctx, CreateProjectOptions{Name: "Other"})
	require.NoError(t, err)

	for _, item := range []*SLTable{
		{UID: "tblOne", ProjectID: project.ID}, {UID: "tblTwo", ProjectID: project.ID},
		{UID: "tblDeleted", ProjectID: project.ID}, {UID: "tblOther", ProjectID: other.ID},
	} {
		require.NoError(t, database.Create(item).Error)
	}

	require.NoError(t, database.Where("uid = ?", "tblDeleted").Delete(&SLTable{}).Error)

	for _, uid := range []string{"wflOld", "wflNew"} {
		require.NoError(t, database.Create(&Workflow{UID: uid, ProjectID: project.ID, Draft: datatypes.JSON(`{}`)}).Error)
	}

	require.NoError(t, database.Migrator().DropColumn(&SLTable{}, "Position"))
	require.NoError(t, database.Migrator().DropColumn(&Workflow{}, "Position"))
	require.NoError(t, database.Transaction(initializeProjectNavigationPositions))

	want := []string{"tblOne", "tblTwo", "wflNew", "wflOld"}
	require.Equal(t, want, navigationUIDs(t, database, project.ID))
	require.Equal(t, []string{"tblOther"}, navigationUIDs(t, database, other.ID))

	items, err := listProjectNavigation(ctx, database, project.ID)
	require.NoError(t, err)
	for i, item := range items {
		require.Equal(t, i, item.Position)
	}

	_, err = NewProjectsStore(database).MoveNavigation(ctx, project.ID, want[0], want[3], true)
	require.NoError(t, err)

	before := navigationUIDs(t, database, project.ID)
	require.NoError(t, database.Transaction(initializeProjectNavigationPositions))
	require.Equal(t, before, navigationUIDs(t, database, project.ID), "a restart must not reset positions")
}
