package script

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
)

func TestRunValues(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		code string
		want interface{}
	}{
		{"sync", `function execute(params) { return params.text.toUpperCase() }`, "HELLO"},
		{"async", `async function execute(params) { await null; return params.n * 2 }`, float64(42)},
		{"object", `async function execute() { return { a: [1, "b"], c: true } }`, map[string]interface{}{"a": []interface{}{float64(1), "b"}, "c": true}},
		{"undefined", `function execute() {}`, nil},
		{"context", `function execute(params, context) { return context.recordUID }`, "recAAAAAAAAAAA"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Run(ctx, Options{
				Code:    tc.code,
				Params:  map[string]interface{}{"text": "hello", "n": 21},
				Context: map[string]interface{}{"recordUID": "recAAAAAAAAAAA"},
				Timeout: time.Second,
			})
			require.NoError(t, err)
			require.Equal(t, tc.want, result.Value)
		})
	}
}

func TestRunErrors(t *testing.T) {
	ctx := context.Background()

	_, err := Run(ctx, Options{Code: `const x = 1`, Timeout: time.Second})
	require.ErrorIs(t, err, ErrNoExecute)

	_, err = Run(ctx, Options{Code: `function execute() { throw new Error("boom") }`, Timeout: time.Second})
	var scriptErr *Error
	require.ErrorAs(t, err, &scriptErr)
	require.Equal(t, "Error: boom", scriptErr.Message)

	_, err = Run(ctx, Options{Code: `async function execute() { throw new TypeError("bad") }`, Timeout: time.Second})
	require.ErrorAs(t, err, &scriptErr)
	require.Equal(t, "TypeError: bad", scriptErr.Message)

	_, err = Run(ctx, Options{Code: `function execute() { return new Promise(() => {}) }`, Timeout: time.Second})
	require.ErrorIs(t, err, ErrPendingPromise)

	start := time.Now()
	_, err = Run(ctx, Options{Code: `function execute() { for (;;) {} }`, Timeout: 100 * time.Millisecond})
	require.ErrorIs(t, err, ErrTimeout)
	require.Less(t, time.Since(start), 2*time.Second)

	_, err = Run(ctx, Options{Code: `function execute() { return execute() }`, Timeout: time.Second})
	require.ErrorAs(t, err, &scriptErr)

	_, err = Run(ctx, Options{Code: `function execute() { return require("fs") }`, Timeout: time.Second})
	require.ErrorAs(t, err, &scriptErr)

	require.Error(t, Compile(`function execute( {`))
	require.NoError(t, Compile(`async function execute() { return 1 }`))
}

