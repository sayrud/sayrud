package route

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/conf"
	"github.com/wuhan005/sayrud/internal/observability/metrics"
)

func TestMetricsEndpoint(t *testing.T) {
	provider, handler, err := metrics.New(conf.MetricsConfig{
		Enabled:   true,
		Whitelist: []string{"192.0.2.0/24"},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	counter, err := provider.Meter("route-test").Int64Counter("route_requests")
	require.NoError(t, err)
	counter.Add(context.Background(), 7)

	router := New(Options{MetricsHandler: handler})
	for _, path := range []string{"/-/metrics", "/-/metrics?source=prometheus"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.RemoteAddr = "192.0.2.10:12345"
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusOK, response.Code)
			require.Contains(t, response.Header().Get("Content-Type"), "text/plain")
			var parser expfmt.TextParser
			families, err := parser.TextToMetricFamilies(strings.NewReader(response.Body.String()))
			require.NoError(t, err)
			family := families["route_requests_total"]
			require.NotNil(t, family, "recorded OTel counter must be exposed by the application route")
			require.Len(t, family.GetMetric(), 1)
			require.Equal(t, float64(7), family.Metric[0].GetCounter().GetValue())
		})
	}
}

func TestMetricsEndpointRejectsUnlistedPeer(t *testing.T) {
	t.Setenv("IP_HEADER", "X-Real-IP")
	provider, handler, err := metrics.New(conf.MetricsConfig{
		Enabled:   true,
		Whitelist: []string{"192.0.2.0/24"},
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	router := New(Options{MetricsHandler: handler})

	for _, spoofHeaders := range []bool{false, true} {
		name := "unlisted peer"
		if spoofHeaders {
			name = "spoofed forwarding headers"
		}
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/-/metrics", nil)
			request.RemoteAddr = "198.51.100.10:12345"
			if spoofHeaders {
				request.Header.Set("X-Forwarded-For", "192.0.2.10")
				request.Header.Set("X-Real-IP", "192.0.2.10")
				request.Header.Set("Forwarded", "for=192.0.2.10")
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusForbidden, response.Code)
			require.Equal(t, "Forbidden\n", response.Body.String())
		})
	}
}

func TestMetricsEndpointAbsentWhenDisabled(t *testing.T) {
	provider, disabledHandler, err := metrics.New(conf.MetricsConfig{})
	require.NoError(t, err)
	require.Nil(t, disabledHandler)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	router := New(Options{MetricsHandler: disabledHandler})
	request := httptest.NewRequest(http.MethodGet, "/-/metrics", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNotFound, response.Code)
	require.NotEmpty(t, response.Body.String())
}
