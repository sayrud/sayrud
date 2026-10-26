package hub

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wuhan005/sayrud/internal/appconst"
)

const pluginSource = `export const manifest = {id: "slack", version: "1.2.0"}; export function execute() {}`

// fakeHub serves the release of the slack plugin and the update check, signed by a trusted test key.
type fakeHub struct {
	*httptest.Server
	key           ed25519.PrivateKey
	source        string
	signature     string
	updates       atomic.Int32
	lastBody      atomic.Value
	lastUserAgent atomic.Value
}

func newFakeHub(t *testing.T) *fakeHub {
	t.Helper()
	h := &fakeHub{key: newKey(t), source: pluginSource}
	trust(t, h.key.Public().(ed25519.PublicKey))
	h.signature = sign(t, h.key, statementOf(KindWorkflowPlugin, "slack", "1.2.0", pluginSource))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /open/v1/packages/workflow-plugin/slack/versions/{version}", func(w http.ResponseWriter, r *http.Request) {
		if v := r.PathValue("version"); v != LatestVersion && v != "1.2.0" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"package::release_not_found","msg":"The release does not exist"}`))
			return
		}
		writeData(w, map[string]interface{}{
			"kind": KindWorkflowPlugin, "id": "slack", "version": "1.2.0", "language": "javascript",
			"sourceURL": h.URL + "/open/v1/packages/workflow-plugin/slack/versions/1.2.0/source.js",
			"signature": json.RawMessage(h.signature), "changelog": "Faster", "official": true,
		})
	})
	mux.HandleFunc("GET /open/v1/packages/workflow-plugin/slack/versions/1.2.0/source.js", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(h.source))
	})
	mux.HandleFunc("POST /open/v1/updates", func(w http.ResponseWriter, r *http.Request) {
		h.updates.Add(1)
		var body struct {
			AppVersion string      `json:"appVersion"`
			Packages   []Installed `json:"packages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		h.lastBody.Store(body.AppVersion)
		list := []map[string]interface{}{}
		for _, p := range body.Packages {
			item := map[string]interface{}{"kind": p.Kind, "id": p.ID, "version": p.Version, "found": p.ID == "slack", "latest": nil}
			if p.ID == "slack" {
				item["latest"] = map[string]interface{}{"version": "1.2.0"}
				item["yanked"], item["yankReason"] = true, "Leaks tokens"
			}
			list = append(list, item)
		}
		writeData(w, map[string]interface{}{"packages": list})
	})
	h.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.lastUserAgent.Store(r.UserAgent())
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(h.Close)

	saved := baseURL
	baseURL = h.URL
	t.Cleanup(func() { baseURL = saved })
	return h
}

func writeData(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": "success", "data": data})
}

func TestRequestUserAgent(t *testing.T) {
	previousVersion := appconst.BuildVersion
	t.Cleanup(func() { appconst.BuildVersion = previousVersion })

	for _, tc := range []struct {
		name, version, want string
	}{
		{"development", "", "Sayrud/dev"},
		{"release tag", "v1.4.0", "Sayrud/1.4.0"},
		{"release version", "1.4.0", "Sayrud/1.4.0"},
		{"prerelease", "v1.5.0-rc.1", "Sayrud/1.5.0-rc.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			appconst.BuildVersion = tc.version
			h := newFakeHub(t)
			ctx := t.Context()

			_, err := GetRelease(ctx, KindWorkflowPlugin, "slack", LatestVersion, "en")
			require.NoError(t, err)
			require.Equal(t, tc.want, h.lastUserAgent.Load(), "release API request")

			_, err = Download(ctx, KindWorkflowPlugin, "slack", LatestVersion, "en", 1<<20)
			require.NoError(t, err)
			require.Equal(t, tc.want, h.lastUserAgent.Load(), "source download")

			_, err = CheckUpdates(ctx, "1.4.0", []Installed{{Kind: KindWorkflowPlugin, ID: "slack", Version: "1.0.0"}}, true)
			require.NoError(t, err)
			require.Equal(t, tc.want, h.lastUserAgent.Load(), "update API request")
		})
	}
}