func TestRunLogs(t *testing.T) {
	result, err := Run(context.Background(), Options{
		Code:        `function execute(p, context) { console.log("a", 1, {b: 2}); context.log("secret-value"); return 1 }`,
		Credentials: []Credential{{Key: "k", Type: db.ShortcutCredentialBearer, Value: "secret-value"}},
		Timeout:     time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, []string{`a 1 {"b":2}`, "******"}, result.Logs)
}

func TestFetch(t *testing.T) {
	allowPrivateNetwork = true
	t.Cleanup(func() { allowPrivateNetwork = false })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/echo":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"auth":"` + r.Header.Get("Authorization") + `","key":"` + r.URL.Query().Get("key") + `","method":"` + r.Method + `","type":"` + r.Header.Get("Content-Type") + `"}`))
		case "/redirect":
			http.Redirect(w, r, "http://example.com/", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	run := func(code string, credentials ...Credential) (*Result, error) {
		return Run(context.Background(), Options{
			Code:        code,
			Params:      map[string]interface{}{"base": server.URL},
			Domains:     []string{u.Hostname()},
			Credentials: credentials,
			Timeout:     5 * time.Second,
		})
	}

	result, err := run(`async function execute(p, context) {
		const resp = await context.fetch(p.base + "/echo", { method: "post", body: { a: 1 } }, "token")
		return { status: resp.status, ok: resp.ok, data: resp.json() }
	}`, Credential{Key: "token", Type: db.ShortcutCredentialBearer, Value: "s3cret"})
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{
		"status": float64(200),
		"ok":     true,
		"data":   map[string]interface{}{"auth": "Bearer s3cret", "key": "", "method": "POST", "type": "application/json"},
	}, result.Value)

	result, err = run(`function execute(p, context) {
		const resp = context.fetch(p.base + "/echo", {}, "q")
		return [resp.json().key, resp.url.includes("s3cret")]
	}`, Credential{Key: "q", Type: db.ShortcutCredentialQuery, Name: "key", Value: "s3cret"})
	require.NoError(t, err)
	require.Equal(t, []interface{}{"s3cret", false}, result.Value)

	_, err = run(`function execute(p, context) { return context.fetch("https://example.org/") }`)
	require.ErrorContains(t, err, "not in the allowed domains")

	_, err = run(`function execute(p, context) { return context.fetch(p.base + "/redirect").status }`)
	require.ErrorContains(t, err, "not in the allowed domains")

	_, err = run(`function execute(p, context) { return context.fetch(p.base + "/echo", {}, "missing") }`)
	require.ErrorContains(t, err, "unknown credential")

	_, err = run(`function execute(p, context) { return context.fetch("file:///etc/passwd") }`)
	require.ErrorContains(t, err, "unsupported protocol")

	result, err = run(`function execute(p, context) {
		try { context.fetch("https://example.org/") } catch (e) { return "caught" }
	}`)
	require.NoError(t, err)
	require.Equal(t, "caught", result.Value)

	_, err = run(`function execute(p, context) { for (let i = 0; i < 30; i++) context.fetch(p.base + "/echo") }`)
	require.ErrorContains(t, err, "at most")
}

func TestFetchBlocksPrivateNetwork(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	_, err = Run(context.Background(), Options{
		Code:    `function execute(p, context) { return context.fetch(p.url).status }`,
		Params:  map[string]interface{}{"url": server.URL},
		Domains: []string{u.Hostname()},
		Timeout: 5 * time.Second,
	})
	require.ErrorContains(t, err, "is not public")
}

func TestHostAllowed(t *testing.T) {
	domains := []string{"example.com", "api.test.io"}
	require.True(t, HostAllowed("example.com", domains))
	require.True(t, HostAllowed("A.Example.com.", domains))
	require.True(t, HostAllowed("api.test.io", domains))
	require.False(t, HostAllowed("test.io", domains))
	require.False(t, HostAllowed("badexample.com", domains))
	require.False(t, HostAllowed("example.com.evil.net", domains))
	require.False(t, HostAllowed("", domains))
}

func TestNormalizeDomain(t *testing.T) {
	for in, want := range map[string]string{" Example.COM. ": "example.com", "a-b.c": "a-b.c", "1.2.3.4": "1.2.3.4", "::1": "::1"} {
		got, ok := NormalizeDomain(in)
		require.True(t, ok, in)
		require.Equal(t, want, got)
	}
	for _, in := range []string{"", "https://example.com", "a_b.com", "-a.com", "example.com/x", "*.example.com"} {
		_, ok := NormalizeDomain(in)
		require.False(t, ok, in)
	}
}

func TestIsPublicAddr(t *testing.T) {
	for _, s := range []string{"8.8.8.8", "1.1.1.1", "2606:4700:4700::1111"} {
		require.True(t, IsPublicAddr(netip.MustParseAddr(s)), s)
	}
	for _, s := range []string{"127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254", "0.0.0.0", "100.64.0.1", "::1", "fe80::1", "fd00::1", "::ffff:127.0.0.1", "64:ff9b::a00:1"} {
		require.False(t, IsPublicAddr(netip.MustParseAddr(s)), s)
	}
}
