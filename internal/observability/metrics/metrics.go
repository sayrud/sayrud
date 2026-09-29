package metrics

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/wuhan005/sayrud/internal/conf"
)

// New creates a meter provider and its protected Prometheus scrape handler.
// A disabled endpoint has no reader or handler, so it does not collect or export metrics.
func New(cfg conf.MetricsConfig) (*metric.MeterProvider, http.Handler, error) {
	if !cfg.Enabled {
		return metric.NewMeterProvider(), nil, nil
	}

	registry := prometheus.NewRegistry()
	handler, err := restrictAccess(promhttp.HandlerFor(registry, promhttp.HandlerOpts{}), cfg.Whitelist)
	if err != nil {
		return nil, nil, errors.Wrap(err, "metrics whitelist")
	}

	exporter, err := otelprometheus.New(otelprometheus.WithRegisterer(registry))
	if err != nil {
		return nil, nil, errors.Wrap(err, "create prometheus exporter")
	}
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
		metric.WithResource(resource.NewSchemaless(attribute.String("service.name", "sayrud"))),
	)
	return provider, handler, nil
}
