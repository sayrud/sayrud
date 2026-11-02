package db

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBots(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t, &User{}, &UserSession{}, &AccessToken{}, &Setting{}, &UserIdentity{}, &Project{}, &ProjectMember{})
	users := NewUsersStore(gormDB)
	tokens := NewAccessTokensStore(gormDB)

	_, err := users.CreateBot(ctx, "Agent", 0)
	require.ErrorIs(t, err, ErrBotOwnerRequired)

	alice, err := users.Create(ctx, CreateUserOptions{Email: "alice@example.com", UserName: "Alice"})
	require.NoError(t, err)
	bot, err := users.CreateBot(ctx, "Agent", alice.ID)
	require.NoError(t, err)
	assert.True(t, bot.IsBot())
	assert.Equal(t, alice.ID, bot.OwnerID)
	assert.True(t, strings.HasSuffix(bot.Email, "@"+BotEmailDomain))
	assert.False(t, bot.HasPassword())
	another, err := users.CreateBot(ctx, "Agent", alice.ID)
	require.NoError(t, err)
	assert.NotEqual(t, bot.Email, another.Email)

	require.NoError(t, users.EnsureAdmin(ctx))
	got, err := users.GetByID(ctx, alice.ID)
	require.NoError(t, err)
	assert.True(t, got.IsAdmin, "the first human becomes the admin")
	got, err = users.GetByID(ctx, bot.ID)
	require.NoError(t, err)
	assert.False(t, got.IsAdmin)

	require.ErrorIs(t, users.SetAdmin(ctx, bot.ID, true), ErrUserIsBot)
	require.NoError(t, users.UpdatePassword(ctx, bot.ID, "password123"))
	_, err = users.Authenticate(ctx, bot.Email, "password123")
	require.ErrorIs(t, err, ErrBadCredential, "bots never sign in with a password")

	stats, err := users.Stats(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.Total)
	recent, err := users.Recent(ctx, 5)
	require.NoError(t, err)
	require.Len(t, recent, 1)
	assert.Equal(t, alice.ID, recent[0].ID)

	list, total, err := users.List(ctx, ListUsersOptions{Kind: UserKindBot})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Len(t, list, 2)
	_, total, err = users.List(ctx, ListUsersOptions{Kind: UserKindHuman})
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)

	_, plaintext, err := tokens.Create(ctx, CreateAccessTokenOptions{UserID: bot.ID, Name: "CI", Scopes: []AccessTokenScope{AccessTokenScopeRead}, CreatedBy: alice.ID})
	require.NoError(t, err)
	_, kept, err := tokens.Create(ctx, CreateAccessTokenOptions{UserID: another.ID, Name: "CI", Scopes: []AccessTokenScope{AccessTokenScopeRead}, CreatedBy: alice.ID})
	require.NoError(t, err)
	require.NoError(t, users.Delete(ctx, bot.ID, 0))
	_, err = tokens.Verify(ctx, plaintext)
	require.ErrorIs(t, err, ErrAccessTokenNotFound, "deleting the bot revokes its tokens")
	_, err = tokens.Verify(ctx, kept)
	require.NoError(t, err)
}

