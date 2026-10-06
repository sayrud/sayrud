package route

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/conf"
	"github.com/wuhan005/sayrud/internal/shortcut"
)

// Runtime owns the components which must outlive HTTP requests during draining.
type Runtime struct {
	Hub      *collab.Hub
	Engine   *shortcut.Engine
	db       *gorm.DB
	redis    *redis.Client
	draining atomic.Bool
}

func NewRuntime(ctx context.Context, gormDB *gorm.DB, client *redis.Client) (*Runtime, error) {
	hub := collab.NewHub(gormDB)
	if err := hub.StartCluster(ctx, client); err != nil {
		return nil, err
	}

	engine := shortcut.NewEngine(gormDB, hub, conf.Shortcut.Workers)
	engine.Start(ctx)

	return &Runtime{Hub: hub, Engine: engine, db: gormDB, redis: client}, nil
}

func (r *Runtime) Ready(ctx context.Context) error {
	if r.draining.Load() || !r.Hub.ClusterReady() {
		return fmt.Errorf("service is draining or collaboration is unavailable")
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}
	if err := r.redis.Ping(ctx).Err(); err != nil {
		return err
	}

	if r.draining.Load() || !r.Hub.ClusterReady() {
		return fmt.Errorf("service is draining or collaboration is unavailable")
	}

	return nil
}

func (r *Runtime) BeginDrain() {
	r.draining.Store(true)
	r.Engine.Stop()
	r.Hub.BeginDrain()
}

// Shutdown is called after ordinary HTTP requests have drained. The deadline
// reserves a final cleanup window for interrupted workers to return their leases.
func (r *Runtime) Shutdown(grace, cleanup context.Context) error {
	r.BeginDrain()
	_ = r.Hub.Shutdown(grace)
	workerErr := r.Engine.Shutdown(grace, cleanup)

	// Workers may have committed new follow-up tasks while draining.
	socketCleanupErr := r.Hub.Shutdown(cleanup)
	backgroundErr := r.Hub.WaitBackground(cleanup)
	r.Hub.CancelBackground()
	r.Hub.StopCluster()

	for _, err := range []error{workerErr, socketCleanupErr, backgroundErr} {
		if err != nil {
			return err
		}
	}

	return nil
}
