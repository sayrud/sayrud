package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMigrateProjectMembers(t *testing.T) {
	for _, withID := range []bool{false, true} {
		name, idColumn := "without id", ""
		if withID {
			name, idColumn = "with id", "id bigserial,"
		}
		t.Run(name, func(t *testing.T) {
			database := newTestDB(t)
			require.NoError(t, database.Exec(`CREATE TABLE project_members (`+idColumn+`
				project_id bigint, user_id bigint, role varchar(16),
				created_at timestamptz, updated_at timestamptz,
				CONSTRAINT "legacy members key" PRIMARY KEY (project_id, user_id)
			)`).Error)
			if withID {
				require.NoError(t, database.Exec(`CREATE UNIQUE INDEX idx_project_members_project_user ON project_members (project_id, user_id)`).Error)
			}
			require.NoError(t, database.Exec(`INSERT INTO project_members (project_id, user_id, role, created_at, updated_at)
				VALUES (1, 2, 'editor', now(), now()), (1, 3, 'viewer', now(), now())`).Error)

			migrate := func(tx *gorm.DB) error {
				if err := migrateProjectMembers(tx); err != nil {
					return err
				}
				return tx.AutoMigrate(&ProjectMember{})
			}
			require.NoError(t, database.Transaction(migrate))

			ctx := context.Background()
			store := NewProjectMembersStore(database)
			members, err := store.ListByProjectID(ctx, 1)
			require.NoError(t, err)
			require.Len(t, members, 2)
			require.Equal(t, int64(1), members[0].ID)
			require.Equal(t, int64(2), members[1].ID)
			require.Equal(t, int64(2), members[0].UserID)
			require.Equal(t, ProjectRoleEditor, members[0].Role)
			require.False(t, members[0].CreatedAt.IsZero())
			require.False(t, members[0].UpdatedAt.IsZero())
			require.Equal(t, int64(3), members[1].UserID)
			require.Equal(t, ProjectRoleViewer, members[1].Role)

			var primaryKey string
			require.NoError(t, database.Raw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
				WHERE conrelid = 'project_members'::regclass AND contype = 'p'`).Scan(&primaryKey).Error)
			require.Equal(t, "PRIMARY KEY (id)", primaryKey)

			require.NoError(t, store.Set(ctx, 1, 2, ProjectRoleManager))
			role, err := store.GetRole(ctx, 1, 2)
			require.NoError(t, err)
			require.Equal(t, ProjectRoleManager, role)
			require.NoError(t, store.Delete(ctx, 1, 2))
			_, err = store.GetRole(ctx, 1, 2)
			require.ErrorIs(t, err, ErrProjectMemberNotFound)
			require.NoError(t, store.Set(ctx, 1, 2, ProjectRoleEditor))
			members, err = store.ListByProjectID(ctx, 1)
			require.NoError(t, err)
			require.Len(t, members, 2)
			require.NotEqual(t, int64(1), members[1].ID)
			require.Equal(t, int64(2), members[1].UserID)
			require.Equal(t, ProjectRoleEditor, members[1].Role)

			// A restart preserves the re-added member and its deleted predecessor.
			require.NoError(t, database.Transaction(migrate))
			var count int64
			require.NoError(t, database.Unscoped().Model(&ProjectMember{}).Count(&count).Error)
			require.Equal(t, int64(3), count)
		})
	}
}

func TestMigrateProjectMembersFreshDatabase(t *testing.T) {
	database := newTestDB(t)
	for range 2 {
		require.NoError(t, database.Transaction(func(tx *gorm.DB) error {
			if err := migrateProjectMembers(tx); err != nil {
				return err
			}
			return tx.AutoMigrate(&ProjectMember{})
		}))
	}
}

func TestMigrateProjectMembersRollback(t *testing.T) {
	database := newTestDB(t)
	require.NoError(t, database.Exec(`CREATE TABLE project_members (
		id bigint, project_id bigint, user_id bigint,
		PRIMARY KEY (project_id, user_id)
	)`).Error)
	require.NoError(t, database.Exec(`INSERT INTO project_members VALUES (1, 1, 2), (1, 1, 3)`).Error)

	// Invalid existing IDs must fail without changing the old primary key or rows.
	require.Error(t, database.Transaction(migrateProjectMembers))
	var primaryKey string
	require.NoError(t, database.Raw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conrelid = 'project_members'::regclass AND contype = 'p'`).Scan(&primaryKey).Error)
	require.Equal(t, "PRIMARY KEY (project_id, user_id)", primaryKey)
	var count int64
	require.NoError(t, database.Table("project_members").Count(&count).Error)
	require.Equal(t, int64(2), count)
}
