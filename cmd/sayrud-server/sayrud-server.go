package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/conf"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/observability"
	"github.com/wuhan005/sayrud/internal/redis"
	"github.com/wuhan005/sayrud/internal/route"
	"github.com/wuhan005/sayrud/internal/sso"
	"github.com/wuhan005/sayrud/internal/storage"
)

func main() {
	configFilePath := flag.String("config", "./config/sayrud.yaml", "path to the configuration file")
	flag.Parse()
	signals, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	if err := conf.Init(*configFilePath); err != nil {
		logrus.WithError(err).Fatal("Failed to initialize configuration")
	}
	if err := sso.CheckSecretKey(); err != nil {
		logrus.WithError(err).Fatal("Invalid auth.secret_key")
	}
	if !sso.SecretsReady() {
		logrus.Warn("auth.secret_key is not configured, third-party sign-in methods can not be saved")
	}

	ctx := context.Background()
	metricsHandler, cancel, err := observability.Init(ctx)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to initialize observability")
	}
	defer cancel()

	db, err := db.Init()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to initialize database")
	}

	rdb, err := redis.Init()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to initialize redis")
	}

	if err := storage.Init(ctx); err != nil {
		logrus.WithError(err).Fatal("Failed to initialize storage")
	}

	if signals.Err() != nil {
		return
	}

	runtime, err := route.NewRuntime(ctx, db, rdb)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to start collaboration")
	}
	defer func() { _ = rdb.Close() }()

	sqlDB, err := db.DB()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to get database connection")
	}
	defer func() { _ = sqlDB.Close() }()

	address := fmt.Sprintf("0.0.0.0:%d", conf.App.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		logrus.WithError(err).Fatalf("Failed to listen on %s", address)
	}
	logrus.Infof("Listening on %s", address)

	requests, cancelRequests := context.WithCancel(ctx)
	defer cancelRequests()
	server := &http.Server{
		Handler: route.New(route.Options{
			DB: db, MetricsHandler: metricsHandler, Runtime: runtime,
		}),
		BaseContext:       func(net.Listener) context.Context { return requests },
		ReadHeaderTimeout: 10 * time.Second,
	}

	if err := serve(signals, listener, server, runtime, cancelRequests, conf.App.DrainDelay, conf.App.ShutdownTimeout); err != nil {
		logrus.WithError(err).Error("Server stopped with incomplete graceful shutdown")
	}
	logrus.Info("Shutting down")
}

// serve reserves part of the total budget for canceled workers to release their
// jobs. Ordinary requests keep their context until grace expires.
func serve(signals context.Context, listener net.Listener, server *http.Server, runtime *route.Runtime, cancelRequests context.CancelFunc, drainDelay, timeout time.Duration) error {
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()

	var serveErr error
	select {
	case <-signals.Done():
	case serveErr = <-served:
	}

	runtime.BeginDrain()

	total, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	reserve := min(5*time.Second, timeout/2)
	grace, cancelGrace := context.WithTimeout(total, timeout-reserve)
	defer cancelGrace()

	timer := time.NewTimer(drainDelay)
	select {
	case <-timer.C:
	case <-grace.Done():
	}
	timer.Stop()

	httpErr := server.Shutdown(grace)
	if httpErr != nil {
		cancelRequests()
		_ = server.Close()
	}

	runtimeErr := runtime.Shutdown(grace, total)
	cancelRequests()

	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return serveErr
	}
	if httpErr != nil {
		return httpErr
	}
	return runtimeErr
}
