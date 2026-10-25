package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
)

func TestWorkflowPluginConversion(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest string
		label    string
	}{
		{name: "valid manifest", manifest: `{"id":"greet","version":"1.0.0","name":{"en":"Greet","zh-CN":"问候"}}`, label: "Greet"},
		{name: "invalid stored manifest", manifest: `{`, label: "Stored name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plugin := &db.WorkflowPlugin{
				PluginID: "greet", Name: "Stored name", Version: "1.0.0", Language: "typescript",
				Source: "export function execute() {}", Manifest: []byte(tc.manifest), Enabled: true,
			}

			// Conversion must work without a request or a database connection.
			resp := toWorkflowPlugin(context.Context{}, plugin)
			require.Equal(t, plugin.PluginID, resp.ID)
			require.Equal(t, plugin.Version, resp.Version)
			require.Equal(t, plugin.Language, resp.Language)
			require.Equal(t, "plugin:greet", resp.Module.Type)
			require.Equal(t, tc.label, resp.Module.Label)
			require.NotNil(t, resp.Domains)

			raw, err := json.Marshal(resp)
			require.NoError(t, err)
			require.NotContains(t, string(raw), `"usedBy"`)
			require.NotContains(t, string(raw), `"source"`)
		})
	}
}
