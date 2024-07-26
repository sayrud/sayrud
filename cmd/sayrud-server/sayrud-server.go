package main

import (
	"flag"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/uptrace/opentelemetry-go-extra/otellogrus"
	"github.com/uptrace/uptrace-go/uptrace"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/redis"
	"github.com/wuhan005/sayrud/internal/route"
)

func main() {
	port := flag.Int("port", 8080, "port to listen")
	flag.Parse()

	uptraceDsn := os.Getenv("UPTRACE_DSN")
	if uptraceDsn != "" {
		uptrace.ConfigureOpentelemetry(
			uptrace.WithDSN(uptraceDsn),
			uptrace.WithServiceName("sayrud"),
		)
		logrus.Debug("Tracing enabled.")
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
