package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"github.com/wuhan005/sayrud/internal/conf"
)

func TestInitDisabled(t *testing.T) {
	isolateObservability(t)
	handler, shutdown, err := Init(context.Background())
	require.NoError(t, err)
	t.Cleanup(shutdown)
	require.Nil(t, handler)

	_, span := otel.Tracer("test").Start(context.Background(), "disabled")
	defer span.End()
	require.False(t, span.IsRecording())
}

func TestInitMetricsWithoutTracing(t *testing.T) {
	isolateObservability(t)
	conf.Observability.Tracing = conf.TracingConfig{Endpoint: "unused while disabled"}
	conf.Observability.Metrics = conf.MetricsConfig{Enabled: true, Whitelist: []string{"127.0.0.1"}}
	handler, shutdown, err := Init(context.Background())
	require.NoError(t, err)
	t.Cleanup(shutdown)
	require.NotNil(t, handler)

	// Existing instrumentation uses the global provider installed by Init.
	counter, err := otel.Meter("test").Int64Counter("observability_requests")
	require.NoError(t, err)
	counter.Add(context.Background(), 5)
	request := httptest.NewRequest(http.MethodGet, "/-/metrics", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	var parser expfmt.TextParser
	families, err := parser.TextToMetricFamilies(strings.NewReader(response.Body.String()))
	require.NoError(t, err)
	require.NotNil(t, families["observability_requests_total"])
	require.Len(t, families["observability_requests_total"].Metric, 1)
	require.Equal(t, float64(5), families["observability_requests_total"].Metric[0].GetCounter().GetValue())

	_, span := otel.Tracer("test").Start(context.Background(), "metrics only")
	defer span.End()
	require.False(t, span.IsRecording())

	request.RemoteAddr = "192.0.2.1:1234"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusForbidden, response.Code)
}

func TestInitFailurePreservesProviders(t *testing.T) {
	for _, tc := range []struct {
		name    string
		tracing conf.TracingConfig
		metrics conf.MetricsConfig
		want    string
	}{
		{
			name:    "invalid metrics whitelist",
			metrics: conf.MetricsConfig{Enabled: true, Whitelist: []string{"invalid"}},
			want:    "initialize metrics",
		},
		{
			name:    "missing tracing endpoint after metrics initialization",
			tracing: conf.TracingConfig{Enabled: true},
			metrics: conf.MetricsConfig{Enabled: true, Whitelist: []string{"127.0.0.1"}},
			want:    "initialize tracing",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateObservability(t)
			conf.Observability.Tracing = tc.tracing
			conf.Observability.Metrics = tc.metrics
			previousMeter, previousTracer := otel.GetMeterProvider(), otel.GetTracerProvider()
			handler, shutdown, err := Init(context.Background())
			require.ErrorContains(t, err, tc.want)
			require.Nil(t, handler)
			require.Nil(t, shutdown)
			require.Same(t, previousMeter, otel.GetMeterProvider())
			require.Same(t, previousTracer, otel.GetTracerProvider())
		})
	}
}

func isolateObservability(t *testing.T) {
	t.Helper()
	previousConfig := conf.Observability
	conf.Observability.Tracing = conf.TracingConfig{}
	conf.Observability.Metrics = conf.MetricsConfig{}
	previousMeter, previousTracer := otel.GetMeterProvider(), otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		conf.Observability = previousConfig
		otel.SetMeterProvider(previousMeter)
		otel.SetTracerProvider(previousTracer)
		otel.SetTextMapPropagator(previousPropagator)
	})
}
