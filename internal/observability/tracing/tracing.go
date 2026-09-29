// Copyright 2022 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tracing

import (
	"context"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/wuhan005/sayrud/internal/conf"
)

// New creates a tracer provider without changing the global OpenTelemetry setup.
func New(ctx context.Context, cfg conf.TracingConfig) (*sdktrace.TracerProvider, error) {
	if !cfg.Enabled {
		return sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample())), nil
	}

	endpoint := strings.TrimSpace(cfg.Endpoint)
	if endpoint == "" {
		return nil, errors.New("tracing endpoint is required when tracing is enabled")
	}

	r := resource.NewSchemaless(
		attribute.String("token", cfg.Token),
		attribute.String("service.name", "sayrud"),
		attribute.String("host.name", os.Getenv("HOSTNAME")),
	)

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "create tracing exporter")
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(r),
	), nil
}
