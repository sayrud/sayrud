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
				Manifest: shortcut.Manifest{Name: i18n.LocalText{"": "Localized"}, Version: "1.0.0", ResultType: "text"},
				Language: language,
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

// requireManifestError checks that err shows the invalid manifest message like the workflow plugins.
func requireManifestError(t *testing.T, err error, message string) {
	t.Helper()
	var localized *i18n.Error
	require.ErrorAs(t, err, &localized)
	require.Equal(t, "workflow_plugin::invalid_manifest", localized.Key)
	require.Equal(t, []interface{}{message}, localized.Args)
}

// saveForm is a valid form of a text shortcut with the code.
func saveForm(code string) form.SaveFieldShortcut {
	return form.SaveFieldShortcut{
		Manifest: shortcut.Manifest{Name: i18n.LocalText{"": "Test"}, Version: "1.0.0", ResultType: "text"},
		Code:     code,
	}
}

func TestBuildCustomShortcutCredentials(t *testing.T) {
	for _, tc := range []struct {
		name  string
		label string
		want  string
		err   string
	}{
		{name: "display name", label: "服务访问令牌", want: "服务访问令牌"},
		{name: "omitted", want: "api"},
		{name: "blank", label: " \n ", want: "api"},
		{name: "character limit", label: strings.Repeat("令", 500), want: strings.Repeat("令", 500)},
		{name: "too long", label: strings.Repeat("令", 501), err: `the label of credential "api" is longer than 500 characters`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := saveForm("function execute() { return 'text' }")
			f.Credentials = []db.ShortcutCredential{{Key: "api", Label: i18n.LocalText{"": tc.label}, Type: "bearer", Name: "ignored"}}
			f.Secrets = map[string]string{"api": "private-token", "removed": "stale"}
			s, values, err := buildCustomShortcut(f)
			if tc.err != "" {
				requireManifestError(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			require.Empty(t, s.Credentials.Data()[0].Name, "bearer credentials have no name")
			require.Equal(t, map[string]string{"api": "private-token"}, values, "only the values of the defined credentials are kept")

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
		f := saveForm("function execute(params) { return params.text }")
		f.Language = language
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
	require.Equal(t, "1.0.0", legacy.Version, "the shortcuts saved before the version was added")

	for _, version := range []string{"1.0.0", "2.3.0-beta.1+build.4"} {
		f := saveForm("function execute() { return 'text' }")
		f.Version = version
		s, _, err := buildCustomShortcut(f)
		require.NoError(t, err)
		require.Equal(t, version, s.Version)
	}
	for _, version := range []string{"", " 2.3.0 ", "invalid version", "../invalid", "123456789012345678901234567890123"} {
		f := saveForm("function execute() { return 'text' }")
		f.Version = version
		_, _, err := buildCustomShortcut(f)
		requireManifestError(t, err, "version is required, e.g. 1.0.0")
	}
}

func TestBuildCustomShortcutPermissions(t *testing.T) {
	for _, ai := range []bool{true, false} {
		f := saveForm("async function execute(params, context) { return params.text; }")
		f.Permissions = db.ShortcutPermissions{Fetch: []string{"*.Example.com", "example.com", "api.test.io"}, AI: ai}
		f.Timeout = 120
		s, _, err := buildCustomShortcut(f)
		require.NoError(t, err)
		require.Equal(t, ai, s.AIEnabled)
		require.Equal(t, []string{"api.test.io", "example.com"}, s.Domains.Data())
		require.Equal(t, 120, s.TimeoutSeconds)

		payload, err := json.Marshal(dto.ToAdminFieldShortcut(s, nil, 0))
		require.NoError(t, err)
		require.Contains(t, string(payload), `"permissions":{"fetch":["api.test.io","example.com"],"ai":`+map[bool]string{true: "true", false: "false"}[ai]+`}`)
		require.Contains(t, string(payload), `"timeout":120`)
	}

	s, _, err := buildCustomShortcut(saveForm("function execute() { return 'text' }"))
	require.NoError(t, err)
	require.Equal(t, 30, s.TimeoutSeconds)
	require.Equal(t, []string{}, s.Domains.Data())
}

func TestBuildCustomShortcutFieldOptions(t *testing.T) {
	f := saveForm("function execute(params) { return params.categories[0] }")
	f.ResultType = "single_select"
	f.Sections = []db.ShortcutSection{{Fields: []db.ShortcutField{{Key: "categories", Label: i18n.LocalText{"": "Categories"}, Widget: db.ShortcutWidgetFieldOptions, Required: true,
		Default: "stale", FieldTypes: []db.SLFieldType{db.TextFieldType}, Options: []db.ShortcutOption{{Value: "stale"}}}}}}
	s, _, err := buildCustomShortcut(f)
	require.NoError(t, err)
	require.Equal(t, []db.ShortcutField{{Key: "categories", Label: i18n.LocalText{"": "Categories"}, Widget: db.ShortcutWidgetFieldOptions, Required: true}}, s.Fields())
	f.ResultType = "multi_select"
	_, _, err = buildCustomShortcut(f)
	require.NoError(t, err)
	f.ResultType = "text"
	_, _, err = buildCustomShortcut(f)
	requireManifestError(t, err, `field "categories" edits the field options, which needs the resultType single_select or multi_select`)
}
