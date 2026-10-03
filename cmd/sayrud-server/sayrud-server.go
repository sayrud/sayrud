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
)

func main() {
	configFilePath := flag.String("config", "./config/sayrud.yaml", "path to the configuration file")
	flag.Parse()

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

	if _, err := redis.Init(); err != nil {
		logrus.WithError(err).Fatal("Failed to initialize redis")
	}

	address := fmt.Sprintf("0.0.0.0:%d", conf.App.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		logrus.WithError(err).Fatalf("Failed to listen on %s", address)
	}
	logrus.Infof("Listening on %s", address)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	ctx, cancel = context.WithCancel(ctx)
	server := http.Server{
		Handler: route.New(route.Options{
			DB:             db,
			MetricsHandler: metricsHandler,
			Context:        ctx,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-c
		_ = server.Shutdown(ctx)
		cancel()
	}()

	if err := server.Serve(listener); err != nil {
		if !errors.Is(err, http.ErrServerClosed) {
			logrus.WithError(err).Errorf("Failed to start server")
			cancel()
		}
	}

	<-ctx.Done()
	logrus.Info("Shutting down")
}
