package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/conf"
	appcontext "github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/shortcut"
)

func TestAdminShortcutLocalizedScriptError(t *testing.T) {
	f := flamego.New()
	f.Use(i18n.Middleware(), appcontext.Contexter(nil))
	f.Post("/test", form.Bind(form.TestFieldShortcut{}), Admin.TestFieldShortcut)
	for _, language := range []string{"javascript", "typescript"} {
		body, err := json.Marshal(form.TestFieldShortcut{
			Shortcut: form.SaveFieldShortcut{
				Name: "Localized", ResultType: "text", Language: language,
				Code: `export async function execute() { throw Object.assign(new Error("Request failed"), {
  i18n: {key: "request_failed", messages: {en: "HTTP {status}", "zh-CN": "状态码 {status}"}, params: {status: 429}}
}); }`,
			},
		})
		require.NoError(t, err)
		for lang, want := range map[string]string{"en": "The script failed: HTTP 429", "zh-CN": "脚本执行出错: 状态码 429", "de": "Das Skript ist fehlgeschlagen: HTTP 429"} {
			req := httptest.NewRequest(http.MethodPost, "/test?lang="+lang, strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			f.ServeHTTP(response, req)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var payload struct {
				Data dto.TestFieldShortcutResp `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
			require.Equal(t, want, payload.Data.Error, language+"/"+lang)
		}
	}
}

func TestBuildCustomShortcutCredentialLabel(t *testing.T) {
	for _, tc := range []struct {
		name  string
		label string
		want  string
	}{
		{name: "display name", label: "  服务访问令牌  ", want: "服务访问令牌"},
		{name: "omitted", want: "api"},
		{name: "blank", label: " \n ", want: "api"},
		{name: "character limit", label: strings.Repeat("令", 64), want: strings.Repeat("令", 64)},
		{name: "too long", label: strings.Repeat("令", 65)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, values, err := buildCustomShortcut(form.SaveFieldShortcut{
				Name: "Credential label", ResultType: "text", Code: "function execute() { return 'text' }",
				Credentials: []form.SaveShortcutCredential{{Key: "api", Label: i18n.LocalText{"": tc.label}, Type: "bearer", Value: "private-token"}},
			})
			if tc.want == "" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, s.Credentials.Data()[0].Label.In("en"))
			require.Equal(t, map[string]string{"api": "private-token"}, values)

			// Credential definitions survive JSON storage without containing their values.
			raw, err := json.Marshal(s.Credentials)
			require.NoError(t, err)
			require.NotContains(t, string(raw), "private-token")
			require.NoError(t, json.Unmarshal(raw, &s.Credentials))
			resp := dto.ToAdminFieldShortcut(s, map[string]bool{"api": true}, 0)
			require.Equal(t, tc.want, resp.Credentials[0].Label.In("en"))
			require.True(t, resp.Credentials[0].HasValue)
		})
	}
}

func TestCredentialLabelChangePreservesSecret(t *testing.T) {
	previousKey := conf.Auth.SecretKey
	t.Cleanup(func() { conf.Auth.SecretKey = previousKey })
	conf.Auth.SecretKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

	sealed, err := sealSecrets("", []db.ShortcutCredential{{Key: "api", Label: i18n.LocalText{"": "API Key"}, Type: db.ShortcutCredentialBearer}}, map[string]string{"api": "private-token"})
	require.NoError(t, err)
	sealed, err = sealSecrets(sealed, []db.ShortcutCredential{{Key: "api", Label: i18n.LocalText{"": "New display name"}, Type: db.ShortcutCredentialBearer}}, nil)
	require.NoError(t, err)
	values, err := shortcut.DecodeSecrets(sealed)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"api": "private-token"}, values)
}

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
		FormItems: []db.ShortcutFormItem{{Key: "categories", Label: i18n.LocalText{"": "Categories"}, Component: db.ShortcutFormFieldOptions, Required: true,
			Default: "stale", FieldTypes: []db.SLFieldType{db.TextFieldType}, Options: []db.ShortcutFormOption{{Value: "stale"}}}},
	}
	s, _, err := buildCustomShortcut(f)
	require.NoError(t, err)
	require.Equal(t, []db.ShortcutFormItem{{Key: "categories", Label: i18n.LocalText{"": "Categories"}, Component: db.ShortcutFormFieldOptions, Required: true}}, s.FormItems.Data())
	f.ResultType = "multi_select"
	_, _, err = buildCustomShortcut(f)
	require.NoError(t, err)
	f.ResultType = "text"
	_, _, err = buildCustomShortcut(f)
	require.Error(t, err)
}
