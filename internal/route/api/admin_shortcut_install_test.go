package api

import (
	stdcontext "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/shortcut"
	"github.com/wuhan005/sayrud/internal/shortcut/script"
	"github.com/wuhan005/sayrud/internal/workflow/sandbox"
)

const installableShortcut = `
export const manifest = {
  id: "uppercase_text", homepage: "https://example.com/uppercase",
  name: "Uppercase", icon: "tag", color: "#3370ff", version: "1.2.3", resultType: "text",
  permissions: {fetch: ["API.EXAMPLE.COM"], ai: true},
  credentials: [{key: "api", label: "API Key", type: "bearer", value: "ignored-secret"}],
  sections: [{fields: [{key: "text", label: "Text", widget: "text", required: true}]}]
};
export async function execute(params, context) {
  context.log("running");
  return params.text.toUpperCase();
}`

const installableTypeScriptShortcut = `
interface Params { text: string }
export const manifest = { name: "Typed shortcut", version: "2.0.0-beta.1", resultType: "text" } as const;
export function execute(params: Params): string { return params.text.toUpperCase() }
`

func TestInspectTypeScriptShortcut(t *testing.T) {
	s, err := inspectFieldShortcut(stdcontext.Background(), installableTypeScriptShortcut, sandbox.TypeScript)
	require.NoError(t, err)
	require.Equal(t, "typescript", s.Language)
	require.Equal(t, "2.0.0-beta.1", s.Version)
	require.Equal(t, installableTypeScriptShortcut, s.Code)
	result, err := script.Run(stdcontext.Background(), script.Options{Code: s.Code, Language: s.Language, Params: map[string]interface{}{"text": "hello"}})
	require.NoError(t, err)
	require.Equal(t, "HELLO", result.Value)

	_, err = inspectFieldShortcut(stdcontext.Background(), installableTypeScriptShortcut, sandbox.JavaScript)
	require.Error(t, err)

	resp := dto.ToAdminFieldShortcut(s, nil, 0)
	require.Equal(t, "typescript", resp.Language)
}

func TestInspectFieldShortcutNamespacedID(t *testing.T) {
	source := strings.Replace(installableShortcut, `id: "uppercase_text"`, `id: "com.sayrud.smart_classification"`, 1)
	s, err := inspectFieldShortcut(stdcontext.Background(), source, sandbox.JavaScript)
	require.NoError(t, err)
	require.Equal(t, "com.sayrud.smart_classification", s.ManifestID)
	require.Equal(t, s.ManifestID, dto.ToAdminFieldShortcut(s, nil, 0).ID)
}

func TestInspectFieldShortcut(t *testing.T) {
	s, err := inspectFieldShortcut(stdcontext.Background(), installableShortcut, sandbox.JavaScript)
	require.NoError(t, err)
	require.Equal(t, "uppercase_text", s.ManifestID)
	require.Equal(t, "https://example.com/uppercase", s.Homepage)
	require.Equal(t, "tag", s.Icon)
	require.Equal(t, "tag", dto.ToAdminFieldShortcut(s, nil, 0).Icon)
	require.Equal(t, "#3370ff", s.Color)
	require.Equal(t, "#3370ff", dto.ToAdminFieldShortcut(s, nil, 0).Color)
	require.Equal(t, "#3370ff", toManifest(s, true).Color)
	require.Equal(t, i18n.LocalText{"": "Uppercase"}, s.Name)
	require.Equal(t, "1.2.3", s.Version)
	require.Equal(t, db.TextFieldType, s.ResultType)
	require.Equal(t, shortcut.DefaultTimeout, s.TimeoutSeconds)
	require.True(t, s.Enabled)
	require.True(t, s.AIEnabled)
	require.Equal(t, []string{"api.example.com"}, s.Domains.Data())
	require.Equal(t, []db.ShortcutField{{Key: "text", Widget: db.ShortcutWidgetText, Label: i18n.LocalText{"": "Text"}, Required: true}}, s.Fields())
	require.Equal(t, []db.ShortcutCredential{{Key: "api", Label: i18n.LocalText{"": "API Key"}, Type: db.ShortcutCredentialBearer}}, s.Credentials.Data())
	require.Empty(t, s.Secrets)
	require.Empty(t, s.UID, "inspection must not install the shortcut")

	result, err := script.Run(stdcontext.Background(), script.Options{Code: s.Code, Params: map[string]interface{}{"text": "hello"}})
	require.NoError(t, err)
	require.Equal(t, "HELLO", result.Value)
	require.Equal(t, []string{"running"}, result.Logs)

	// The downloadable template must pass the same installation and execution path.
	template, err := os.ReadFile("../../../frontend/src/assets/templates/field-shortcut.js")
	require.NoError(t, err)
	s, err = inspectFieldShortcut(stdcontext.Background(), string(template), sandbox.JavaScript)
	require.NoError(t, err)
	result, err = script.Run(stdcontext.Background(), script.Options{Code: s.Code, Params: map[string]interface{}{"text": "example"}})
	require.NoError(t, err)
	require.Equal(t, "EXAMPLE", result.Value)
	require.Equal(t, "1.0.0", s.Version)
	require.Equal(t, "uppercase_text", s.ManifestID)

	_, err = inspectFieldShortcut(stdcontext.Background(), strings.Replace(installableShortcut, `version: "1.2.3", `, "", 1), sandbox.JavaScript)
	requireManifestError(t, err, "version is required, e.g. 1.0.0")
}

