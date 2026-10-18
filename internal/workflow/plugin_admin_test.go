package workflow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/workflow/sandbox"
)

func TestDownloadPluginSourceLanguage(t *testing.T) {
	previous := db.Settings
	db.Settings = &pluginNetworkSettings{}
	t.Cleanup(func() { db.Settings = previous })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		_, _ = w.Write([]byte(greetPlugin))
	}))
	defer server.Close()

	for _, tc := range []struct {
		path     string
		fallback sandbox.Language
		want     sandbox.Language
	}{
		{path: "/plugin?format=ts", fallback: sandbox.TypeScript, want: sandbox.TypeScript},
		{path: "/raw/abc123", fallback: sandbox.TypeScript, want: sandbox.TypeScript},
		{path: "/plugin.txt", fallback: sandbox.TypeScript, want: sandbox.TypeScript},
		{path: "/plugin", fallback: sandbox.JavaScript, want: sandbox.JavaScript},
		{path: "/plugin.ts", fallback: sandbox.JavaScript, want: sandbox.TypeScript},
		{path: "/PLUGIN.MTS", fallback: sandbox.JavaScript, want: sandbox.TypeScript},
		{path: "/plugin.js", fallback: sandbox.TypeScript, want: sandbox.JavaScript},
		{path: "/plugin.mjs", fallback: sandbox.TypeScript, want: sandbox.JavaScript},
		{path: "/PLUGIN.CJS", fallback: sandbox.TypeScript, want: sandbox.JavaScript},
	} {
		t.Run(tc.path, func(t *testing.T) {
			source, lang, err := DownloadPluginSource(context.Background(), server.URL+tc.path, tc.fallback)
			require.NoError(t, err)
			assert.Equal(t, greetPlugin, source)
			assert.Equal(t, tc.want, lang)
		})
	}
}
