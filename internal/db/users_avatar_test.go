package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetAvatarSerializesReplacements(t *testing.T) {
	store := NewUsersStore(newTestDB(t, &User{}))
	ctx := context.Background()
	user, err := store.Create(ctx, CreateUserOptions{Email: "avatar@example.com", UserName: "Avatar"})
	require.NoError(t, err)

	start := make(chan struct{})
	type result struct {
		previous string
		err      error
	}
	results := make(chan result, 4)
	for i := 0; i < cap(results); i++ {
		go func(i int) {
			<-start
			_, previous, err := store.SetAvatar(ctx, user.ID, fmt.Sprintf("avatar-%d", i))
			results <- result{previous, err}
		}(i)
	}
	close(start)

	previous := map[string]bool{}
	for i := 0; i < cap(results); i++ {
		res := <-results
		require.NoError(t, res.err)
		require.False(t, previous[res.previous], "each replaced avatar is returned once for cleanup")
		previous[res.previous] = true
	}

	current, err := store.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, previous[""])
	require.False(t, previous[current.AvatarFileUID], "the current avatar must never be scheduled for cleanup")
	require.Equal(t, "Avatar", current.UserName)

	updated, old, err := store.SetAvatar(ctx, user.ID, "")
	require.NoError(t, err)
	require.Equal(t, current.AvatarFileUID, old)
	require.Empty(t, updated.AvatarFileUID)
	require.True(t, updated.UpdatedAt.After(user.UpdatedAt))

	_, _, err = store.SetAvatar(ctx, user.ID+1, "unused")
	require.ErrorIs(t, err, ErrUserNotFound)
}