func TestDownload(t *testing.T) {
	ctx := context.Background()

	t.Run("verified", func(t *testing.T) {
		h := newFakeHub(t)
		p, err := Download(ctx, KindWorkflowPlugin, "slack", LatestVersion, "en", 1<<20)
		require.NoError(t, err)
		require.Equal(t, pluginSource, p.Source)
		require.Equal(t, "1.2.0", p.Statement.Version)
		require.True(t, p.Statement.Official)
		require.Equal(t, "Faster", p.Release.Changelog)
		require.Equal(t, h.signature, p.Signature)
	})

	t.Run("tampered source", func(t *testing.T) {
		h := newFakeHub(t)
		h.source = strings.Replace(pluginSource, "execute", "steal", 1)
		_, err := Download(ctx, KindWorkflowPlugin, "slack", LatestVersion, "en", 1<<20)
		var verifyErr *VerifyError
		require.ErrorAs(t, err, &verifyErr)
		require.ErrorIs(t, err, ErrStatementMismatch)
	})

	t.Run("untrusted signature", func(t *testing.T) {
		newFakeHub(t)
		trust(t, newKey(t).Public().(ed25519.PublicKey))
		_, err := Download(ctx, KindWorkflowPlugin, "slack", LatestVersion, "en", 1<<20)
		require.ErrorIs(t, err, ErrUntrusted)
	})

	t.Run("too large", func(t *testing.T) {
		newFakeHub(t)
		_, err := Download(ctx, KindWorkflowPlugin, "slack", LatestVersion, "en", 10)
		var hubErr *Error
		require.ErrorAs(t, err, &hubErr)
	})

	t.Run("not found", func(t *testing.T) {
		newFakeHub(t)
		_, err := Download(ctx, KindWorkflowPlugin, "slack", "1.0.0", "en", 1<<20)
		require.ErrorIs(t, err, ErrNotFound)
		_, err = Download(ctx, KindFieldShortcut, "slack", LatestVersion, "en", 1<<20)
		require.ErrorIs(t, err, ErrNotFound)
	})

	t.Run("unreachable", func(t *testing.T) {
		h := newFakeHub(t)
		h.Close()
		_, err := Download(ctx, KindWorkflowPlugin, "slack", LatestVersion, "en", 1<<20)
		var hubErr *Error
		require.ErrorAs(t, err, &hubErr)
	})
}

func TestCheckUpdates(t *testing.T) {
	ctx := context.Background()
	h := newFakeHub(t)
	installed := []Installed{
		{Kind: KindWorkflowPlugin, ID: "slack", Version: "1.0.0"},
		{Kind: KindFieldShortcut, ID: "translate", Version: "2.0.0"},
		{Kind: KindWorkflowPlugin, ID: "slack", Version: "1.0.0"},
	}

	updates, err := CheckUpdates(ctx, "1.4.0", installed, true)
	require.NoError(t, err)
	require.Len(t, updates.Packages, 2, "the duplicated installations are checked once")
	require.Equal(t, "1.4.0", h.lastBody.Load())
	var slack *Update
	for _, u := range updates.Packages {
		if u.ID == "slack" {
			slack = u
		}
	}
	require.NotNil(t, slack)
	require.True(t, slack.Found)
	require.True(t, slack.Yanked)
	require.Equal(t, "Leaks tokens", slack.YankReason)
	require.Equal(t, "1.2.0", slack.Latest.Version)

	// The same check is served from the cache until it is refreshed or the installed packages change.
	_, err = CheckUpdates(ctx, "1.4.0", installed[:2], false)
	require.NoError(t, err)
	require.EqualValues(t, 1, h.updates.Load())
	_, err = CheckUpdates(ctx, "1.4.0", installed[:1], false)
	require.NoError(t, err)
	require.EqualValues(t, 2, h.updates.Load())
	_, err = CheckUpdates(ctx, "1.4.0", installed[:1], true)
	require.NoError(t, err)
	require.EqualValues(t, 3, h.updates.Load())

	empty, err := CheckUpdates(ctx, "1.4.0", nil, true)
	require.NoError(t, err)
	require.Empty(t, empty.Packages)
	require.EqualValues(t, 3, h.updates.Load(), "nothing is sent without an installed package")
}