func TestBotOwners(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t, &User{}, &UserSession{}, &AccessToken{}, &Setting{}, &UserIdentity{}, &Project{}, &ProjectMember{})
	users := NewUsersStore(gormDB)
	tokens := NewAccessTokensStore(gormDB)

	_, err := users.CreateFirstAdmin(ctx, CreateUserOptions{Email: "admin@example.com", UserName: "Admin", Password: "password123"})
	require.NoError(t, err)
	alice, err := users.Create(ctx, CreateUserOptions{Email: "alice@example.com", UserName: "Alice"})
	require.NoError(t, err)
	bob, err := users.Create(ctx, CreateUserOptions{Email: "bob@example.com", UserName: "Bob"})
	require.NoError(t, err)

	mine, err := users.CreateBot(ctx, "Alice agent", alice.ID)
	require.NoError(t, err)
	theirs, err := users.CreateBot(ctx, "Bob agent", bob.ID)
	require.NoError(t, err)

	count, err := users.CountBotsByOwner(ctx, alice.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)
	own, _, err := users.List(ctx, ListUsersOptions{Kind: UserKindBot, OwnerID: alice.ID})
	require.NoError(t, err)
	require.Len(t, own, 1)
	assert.Equal(t, mine.ID, own[0].ID)

	_, mineToken, err := tokens.Create(ctx, CreateAccessTokenOptions{UserID: mine.ID, Name: "CI", Scopes: []AccessTokenScope{AccessTokenScopeRead}, CreatedBy: alice.ID})
	require.NoError(t, err)
	_, theirToken, err := tokens.Create(ctx, CreateAccessTokenOptions{UserID: theirs.ID, Name: "CI", Scopes: []AccessTokenScope{AccessTokenScopeRead}, CreatedBy: bob.ID})
	require.NoError(t, err)
	require.NoError(t, gormDB.Create(&ProjectMember{ProjectID: 9, UserID: mine.ID, Role: ProjectRoleEditor}).Error)

	require.NoError(t, users.SetBotOwner(ctx, theirs.ID, alice.ID))
	moved, err := users.GetByID(ctx, theirs.ID)
	require.NoError(t, err)
	assert.Equal(t, alice.ID, moved.OwnerID)
	require.ErrorIs(t, users.SetBotOwner(ctx, alice.ID, bob.ID), ErrUserNotFound, "only bots have owners")
	require.ErrorIs(t, users.SetBotOwner(ctx, mine.ID, 0), ErrBotOwnerRequired)

	counts, err := users.CountBotsByOwners(ctx, []int64{alice.ID, bob.ID})
	require.NoError(t, err)
	assert.Equal(t, map[int64]int64{alice.ID: 2}, counts)

	require.NoError(t, users.Delete(ctx, alice.ID, 0))
	for _, id := range []int64{mine.ID, theirs.ID} {
		_, err = users.GetByID(ctx, id)
		require.ErrorIs(t, err, ErrUserNotFound, "the bots are deleted with their owner")
	}
	for _, token := range []string{mineToken, theirToken} {
		_, err = tokens.Verify(ctx, token)
		require.ErrorIs(t, err, ErrAccessTokenNotFound)
	}
	var memberships int64
	require.NoError(t, gormDB.Model(&ProjectMember{}).Where("user_id = ?", mine.ID).Count(&memberships).Error)
	assert.Zero(t, memberships)
}

func TestAssignBotOwners(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t, &User{}, &Project{}, &ProjectMember{})
	users := NewUsersStore(gormDB)

	orphan := &User{Email: "orphan@" + BotEmailDomain, UserName: "Orphan", Kind: UserKindBot}
	require.NoError(t, gormDB.Create(orphan).Error)
	require.NoError(t, assignBotOwners(gormDB), "nothing to do without an admin")
	got, err := users.GetByID(ctx, orphan.ID)
	require.NoError(t, err)
	assert.Zero(t, got.OwnerID)

	admin, err := users.CreateFirstAdmin(ctx, CreateUserOptions{Email: "admin@example.com", UserName: "Admin", Password: "password123"})
	require.NoError(t, err)
	project := &Project{UID: "base", Name: "Base", OwnerUserID: orphan.ID}
	require.NoError(t, gormDB.Create(project).Error)
	require.NoError(t, gormDB.Create(&ProjectMember{ProjectID: project.ID, UserID: admin.ID, Role: ProjectRoleViewer}).Error)

	require.NoError(t, assignBotOwners(gormDB))
	got, err = users.GetByID(ctx, orphan.ID)
	require.NoError(t, err)
	assert.Equal(t, admin.ID, got.OwnerID)

	var owned Project
	require.NoError(t, gormDB.First(&owned, project.ID).Error)
	assert.Equal(t, admin.ID, owned.OwnerUserID, "bots hand their projects to their owners")
	role, err := NewProjectMembersStore(gormDB).GetRole(ctx, project.ID, orphan.ID)
	require.NoError(t, err)
	assert.Equal(t, ProjectRoleManager, role)
	require.NoError(t, assignBotOwners(gormDB), "it is idempotent")
}
