package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

func TestNormalizeAccessTokenScopes(t *testing.T) {
	scopes, ok := NormalizeAccessTokenScopes([]string{"events", "read", "events"})
	require.True(t, ok)
	assert.Equal(t, []AccessTokenScope{AccessTokenScopeRead, AccessTokenScopeEvents}, scopes)

	_, ok = NormalizeAccessTokenScopes(nil)
	assert.False(t, ok)
	_, ok = NormalizeAccessTokenScopes([]string{"read", "admin"})
	assert.False(t, ok)
}

func TestAccessTokens(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t, &AccessToken{})
	store := NewAccessTokensStore(gormDB)

	token, plaintext, err := store.Create(ctx, CreateAccessTokenOptions{
		UserID: 7, Name: "Agent", Scopes: []AccessTokenScope{AccessTokenScopeRead}, CreatedBy: 1,
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(plaintext, "sat_"))
	assert.Len(t, plaintext, 44)
	assert.Equal(t, plaintext[:12], token.Prefix)
	assert.NotContains(t, token.Hash, plaintext)

	got, err := store.Verify(ctx, plaintext)
	require.NoError(t, err)
	assert.Equal(t, token.ID, got.ID)
	assert.True(t, got.HasScope(AccessTokenScopeRead))
	assert.False(t, got.HasScope(AccessTokenScopeWrite))
	assert.True(t, got.AllowsProject(42), "an unrestricted token follows the memberships")
	assert.NotNil(t, got.ProjectIDs)

	for _, wrong := range []string{"", "sk_" + plaintext[4:], plaintext + "x", strings.ToUpper(plaintext)} {
		_, err = store.Verify(ctx, wrong)
		require.ErrorIs(t, err, ErrAccessTokenNotFound, wrong)
	}

	t.Run("touch at most once a minute", func(t *testing.T) {
		require.NoError(t, store.Touch(ctx, got, "10.0.0.1"))
		first, err := store.Verify(ctx, plaintext)
		require.NoError(t, err)
		require.NotNil(t, first.LastUsedAt)
		assert.Equal(t, "10.0.0.1", first.LastUsedIP)

		require.NoError(t, store.Touch(ctx, got, "10.0.0.2"), "a stale copy still does not write within the interval")
		second, err := store.Verify(ctx, plaintext)
		require.NoError(t, err)
		assert.Equal(t, "10.0.0.1", second.LastUsedIP)

		past := dbutil.Now().Add(-2 * time.Minute)
		require.NoError(t, gormDB.Model(&AccessToken{}).Where("id = ?", token.ID).Update("last_used_at", past).Error)
		second.LastUsedAt = &past
		require.NoError(t, store.Touch(ctx, second, "10.0.0.3"))
		third, err := store.Verify(ctx, plaintext)
		require.NoError(t, err)
		assert.Equal(t, "10.0.0.3", third.LastUsedIP)
	})

	t.Run("expired and restricted", func(t *testing.T) {
		past := dbutil.Now().Add(-time.Second)
		_, expired, err := store.Create(ctx, CreateAccessTokenOptions{UserID: 7, Name: "Old", Scopes: []AccessTokenScope{AccessTokenScopeRead}, ExpiresAt: &past})
		require.NoError(t, err)
		_, err = store.Verify(ctx, expired)
		require.ErrorIs(t, err, ErrAccessTokenExpired)

		future := dbutil.Now().Add(time.Hour)
		_, restricted, err := store.Create(ctx, CreateAccessTokenOptions{
			UserID: 7, Name: "Scoped", Scopes: []AccessTokenScope{AccessTokenScopeRead, AccessTokenScopeWrite}, ProjectIDs: []int64{3}, ExpiresAt: &future,
		})
		require.NoError(t, err)
		got, err := store.Verify(ctx, restricted)
		require.NoError(t, err)
		assert.True(t, got.AllowsProject(3))
		assert.False(t, got.AllowsProject(4))
	})

	t.Run("list, latest and delete", func(t *testing.T) {
		other, _, err := store.Create(ctx, CreateAccessTokenOptions{UserID: 8, Name: "Other", Scopes: []AccessTokenScope{AccessTokenScopeRead}})
		require.NoError(t, err)

		list, err := store.ListByUserID(ctx, 7)
		require.NoError(t, err)
		require.Len(t, list, 3)
		assert.Equal(t, "Scoped", list[0].Name, "the newest comes first")

		latest, err := store.LatestByUserIDs(ctx, []int64{7, 8, 9})
		require.NoError(t, err)
		assert.Equal(t, "Scoped", latest[7].Name)
		assert.Equal(t, other.ID, latest[8].ID)
		assert.NotContains(t, latest, int64(9))

		require.ErrorIs(t, store.Delete(ctx, 8, token.ID), ErrAccessTokenNotFound, "another user can not revoke it")
		require.NoError(t, store.Delete(ctx, 7, token.ID))
		_, err = store.Verify(ctx, plaintext)
		require.ErrorIs(t, err, ErrAccessTokenNotFound)

		require.NoError(t, store.DeleteOthers(ctx, 7, latest[7].ID))
		list, err = store.ListByUserID(ctx, 7)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.Equal(t, latest[7].ID, list[0].ID)
	})

	t.Run("update access and rotate", func(t *testing.T) {
		bot, secret, err := store.Create(ctx, CreateAccessTokenOptions{UserID: 20, Name: "Bot", Scopes: []AccessTokenScope{AccessTokenScopeRead}})
		require.NoError(t, err)
		require.NoError(t, store.Touch(ctx, bot, "10.0.0.9"))

		future := dbutil.Now().Add(24 * time.Hour)
		require.NoError(t, store.UpdateAccess(ctx, bot.ID, AccessTokenAccess{
			Scopes: []AccessTokenScope{AccessTokenScopeRead, AccessTokenScopeWrite}, ProjectIDs: []int64{5}, ExpiresAt: &future,
		}))
		got, err := store.Verify(ctx, secret)
		require.NoError(t, err, "changing the access keeps the secret")
		assert.True(t, got.HasScope(AccessTokenScopeWrite))
		assert.Equal(t, []int64{5}, []int64(got.ProjectIDs))
		require.NotNil(t, got.ExpiresAt)
		require.NoError(t, store.UpdateAccess(ctx, bot.ID, AccessTokenAccess{Scopes: []AccessTokenScope{AccessTokenScopeRead}}))
		got, err = store.Verify(ctx, secret)
		require.NoError(t, err)
		assert.Nil(t, got.ExpiresAt, "the expiration can be cleared")
		assert.NotNil(t, got.ProjectIDs)

		rotated, newSecret, err := store.Rotate(ctx, bot.ID)
		require.NoError(t, err)
		assert.NotEqual(t, secret, newSecret)
		assert.Equal(t, newSecret[:12], rotated.Prefix)
		assert.Nil(t, rotated.LastUsedAt)
		assert.Empty(t, rotated.LastUsedIP)
		assert.True(t, rotated.HasScope(AccessTokenScopeRead), "rotating keeps the access")
		_, err = store.Verify(ctx, secret)
		require.ErrorIs(t, err, ErrAccessTokenNotFound, "the old secret stops working")
		_, err = store.Verify(ctx, newSecret)
		require.NoError(t, err)

		require.ErrorIs(t, store.UpdateAccess(ctx, 9999, AccessTokenAccess{}), ErrAccessTokenNotFound)
		_, _, err = store.Rotate(ctx, 9999)
		require.ErrorIs(t, err, ErrAccessTokenNotFound)
	})
}
