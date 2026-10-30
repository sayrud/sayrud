package cmd

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/observability"
	"github.com/wuhan005/sayrud/internal/redis"
	"github.com/wuhan005/sayrud/internal/route"
	"github.com/wuhan005/sayrud/internal/sso"
	"github.com/wuhan005/sayrud/internal/storage"
)

// lifecycle is the part of the runtime that serve drains and shuts down.
type lifecycle interface {
	BeginDrain()
	Shutdown(grace, cleanup context.Context) error
}

// switchHandler serves the requests with the latest stored handler, so the wizard can hand over to the application atomically.
type switchHandler struct {
	handler atomic.Pointer[http.Handler]
}

func (s *switchHandler) Store(h http.Handler) {
	s.handler.Store(&h)
}

func (s *switchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	(*s.handler.Load()).ServeHTTP(w, r)
}

var errShuttingDown = errors.New("server is shutting down")

// application owns the services started from the loaded configuration, it stays empty until the configuration exists.
type application struct {
	mu       sync.Mutex
	runtime  *route.Runtime
	router   http.Handler
	draining bool
	// closers release the connections in reverse order.
	closers []func()
}

// start connects the services with the loaded configuration and builds the router.
// commit runs after the connections succeed and before the background workers start, a failed start releases what it opened.
func (a *application) start(ctx context.Context, commit func() error) (err error) {
	defer func() {
		if err != nil {
			a.close()
		}
	}()

	if err := sso.CheckSecretKey(); err != nil {
		return errors.Wrap(err, "invalid auth.secret_key")
	}
	if !sso.SecretsReady() {
		logrus.Warn("auth.secret_key is not configured, third-party sign-in methods can not be saved")
	}

	metricsHandler, shutdownObservability, err := observability.Init(ctx)
	if err != nil {
		return errors.Wrap(err, "initialize observability")
	}
	a.addCloser(shutdownObservability)

	gormDB, err := db.Init()
	if err != nil {
		return errors.Wrap(err, "initialize database")
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		return errors.Wrap(err, "get database connection")
	}
	a.addCloser(func() { _ = sqlDB.Close() })

	rdb, err := redis.Init()
	if err != nil {
		return errors.Wrap(err, "initialize redis")
	}
	a.addCloser(func() { _ = rdb.Close() })

	if err := storage.Init(ctx); err != nil {
		return errors.Wrap(err, "initialize storage")
	}

	if commit != nil {
		if err := commit(); err != nil {
			return err
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.draining {
		return errShuttingDown
	}
	runtime, err := route.NewRuntime(ctx, gormDB, rdb)
	if err != nil {
		return errors.Wrap(err, "start collaboration")
	}
	a.runtime = runtime
	a.router = route.New(route.Options{DB: gormDB, MetricsHandler: metricsHandler, Runtime: runtime})
	return nil
}

func (a *application) addCloser(closer func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closers = append(a.closers, closer)
}

// close releases the connections, it must be called after Shutdown if the runtime has started.
func (a *application) close() {
	a.mu.Lock()
	closers := a.closers
	a.closers = nil
	a.mu.Unlock()

	for i := len(closers) - 1; i >= 0; i-- {
		closers[i]()
	}
}

// Router returns the router of the application, nil before start succeeds.
func (a *application) Router() http.Handler {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.router
}

// BeginDrain stops the runtime from taking new work, and keeps it from starting later.
func (a *application) BeginDrain() {
	a.mu.Lock()
	a.draining = true
	runtime := a.runtime
	a.mu.Unlock()

	if runtime != nil {
		runtime.BeginDrain()
	}
}

func (a *application) Shutdown(grace, cleanup context.Context) error {
	a.mu.Lock()
	a.draining = true
	runtime := a.runtime
	a.mu.Unlock()

	if runtime == nil {
		return nil
	}
	return runtime.Shutdown(grace, cleanup)
}
