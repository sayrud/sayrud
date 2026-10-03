package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/form"
)

func TestBuildCustomShortcutAIEnabled(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		s, _, err := buildCustomShortcut(form.SaveFieldShortcut{
			Name:       "Custom AI shortcut",
			ResultType: "text",
			Code:       "async function execute(params, context) { return params.text; }",
			AIEnabled:  enabled,
		})
		require.NoError(t, err)
		require.Equal(t, enabled, s.AIEnabled)

		resp, err := toAdminFieldShortcut(s, nil)
		require.NoError(t, err)
		require.Equal(t, enabled, resp.AIEnabled)

		payload, err := json.Marshal(resp)
		require.NoError(t, err)
		if enabled {
			require.Contains(t, string(payload), `"aiEnabled":true`)
		} else {
			require.Contains(t, string(payload), `"aiEnabled":false`)
		}
	}
}
