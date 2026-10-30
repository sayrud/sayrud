package setup

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
)

type wizardUsersStore struct {
	db.UsersStore
	exists  atomic.Bool
	created atomic.Int32
}

func (s *wizardUsersStore) CreateFirstAdmin(_ context.Context, options db.CreateUserOptions) (*db.User, error) {
	if s.exists.Swap(true) {
		return nil, db.ErrUsersExist
	}
	s.created.Add(1)
	return &db.User{Email: options.Email, UserName: options.UserName, IsAdmin: true}, nil
}

func (s *wizardUsersStore) Exists(context.Context) (bool, error) {
	return s.exists.Load(), nil
}

func newTestWizard(t *testing.T, stage Stage) (*Wizard, *wizardUsersStore, *atomic.Int32) {
	t.Helper()
	previous := db.Users
	t.Cleanup(func() { db.Users = previous })
	users := &wizardUsersStore{}
	db.Users = users

	token, err := NewToken()
	require.NoError(t, err)
	var finished atomic.Int32
	wizard := New(Options{Token: token, Stage: stage, Hooks: Hooks{
		ConfigureDatabase: func(context.Context, DatabaseOptions) error { return nil },
		Finish:            func() { finished.Add(1) },
	}})
	return wizard, users, &finished
}

func TestWizardAuthorize(t *testing.T) {
	wizard, _, _ := newTestWizard(t, StageDatabase)

	require.NoError(t, wizard.Authorize("1.1.1.1", wizard.token.String()))
	for range maxTokenFailures {
		require.ErrorIs(t, wizard.Authorize("1.1.1.1", "wrong"), ErrInvalidToken)
	}
	require.ErrorIs(t, wizard.Authorize("1.1.1.1", wizard.token.String()), ErrTooManyAttempts, "the right token is rejected after too many failures")
	require.NoError(t, wizard.Authorize("2.2.2.2", wizard.token.String()))
}

func TestWizardCreateAdmin(t *testing.T) {
	t.Run("database stage", func(t *testing.T) {
		wizard, users, finished := newTestWizard(t, StageDatabase)
		_, err := wizard.CreateAdmin(context.Background(), db.CreateUserOptions{Email: "admin@example.com"})
		require.ErrorIs(t, err, ErrStage)
		assert.Zero(t, users.created.Load())
		assert.Zero(t, finished.Load())
	})

	t.Run("creates once", func(t *testing.T) {
		wizard, users, finished := newTestWizard(t, StageAdmin)
		user, err := wizard.CreateAdmin(context.Background(), db.CreateUserOptions{Email: "admin@example.com", UserName: "Admin"})
		require.NoError(t, err)
		assert.True(t, user.IsAdmin)
		assert.Equal(t, StageDone, wizard.Stage())
		assert.EqualValues(t, 1, finished.Load())

		_, err = wizard.CreateAdmin(context.Background(), db.CreateUserOptions{Email: "other@example.com"})
		require.ErrorIs(t, err, ErrInstalled)
		assert.EqualValues(t, 1, users.created.Load())
		assert.EqualValues(t, 1, finished.Load(), "the application takes over only once")
	})

	t.Run("created by another instance", func(t *testing.T) {
		wizard, users, finished := newTestWizard(t, StageAdmin)
		users.exists.Store(true)
		_, err := wizard.CreateAdmin(context.Background(), db.CreateUserOptions{Email: "admin@example.com"})
		require.ErrorIs(t, err, ErrInstalled)
		assert.Equal(t, StageDone, wizard.Stage())
		assert.EqualValues(t, 1, finished.Load(), "the wizard hands over to the application")
	})
}

func TestWizardConfigureDatabaseStage(t *testing.T) {
	wizard, _, _ := newTestWizard(t, StageAdmin)
	require.ErrorIs(t, wizard.ConfigureDatabase(context.Background(), DatabaseOptions{}), ErrStage)
}

func TestWizardWatch(t *testing.T) {
	wizard, users, finished := newTestWizard(t, StageAdmin)
	done := make(chan struct{})
	go func() {
		defer close(done)
		wizard.Watch(context.Background(), 10*time.Millisecond)
	}()

	users.exists.Store(true)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Watch did not finish after another instance created the admin")
	}
	assert.Equal(t, StageDone, wizard.Stage())
	assert.EqualValues(t, 1, finished.Load())
}
