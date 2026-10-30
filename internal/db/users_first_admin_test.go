package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateFirstAdmin(t *testing.T) {
	store := NewUsersStore(newTestDB(t, &User{}))
	ctx := context.Background()

	exists, err := store.Exists(ctx)
	require.NoError(t, err)
	require.False(t, exists)

	start := make(chan struct{})
	results := make(chan error, 4)
	for i := 0; i < cap(results); i++ {
		go func(i int) {
			<-start
			_, err := store.CreateFirstAdmin(ctx, CreateUserOptions{
				Email: fmt.Sprintf(" Admin%d@Example.com ", i), UserName: "Admin", Password: "password123",
			})
			results <- err
		}(i)
	}
	close(start)

	created := 0
	for i := 0; i < cap(results); i++ {
		if err := <-results; err == nil {
			created++
		} else {
			require.ErrorIs(t, err, ErrUsersExist)
		}
	}
	require.Equal(t, 1, created, "only one of the concurrent installers creates the admin")

	exists, err = store.Exists(ctx)
	require.NoError(t, err)
	require.True(t, exists)

	stats, err := store.Stats(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.Total)
	assert.EqualValues(t, 1, stats.Admins)

	users, _, err := store.List(ctx, ListUsersOptions{})
	require.NoError(t, err)
	require.Len(t, users, 1)
	assert.Regexp(t, `^admin\d@example\.com$`, users[0].Email)
	_, err = store.Authenticate(ctx, users[0].Email, "password123")
	require.NoError(t, err)
}
