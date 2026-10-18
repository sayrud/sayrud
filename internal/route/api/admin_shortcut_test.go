package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/shortcut"
)

func TestBuildCustomShortcutLanguage(t *testing.T) {
	legacy, err := toAdminFieldShortcut(&db.CustomFieldShortcut{}, nil)
	require.NoError(t, err)
	require.Equal(t, "javascript", legacy.Language)

	for _, language := range []string{"", "javascript", "typescript", "unknown"} {
		f := form.SaveFieldShortcut{Name: "Typed", ResultType: "text", Language: language, Code: "function execute(params) { return params.text }"}
		if language == "typescript" {
			f.Code = "export function execute(params: {text: string}): string { return params.text.toUpperCase() }"
		}
		s, _, err := buildCustomShortcut(f)
		if language == "unknown" {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		want := language
		if want == "" {
			want = "javascript"
		}
		require.Equal(t, want, s.Language)
		resp, err := toAdminFieldShortcut(s, nil)
		require.NoError(t, err)
		require.Equal(t, want, resp.Language)
		if language == "typescript" {
			result, err := shortcut.RunCustom(context.Background(), s, nil, map[string]interface{}{"text": "hello"}, shortcut.Env{})
			require.NoError(t, err)
			require.Equal(t, "HELLO", result.Value)
		}
	}
}

func TestBuildCustomShortcutVersion(t *testing.T) {
	legacy, err := toAdminFieldShortcut(&db.CustomFieldShortcut{}, nil)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", legacy.Version)

	for _, tc := range []struct {
		version string
		want    string
	}{
		{want: "1.0.0"},
		{version: " 2.3.0-beta.1+build.4 ", want: "2.3.0-beta.1+build.4"},
		{version: "invalid version"},
		{version: "../invalid"},
		{version: "123456789012345678901234567890123"},
	} {
		s, _, err := buildCustomShortcut(form.SaveFieldShortcut{
			Name: "Versioned", Version: tc.version, ResultType: "text", Code: "function execute() { return 'text' }",
		})
		if tc.want == "" {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, tc.want, s.Version)
		resp, err := toAdminFieldShortcut(s, nil)
		require.NoError(t, err)
		require.Equal(t, tc.want, resp.Version)
	}
}

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
