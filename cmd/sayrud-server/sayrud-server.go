package main

import (
	"context"
	"errors"
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/opentelemetry-go-extra/otellogrus"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/redis"
	"github.com/wuhan005/sayrud/internal/route"
	"github.com/wuhan005/sayrud/internal/tracing"
)

func main() {
	port := flag.Int("port", 8080, "port to listen")
	flag.Parse()

	if os.Getenv("TRACING_ENDPOINT") != "" {
		ctx := context.Background()
		otelShutdown, err := tracing.SetupOTelSDK(ctx)
		if err != nil {
			logrus.WithContext(ctx).WithError(err).Fatal("Failed to initialize OTel SDK")
		}
		defer func() {
			if err = errors.Join(err, otelShutdown(context.Background())); err != nil {
				logrus.WithError(err).Error("Failed to shutdown OTel SDK")
			}
		}()
	}

	logrus.AddHook(otellogrus.NewHook(otellogrus.WithLevels(
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
	)))

	db, err := db.Init()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to initialize database")
	}

	redisClient, err := redis.Init()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to initialize redis")
	}

	f := route.New(db, redisClient)
	f.Run(*port)
}
