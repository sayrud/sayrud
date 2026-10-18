package workflow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/workflow/sandbox"
)

func TestPluginEmptyConfig(t *testing.T) {
	source := `
export const manifest = {
  id: "defaults", version: "1", name: "Defaults",
  sections: [{ fields: [{ key: "message", widget: "value", default: { kind: "const", const: "hello" } }] }]
};
export function execute(config) { return config.message; }
`
	steps := []Step{
		pluginStep(t, `{"id":"omitted","type":"plugin:defaults"}`),
		pluginStep(t, `{"id":"null","type":"plugin:defaults","config":null}`),
		pluginStep(t, `{"id":"empty","type":"plugin:defaults","config":{}}`),
		{ID: "nil", Type: "plugin:defaults"},
	}
	installTestPlugins(t, map[string]bool{source: true})

	for _, step := range steps {
		t.Run(step.ID, func(t *testing.T) {
			if step.Config != nil {
				assert.JSONEq(t, `{}`, string(step.Config.(json.RawMessage)))
			}
			x := newPluginExecution(nil, false)
			_, err := x.runSteps([]Step{step})
			require.NoError(t, err)
			assert.Equal(t, "hello", x.scope["steps"].(map[string]interface{})[step.ID])
		})
	}
}

type pluginNetworkSettings struct{ db.SettingsStore }

func (*pluginNetworkSettings) GetSystem(context.Context) (*db.SystemSettings, error) {
	return &db.SystemSettings{NetworkAllowlist: []string{"127.0.0.1", "::1"}}, nil
}

func TestPluginFetchRedirectPermissions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		domains []string
		chain   bool
		user    bool
		blocked bool
	}{
		{name: "blocked initial host", status: http.StatusFound, domains: []string{"localhost"}, blocked: true},
		{name: "blocked 302", status: http.StatusFound, domains: []string{"127.0.0.1"}, blocked: true},
		{name: "blocked 307", status: http.StatusTemporaryRedirect, domains: []string{"127.0.0.1"}, blocked: true},
		{name: "blocked 308", status: http.StatusPermanentRedirect, domains: []string{"127.0.0.1"}, blocked: true},
		{name: "blocked second hop", status: http.StatusTemporaryRedirect, domains: []string{"127.0.0.1"}, chain: true, blocked: true},
		{name: "declared destination", status: http.StatusTemporaryRedirect, domains: []string{"127.0.0.1", "localhost"}},
		{name: "wildcard destination", status: http.StatusPermanentRedirect, domains: []string{"*"}},
		{name: "credentials in redirect", status: http.StatusTemporaryRedirect, domains: []string{"*"}, user: true, blocked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var received atomic.Int32
			destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received.Add(1)
				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				_, _ = w.Write([]byte(r.Method + ":" + string(body)))
			}))
			defer destination.Close()
			target := strings.Replace(destination.URL, "127.0.0.1", "localhost", 1)
			if tc.user {
				target = strings.Replace(target, "http://", "http://user:password@", 1)
			}
			redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.chain && r.URL.Path == "/" {
					http.Redirect(w, r, "/next", http.StatusTemporaryRedirect)
					return
				}
				http.Redirect(w, r, target, tc.status)
			}))
			defer redirect.Close()

			actions := &DefaultActions{Settings: func() db.SettingsStore { return &pluginNetworkSettings{} }}
			result, err := pluginFetch(actions, tc.domains)(context.Background(), sandbox.FetchRequest{Method: http.MethodPost, URL: redirect.URL, Body: "private payload"})
			if tc.blocked {
				var actionErr *ActionError
				require.ErrorAs(t, err, &actionErr)
				assert.False(t, actionErr.Transient)
				assert.Nil(t, result)
				assert.Zero(t, received.Load())
			} else {
				require.NoError(t, err)
				assert.Equal(t, "POST:private payload", result.Body)
				assert.Equal(t, int32(1), received.Load())
			}
		})
	}
}

func TestPluginFetchBlocksPrivateNetworks(t *testing.T) {
	actions := &DefaultActions{Settings: func() db.SettingsStore { return nil }}
	_, err := pluginFetch(actions, []string{"*"})(context.Background(), sandbox.FetchRequest{URL: "http://127.0.0.1:1"})
	var actionErr *ActionError
	require.ErrorAs(t, err, &actionErr)
	assert.Contains(t, actionErr.Message, "is not public")
	assert.False(t, actionErr.Transient)
}

func TestPluginFetchRedirectSchemes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		via     []string
		target  string
		blocked bool
	}{
		{name: "https destination", via: []string{"https://api.example.com/start"}, target: "https://api.example.com/end"},
		{name: "http destination", via: []string{"http://api.example.com/start"}, target: "http://api.example.com/end"},
		{name: "upgrade", via: []string{"http://api.example.com/start"}, target: "https://api.example.com/end"},
		{name: "downgrade", via: []string{"https://api.example.com/start"}, target: "http://api.example.com/end", blocked: true},
		{name: "downgrade after upgrade", via: []string{"http://api.example.com/start", "https://api.example.com/next"}, target: "http://api.example.com/end", blocked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var via []*http.Request
			for _, address := range tc.via {
				via = append(via, httptest.NewRequest(http.MethodGet, address, nil))
			}
			err := checkPluginFetchRedirect(httptest.NewRequest(http.MethodGet, tc.target, nil), via, []string{"*"})
			if tc.blocked {
				var actionErr *ActionError
				require.ErrorAs(t, err, &actionErr)
				assert.Contains(t, actionErr.Message, "insecure URL")
				assert.False(t, actionErr.Transient)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