func TestInspectFieldShortcutRenamedProperties(t *testing.T) {
	for old, replacement := range map[string]string{
		`permissions: {fetch: ["API.EXAMPLE.COM"], ai: true}`: `domains: ["api.example.com"]`,
		`sections: [{fields:`: `formItems: [{fields:`,
		`resultType: "text",`: `resultType: "text", timeoutSeconds: 10,`,
		`ai: true}`:           `ai: true}, aiEnabled: true`,
	} {
		_, err := inspectFieldShortcut(stdcontext.Background(), strings.Replace(installableShortcut, old, replacement, 1), sandbox.JavaScript)
		require.Error(t, err, replacement)
		var localized *i18n.Error
		require.ErrorAs(t, err, &localized)
		require.Equal(t, "workflow_plugin::invalid_manifest", localized.Key)
		require.Contains(t, localized.Args[0], "is renamed to", replacement)
	}
	_, err := inspectFieldShortcut(stdcontext.Background(), strings.Replace(installableShortcut, `widget: "text"`, `component: "input"`, 1), sandbox.JavaScript)
	requireManifestError(t, err, `field "text" uses component, which is renamed to widget`)
}

func TestInspectFieldShortcutValidation(t *testing.T) {
	for _, source := range []string{
		"", strings.Repeat(" ", maxShortcutCodeSize+1),
		`export const manifest = {};`,
		`export const manifest = []; export function execute() {}`,
		`export const manifest = {name: 1}; export function execute() {}`,
		`export const manifest = {name: "Invalid", version: "1.0.0", resultType: "formula"}; export function execute() {}`,
		strings.Replace(installableShortcut, `label: "API Key"`, `label: 123`, 1),
		strings.Replace(installableShortcut, `widget: "text"`, `widget: "unknown"`, 1),
		strings.Replace(installableShortcut, `widget: "text"`, `widget: "input"`, 1),
		strings.Replace(installableShortcut, `version: "1.2.3"`, `version: "invalid version"`, 1),
		strings.Replace(installableShortcut, `version: "1.2.3"`, `version: 123`, 1),
		strings.Replace(installableShortcut, `"API.EXAMPLE.COM"`, `"https://example.com/path"`, 1),
		`import data from "other.js"; ` + installableShortcut,
		`throw new Error("top-level failure"); ` + installableShortcut,
	} {
		_, err := inspectFieldShortcut(stdcontext.Background(), source, sandbox.JavaScript)
		require.Error(t, err)
	}

	_, err := inspectFieldShortcut(stdcontext.Background(), strings.Replace(installableShortcut, `return params.text.toUpperCase();`, `throw new Error("execute must not run during inspection");`, 1), sandbox.JavaScript)
	require.NoError(t, err)

	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = inspectFieldShortcut(ctx, `while (true) {} `+installableShortcut, sandbox.JavaScript)
	require.Error(t, err)
}

