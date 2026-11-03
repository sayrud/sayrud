package routeutil

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thanhpk/randstr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/wuhan005/sayrud/internal/db"
)

// newTestDB connects to the PostgreSQL of the PG* environment variables in a temporary schema, and skips the test if not configured.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	if os.Getenv("PGHOST") == "" {
		t.Skip("PGHOST is not set")
	}
	admin, err := gorm.Open(postgres.New(postgres.Config{DSN: "", PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	schema := "test_" + strings.ToLower(randstr.String(10))
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	gormDB, err := gorm.Open(postgres.New(postgres.Config{DSN: "search_path=" + schema, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := gormDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, gormDB.AutoMigrate(&db.User{}, &db.Project{}, &db.ProjectMember{}, &db.SLTable{}, &db.SLField{}, &db.Workflow{}))
	return gormDB
}

func TestValidateMembers(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t)
	users := db.NewUsersStore(gormDB)
	owner, err := users.Create(ctx, db.CreateUserOptions{Email: "owner@example.com", UserName: "Owner", Password: "password1"})
	require.NoError(t, err)
	member, err := users.Create(ctx, db.CreateUserOptions{Email: "member@example.com", UserName: "Member", Password: "password1"})
	require.NoError(t, err)
	outsider, err := users.Create(ctx, db.CreateUserOptions{Email: "outsider@example.com", UserName: "Outsider", Password: "password1"})
	require.NoError(t, err)
	bot, err := users.CreateBot(ctx, "Builder", owner.ID)
	require.NoError(t, err)

	project, err := db.NewProjectsStore(gormDB).Create(ctx, db.CreateProjectOptions{OwnerUserID: owner.ID, Name: "Tasks"})
	require.NoError(t, err)
	members := db.NewProjectMembersStore(gormDB)
	require.NoError(t, members.Set(ctx, project.ID, member.ID, db.ProjectRoleEditor))
	require.NoError(t, members.Set(ctx, project.ID, bot.ID, db.ProjectRoleEditor))
	table, err := db.NewSLTablesStore(gormDB).Create(ctx, project.ID, db.CreateSLTableOptions{Name: "Tasks"})
	require.NoError(t, err)
	_, err = db.NewSLFieldsStore(gormDB).Create(ctx, db.CreateSLFieldOptions{SLTableID: table.ID, UID: "fldTeam0001", Label: "Team", Type: db.MemberFieldType, Metadata: map[string]interface{}{"multiple": true}})
	require.NoError(t, err)

	value := func(ids ...int64) map[string]interface{} {
		list := make([]interface{}, 0, len(ids))
		for _, id := range ids {
			list = append(list, float64(id))
		}
		return map[string]interface{}{"fldTeam0001": list}
	}
	_, err = Validate(ctx, gormDB, table.ID, value(owner.ID, member.ID, bot.ID), nil)
	require.NoError(t, err, "the owner, the collaborators and the bots are members")
	_, err = Validate(ctx, gormDB, table.ID, value(outsider.ID), nil)
	require.ErrorIs(t, err, ErrInvalidMember)

	require.NoError(t, members.Delete(ctx, project.ID, member.ID))
	previous, err := json.Marshal(value(owner.ID, member.ID))
	require.NoError(t, err)
	_, err = Validate(ctx, gormDB, table.ID, value(member.ID), previous)
	require.NoError(t, err, "the members who left are kept in the cells already having them")
	_, err = Validate(ctx, gormDB, table.ID, value(member.ID), nil)
	require.ErrorIs(t, err, ErrInvalidMember)

	current, err := members.UserIDs(ctx, project.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{owner.ID, bot.ID}, current)
	former, err := members.FormerUserIDs(ctx, project.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{member.ID}, former)

	directory, err := LoadDirectory(ctx, gormDB, project.ID)
	require.NoError(t, err)
	assert.True(t, directory.IsMember(bot.ID))
	assert.Equal(t, "Member", directory.UserName(member.ID), "the users who left are loaded on demand")
	assert.Equal(t, "", directory.UserName(9999))
}
