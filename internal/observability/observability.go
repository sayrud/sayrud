package observability

import (
	"context"
	"net/http"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
	"github.com/uptrace/opentelemetry-go-extra/otellogrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/wuhan005/sayrud/internal/conf"
	"github.com/wuhan005/sayrud/internal/observability/metrics"
	"github.com/wuhan005/sayrud/internal/observability/tracing"
)

// Init configures tracing and metrics together and returns their shared cleanup.
// The metrics handler is nil when its endpoint is disabled.
func Init(ctx context.Context) (http.Handler, func(), error) {
	cfg := conf.Observability
	meterProvider, metricsHandler, err := metrics.New(cfg.Metrics)
	if err != nil {
		return nil, nil, errors.Wrap(err, "initialize metrics")
	}
	tracerProvider, err := tracing.New(ctx, cfg.Tracing)
	if err != nil {
		return nil, nil, errors.CombineErrors(errors.Wrap(err, "initialize tracing"), meterProvider.Shutdown(ctx))
	}

	// Publish the providers only after both have initialized successfully.
	otel.SetMeterProvider(meterProvider)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if cfg.Tracing.Enabled {
		logrus.AddHook(otellogrus.NewHook())
	}

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := errors.CombineErrors(meterProvider.Shutdown(ctx), tracerProvider.Shutdown(ctx)); err != nil {
			logrus.WithError(err).Error("Failed to shut down observability")
		}
	}
	return metricsHandler, shutdown, nil
}