func TestLocalizedShortcutConfiguration(t *testing.T) {
	source := `export const manifest = {
  name: {en: "Localized", "zh-CN": "本地化"}, version: "1.0.0", description: {en: "Picks a tone", "zh-CN": "选择语气"}, resultType: "text",
  sections: [{title: {en: "Style", "zh-CN": "风格"}, fields: [{key: "tone", label: {en: "Tone", "zh-CN": "语气"}, widget: "select", required: true,
    help: {en: "How it sounds", "zh-CN": "听起来如何"}, placeholder: {en: "Choose a tone", "zh-CN": "选择语气"}, default: "formal",
    options: [{value: "formal", label: {en: "Formal", "zh-CN": "正式"}}]}]}],
  credentials: [{key: "api", label: {en: "API Key", "zh-CN": "API 密钥"}, type: "bearer"}]
};
export function execute(params) { return params.tone; }`
	s, err := inspectFieldShortcut(stdcontext.Background(), source, sandbox.JavaScript)
	require.NoError(t, err)
	require.Equal(t, "本地化", s.Name.In("zh-CN"))
	require.Equal(t, "Picks a tone", s.Description.In("en"))
	require.Equal(t, "风格", s.Sections.Data()[0].Title.In("zh-CN"))
	require.Equal(t, "语气", s.Fields()[0].Label.In("zh-CN"))
	require.Equal(t, "听起来如何", s.Fields()[0].Help.In("zh-CN"))
	require.Equal(t, "Choose a tone", s.Fields()[0].Placeholder.In("en"))
	require.Equal(t, "正式", s.Fields()[0].Options[0].Label.In("zh-CN"))
	require.Equal(t, "API 密钥", s.Credentials.Data()[0].Label.In("zh-CN"))

	// Editing credentials or toggling the shortcut must preserve every translation.
	raw, err := json.Marshal(dto.ToAdminFieldShortcut(s, nil, 0))
	require.NoError(t, err)
	var save form.SaveFieldShortcut
	require.NoError(t, json.Unmarshal(raw, &save))
	saved, _, err := buildCustomShortcut(save)
	require.NoError(t, err)
	require.Equal(t, s.Name, saved.Name)
	require.Equal(t, s.Description, saved.Description)
	require.Equal(t, s.Sections.Data(), saved.Sections.Data())
	require.Equal(t, s.Credentials.Data(), saved.Credentials.Data())
	result, err := script.Run(stdcontext.Background(), script.Options{Code: saved.Code, Params: map[string]interface{}{"tone": "formal"}})
	require.NoError(t, err)
	require.Equal(t, "formal", result.Value, "translation must not change the stored option value")

	for _, invalid := range []string{
		strings.Replace(source, `"zh-CN": "语气"`, `"zh-CN": "`+strings.Repeat("界", 501)+`"`, 1),
		strings.Replace(source, `"zh-CN": "选择语气"`, `"zh-CN": "`+strings.Repeat("界", 501)+`"`, 1),
		strings.Replace(source, `"zh-CN": "正式"`, `"zh-CN": 123`, 1),
		strings.Replace(source, `"zh-CN": "API 密钥"`, `"zh-CN": "`+strings.Repeat("界", 501)+`"`, 1),
		strings.Replace(source, `default: "formal"`, `default: {en: "formal"}`, 1),
	} {
		_, err := inspectFieldShortcut(stdcontext.Background(), invalid, sandbox.JavaScript)
		require.Error(t, err)
	}
}

type installShortcutsStore struct {
	db.FieldShortcutsStore
	created []*db.CustomFieldShortcut
}

func (s *installShortcutsStore) Create(_ stdcontext.Context, shortcut *db.CustomFieldShortcut) error {
	shortcut.UID = db.NewCustomFieldShortcutUID()
	s.created = append(s.created, shortcut)
	return nil
}

