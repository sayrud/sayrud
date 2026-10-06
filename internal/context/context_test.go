package context

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/conf"
	"github.com/wuhan005/sayrud/internal/i18n"
)

func TestContextIP(t *testing.T) {
	previousApp := conf.App
	t.Cleanup(func() { conf.App = previousApp })
	t.Setenv("IP_HEADER", "X-Environment-IP")

	for _, tc := range []struct {
		name       string
		ipHeader   string
		remoteAddr string
		header     string
		want       string
	}{
		{name: "IPv4 remote address", remoteAddr: "192.0.2.10:54321", want: "192.0.2.10"},
		{name: "IPv6 remote address", remoteAddr: "[2001:db8::1]:54321", want: "2001:db8::1"},
		{name: "IP header", ipHeader: "X-Real-IP", remoteAddr: "127.0.0.1:54321", header: "198.51.100.7", want: "198.51.100.7"},
		{name: "IP header with port", ipHeader: "X-Real-IP", remoteAddr: "127.0.0.1:54321", header: "198.51.100.7:8080", want: "198.51.100.7"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf.App.IPHeader = tc.ipHeader

			var got string
			f := flamego.New()
			f.Use(i18n.Middleware(), Contexter(nil))
			f.Get("/", func(ctx Context) { got = ctx.IP() })

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = tc.remoteAddr
			request.Header.Set("X-Environment-IP", "203.0.113.9")
			if tc.ipHeader != "" {
				request.Header.Set(tc.ipHeader, tc.header)
			}
			f.ServeHTTP(httptest.NewRecorder(), request)
			require.Equal(t, tc.want, got)
		})
	}
}
