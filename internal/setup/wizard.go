package setup

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/db"
)

// Stage is the next step of the installation.
type Stage string

const (
	// StageDatabase configures PostgreSQL and Redis, it is the first stage without a configuration file.
	StageDatabase Stage = "database"
	// StageAdmin creates the first admin, it is the first stage when the database has no users.
	StageAdmin Stage = "admin"
	// StageDone means the application has taken over the requests.
	StageDone Stage = "done"
)

const (
	maxTokenFailures   = 10
	tokenFailureWindow = 10 * time.Minute
)

var (
	ErrInvalidToken    = errors.New("invalid setup token")
	ErrTooManyAttempts = errors.New("too many failed setup token attempts")
	ErrBusy            = errors.New("another setup step is in progress")
	ErrStage           = errors.New("the step does not match the current stage")
	ErrInstalled       = errors.New("already installed")
	// ErrRestartRequired is returned by Hooks.ConfigureDatabase when the configuration has been saved but the application failed to start.
	ErrRestartRequired = errors.New("configuration saved, restart required")
)

// Hooks connect the wizard to the server.
type Hooks struct {
	// ConfigureDatabase saves the configuration of the tested services and starts the application behind the wizard.
	ConfigureDatabase func(ctx context.Context, opts DatabaseOptions) error
	// Finish serves the application in place of the wizard, it is called at most once.
	Finish func()
}

// Options are the options of New.
type Options struct {
	Token *Token
	Stage Stage
	Hooks Hooks
}

// Wizard tracks the installation progress shared by the setup routes.
type Wizard struct {
	token   *Token
	hooks   Hooks
	limiter *limiter

	stage atomic.Value
	// busy serializes the steps changing the stage, the concurrent ones fail with ErrBusy instead of waiting.
	busy   sync.Mutex
	finish sync.Once
}

// New creates the wizard starting at the stage.
func New(opts Options) *Wizard {
	w := &Wizard{
		token:   opts.Token,
		hooks:   opts.Hooks,
		limiter: newLimiter(maxTokenFailures, tokenFailureWindow),
	}
	w.stage.Store(opts.Stage)
	return w
}

// Stage returns the current stage.
func (w *Wizard) Stage() Stage {
	return w.stage.Load().(Stage)
}

// Authorize checks the setup token sent by the client, it returns ErrTooManyAttempts after too many failures of the client.
func (w *Wizard) Authorize(client, token string) error {
	if !w.limiter.Allow(client) {
		return ErrTooManyAttempts
	}
	if !w.token.Verify(token) {
		w.limiter.Fail(client)
		return ErrInvalidToken
	}
	return nil
}

// ConfigureDatabase tests the services, saves the configuration and moves to StageAdmin.
// It returns a *CheckError if a service is unusable.
func (w *Wizard) ConfigureDatabase(ctx context.Context, opts DatabaseOptions) error {
	if !w.busy.TryLock() {
		return ErrBusy
	}
	defer w.busy.Unlock()
	if w.Stage() != StageDatabase {
		return ErrStage
	}

	pg, rdb := CheckServices(ctx, opts)
	if pg.Err != nil {
		return &CheckError{Service: "postgres", Err: pg.Err}
	}
	if rdb.Err != nil {
		return &CheckError{Service: "redis", Err: rdb.Err}
	}

	if err := w.hooks.ConfigureDatabase(ctx, opts); err != nil {
		return errors.Wrap(err, "configure database")
	}
	w.stage.Store(StageAdmin)
	return nil
}

// CreateAdmin creates the first admin and hands the requests over to the application.
// It returns ErrInstalled if another replica has created a user in the meantime.
func (w *Wizard) CreateAdmin(ctx context.Context, opts db.CreateUserOptions) (*db.User, error) {
	if !w.busy.TryLock() {
		return nil, ErrBusy
	}
	defer w.busy.Unlock()
	switch w.Stage() {
	case StageAdmin:
	case StageDone:
		return nil, ErrInstalled
	default:
		return nil, ErrStage
	}

	user, err := db.Users.CreateFirstAdmin(ctx, opts)
	if err != nil {
		if errors.Is(err, db.ErrUsersExist) {
			w.Finish()
			return nil, ErrInstalled
		}
		return nil, errors.Wrap(err, "create first admin")
	}
	w.Finish()
	return user, nil
}

// Finish moves to StageDone and lets the application serve the requests.
func (w *Wizard) Finish() {
	w.finish.Do(func() {
		w.stage.Store(StageDone)
		w.hooks.Finish()
	})
}

// Watch finishes the wizard once another replica sharing the database has created the admin, it returns after finishing or when ctx is done.
func (w *Wizard) Watch(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		switch w.Stage() {
		case StageDone:
			return
		case StageAdmin:
			exists, err := db.Users.Exists(ctx)
			if err != nil {
				logrus.WithError(err).Warn("Failed to check whether the admin has been created")
				continue
			}
			if exists {
				logrus.Info("The admin has been created by another instance")
				w.Finish()
				return
			}
		}
	}
}
