package tracing

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"

	"github.com/wuhan005/sayrud/internal/conf"
)

func TestNewDisabled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider, err := New(ctx, conf.TracingConfig{Endpoint: "invalid endpoint://["})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	// Disabling tracing must also override sampling inherited from an upstream span.
	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{1},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
	for _, ctx := range []context.Context{
		context.Background(),
		trace.ContextWithRemoteSpanContext(context.Background(), parent),
	} {
		_, span := provider.Tracer("test").Start(ctx, "disabled")
		require.False(t, span.IsRecording())
		require.False(t, span.SpanContext().IsSampled())
		span.End()
	}
}

func TestNewEnabledRequiresEndpoint(t *testing.T) {
	for _, endpoint := range []string{"", " \t\n"} {
		t.Run(endpoint, func(t *testing.T) {
			provider, err := New(context.Background(), conf.TracingConfig{
				Enabled:  true,
				Endpoint: endpoint,
			})
			require.ErrorContains(t, err, "tracing endpoint is required")
			require.Nil(t, provider)
		})
	}
}

type testTraceReceiver struct {
	collectortrace.UnimplementedTraceServiceServer
	requests chan *collectortrace.ExportTraceServiceRequest
}

func (r *testTraceReceiver) Export(ctx context.Context, request *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	select {
	case r.requests <- request:
		return &collectortrace.ExportTraceServiceResponse{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestNewExportsToConfiguredEndpoint(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	receiver := &testTraceReceiver{requests: make(chan *collectortrace.ExportTraceServiceRequest, 1)}
	server := grpc.NewServer()
	collectortrace.RegisterTraceServiceServer(server, receiver)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		<-serverDone
	})

	t.Setenv("TRACING_ENDPOINT", "127.0.0.1:1")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("HOSTNAME", "trace-test-host")
	provider, err := New(context.Background(), conf.TracingConfig{
		Enabled:  true,
		Endpoint: "  " + listener.Addr().String() + "  ",
		Token:    "configured-token",
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, provider.Shutdown(ctx))
	})

	_, span := provider.Tracer("test").Start(context.Background(), "configured-export")
	require.True(t, span.IsRecording())
	require.True(t, span.SpanContext().IsSampled())
	span.End()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, provider.ForceFlush(ctx))

	select {
	case request := <-receiver.requests:
		require.Len(t, request.ResourceSpans, 1)
		resourceSpans := request.ResourceSpans[0]
		attributes := make(map[string]string)
		for _, attribute := range resourceSpans.GetResource().GetAttributes() {
			attributes[attribute.GetKey()] = attribute.GetValue().GetStringValue()
		}
		require.Equal(t, "sayrud", attributes["service.name"])
		require.Equal(t, "trace-test-host", attributes["host.name"])
		require.Equal(t, "configured-token", attributes["token"])
		require.Len(t, resourceSpans.ScopeSpans, 1)
		require.Len(t, resourceSpans.ScopeSpans[0].Spans, 1)
		require.Equal(t, "configured-export", resourceSpans.ScopeSpans[0].Spans[0].GetName())
	case <-ctx.Done():
		t.Fatal("configured endpoint did not receive the span")
	}
}
