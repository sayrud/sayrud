package metrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric"

	"github.com/wuhan005/sayrud/internal/conf"
)

func TestNewDisabled(t *testing.T) {
	provider, handler, err := New(conf.MetricsConfig{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	require.Nil(t, handler)

	// No reader is installed, so even observable instruments do no collection work.
	collected := false
	_, err = provider.Meter("test").Int64ObservableGauge("test_gauge", metric.WithInt64Callback(
		func(context.Context, metric.Int64Observer) error {
			collected = true
			return nil
		},
	))
	require.NoError(t, err)
	require.NoError(t, provider.ForceFlush(context.Background()))
	require.False(t, collected)
}

func TestNewInvalidWhitelist(t *testing.T) {
	provider, handler, err := New(conf.MetricsConfig{Enabled: true, Whitelist: []string{"invalid"}})
	require.ErrorContains(t, err, "metrics whitelist")
	require.Nil(t, provider)
	require.Nil(t, handler)
}

func TestScrapeCollectsOnlyForAllowedPeers(t *testing.T) {
	provider, handler, err := New(conf.MetricsConfig{Enabled: true, Whitelist: []string{"127.0.0.1"}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	collections := 0
	_, err = provider.Meter("test").Int64ObservableGauge("test_gauge", metric.WithInt64Callback(
		func(_ context.Context, observer metric.Int64Observer) error {
			collections++
			observer.Observe(42)
			return nil
		},
	))
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodGet, "/-/metrics", nil)
	request.RemoteAddr = "192.0.2.1:1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusForbidden, response.Code)
	require.Zero(t, collections)

	request.RemoteAddr = "127.0.0.1:1234"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, 1, collections)
	require.Regexp(t, `(?m)^test_gauge\{[^}\n]*\} 42$`, response.Body.String())
	require.Contains(t, response.Body.String(), "go_goroutines ")
	require.Contains(t, response.Body.String(), `service_name="sayrud"`)
}

func TestRegistriesAreIsolated(t *testing.T) {
	for _, value := range []int64{3, 7} {
		provider, handler, err := New(conf.MetricsConfig{Enabled: true, Whitelist: []string{"127.0.0.1"}})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
		counter, err := provider.Meter("test").Int64Counter("test_requests")
		require.NoError(t, err)
		counter.Add(context.Background(), value)

		request := httptest.NewRequest(http.MethodGet, "/-/metrics", nil)
		request.RemoteAddr = "127.0.0.1:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		require.Regexp(t, fmt.Sprintf(`(?m)^test_requests_total\{[^}\n]*\} %d$`, value), response.Body.String())
	}
}