func TestFieldShortcutInstallRoutes(t *testing.T) {
	previousStore, previousSettings := db.FieldShortcuts, db.Settings
	t.Cleanup(func() { db.FieldShortcuts, db.Settings = previousStore, previousSettings })
	store := &installShortcutsStore{}
	settings := &adminSystemSettingsStore{settings: db.DefaultSystemSettings()}
	db.FieldShortcuts, db.Settings = store, settings

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		if strings.HasSuffix(r.URL.Path, ".mts") {
			_, _ = w.Write([]byte(installableTypeScriptShortcut))
		} else {
			_, _ = w.Write([]byte(installableShortcut))
		}
	}))
	defer server.Close()

	f := flamego.New()
	f.Use(i18n.Middleware(), context.Contexter(nil))
	f.Post("/inspect", form.Bind(form.InspectFieldShortcut{}), Admin.InspectFieldShortcut)
	f.Post("/install", form.Bind(form.InstallFieldShortcut{}), Admin.InstallFieldShortcut)
	request := func(path string, body interface{}, status int) *dto.AdminFieldShortcut {
		t.Helper()
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		f.ServeHTTP(response, req)
		require.Equal(t, status, response.Code, response.Body.String())
		if status != http.StatusOK {
			return nil
		}
		var payload struct {
			Data *dto.AdminFieldShortcut `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
		return payload.Data
	}

	request("/inspect", form.InspectFieldShortcut{URL: "file:///tmp/shortcut.js"}, http.StatusBadRequest)
	request("/inspect", form.InspectFieldShortcut{URL: server.URL}, http.StatusBadRequest)
	settings.settings.NetworkAllowlist = []string{"127.0.0.1", "::1"}

	for _, input := range []form.InspectFieldShortcut{
		{Source: installableShortcut}, {URL: server.URL + "/shortcut.js"},
		{Source: installableTypeScriptShortcut, Language: "typescript"}, {URL: server.URL + "/shortcut.mts"},
	} {
		preview := request("/inspect", input, http.StatusOK)
		if input.Language == "typescript" || strings.HasSuffix(input.URL, ".mts") {
			require.Equal(t, "typescript", preview.Language)
			require.Equal(t, installableTypeScriptShortcut, preview.Code)
		} else {
			require.Equal(t, "javascript", preview.Language)
			require.Equal(t, installableShortcut, preview.Code)
		}
		require.Empty(t, store.created)

		request("/install", form.InstallFieldShortcut{Source: `export const manifest = {};`}, http.StatusBadRequest)
		require.Empty(t, store.created, "installation must validate the source again")
		installed := request("/install", form.InstallFieldShortcut{Source: preview.Code, Language: preview.Language}, http.StatusOK)
		require.NotEmpty(t, installed.UID)
		require.Equal(t, preview.Version, installed.Version)
		require.Equal(t, preview.ID, installed.ID)
		require.Equal(t, preview.Homepage, installed.Homepage)
		require.NotEqual(t, installed.ID, installed.UID)
		require.Equal(t, installed.ID, store.created[0].ManifestID)
		require.Equal(t, installed.Homepage, store.created[0].Homepage)
		if len(installed.Credentials) > 0 {
			require.Equal(t, "API Key", preview.Credentials[0].Label.In("en"))
			require.Equal(t, "API Key", installed.Credentials[0].Label.In("en"))
			require.False(t, installed.Credentials[0].HasValue)
		}
		require.Len(t, store.created, 1)
		require.Equal(t, preview.Code, store.created[0].Code)
		require.Equal(t, preview.Language, store.created[0].Language)
		require.Equal(t, preview.Version, store.created[0].Version)
		store.created = nil
	}
	request("/inspect", form.InspectFieldShortcut{Source: installableTypeScriptShortcut, Language: "unknown"}, http.StatusBadRequest)
	request("/install", form.InstallFieldShortcut{Source: installableTypeScriptShortcut, Language: "javascript"}, http.StatusBadRequest)
}
