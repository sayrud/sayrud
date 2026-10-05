package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
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

func TestBuildCustomShortcutFieldOptions(t *testing.T) {
	f := form.SaveFieldShortcut{
		Name: "Category", ResultType: "single_select", Code: "function execute(params) { return params.categories[0] }",
		FormItems: []db.ShortcutFormItem{{Key: "categories", Label: "Categories", Component: db.ShortcutFormFieldOptions, Required: true,
			Default: "stale", FieldTypes: []db.SLFieldType{db.TextFieldType}, Options: []db.ShortcutFormOption{{Value: "stale"}}}},
	}
	s, _, err := buildCustomShortcut(f)
	require.NoError(t, err)
	require.Equal(t, []db.ShortcutFormItem{{Key: "categories", Label: "Categories", Component: db.ShortcutFormFieldOptions, Required: true}}, s.FormItems.Data())
	f.ResultType = "multi_select"
	_, _, err = buildCustomShortcut(f)
	require.NoError(t, err)
	f.ResultType = "text"
	_, _, err = buildCustomShortcut(f)
	require.Error(t, err)
}
