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
	"github.com/wuhan005/sayrud/internal/shortcut/script"
	"github.com/wuhan005/sayrud/internal/workflow/sandbox"
)

const installableShortcut = `
export const manifest = {
  name: "Uppercase", resultType: "text", aiEnabled: true,
  formItems: [{key: "text", label: "Text", component: "input", required: true}],
  domains: ["API.EXAMPLE.COM"],
  credentials: [{key: "api", type: "bearer", value: "ignored-secret"}]
};
export async function execute(params, context) {
  context.log("running");
  return params.text.toUpperCase();
}`

const installableTypeScriptShortcut = `
interface Params { text: string }
export const manifest = { name: "Typed shortcut", resultType: "text" } as const;
export function execute(params: Params): string { return params.text.toUpperCase() }
`

func TestInspectTypeScriptShortcut(t *testing.T) {
	s, err := inspectFieldShortcut(stdcontext.Background(), installableTypeScriptShortcut, sandbox.TypeScript)
	require.NoError(t, err)
	require.Equal(t, "typescript", s.Language)
	require.Equal(t, installableTypeScriptShortcut, s.Code)
	result, err := script.Run(stdcontext.Background(), script.Options{Code: s.Code, Language: s.Language, Params: map[string]interface{}{"text": "hello"}})
	require.NoError(t, err)
	require.Equal(t, "HELLO", result.Value)

	_, err = inspectFieldShortcut(stdcontext.Background(), installableTypeScriptShortcut, sandbox.JavaScript)
	require.Error(t, err)

	resp := dto.ToAdminFieldShortcut(s, nil, 0)
	require.Equal(t, "typescript", resp.Language)
}

func TestInspectFieldShortcut(t *testing.T) {
	s, err := inspectFieldShortcut(stdcontext.Background(), installableShortcut, sandbox.JavaScript)
	require.NoError(t, err)
	require.Equal(t, "Uppercase", s.Name)
	require.Equal(t, db.TextFieldType, s.ResultType)
	require.Equal(t, defaultShortcutTimeout, s.TimeoutSeconds)
	require.True(t, s.Enabled)
	require.True(t, s.AIEnabled)
	require.Equal(t, []string{"api.example.com"}, s.Domains.Data())
	require.Len(t, s.FormItems.Data(), 1)
	require.Equal(t, []db.ShortcutCredential{{Key: "api", Type: db.ShortcutCredentialBearer}}, s.Credentials.Data())
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
}

func TestInspectFieldShortcutValidation(t *testing.T) {
	for _, source := range []string{
		"", strings.Repeat(" ", maxShortcutCodeSize+1),
		`export const manifest = {};`,
		`export const manifest = []; export function execute() {}`,
		`export const manifest = {name: 1}; export function execute() {}`,
		`export const manifest = {name: "Invalid", resultType: "formula"}; export function execute() {}`,
		strings.Replace(installableShortcut, `component: "input"`, `component: "unknown"`, 1),
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
		if len(installed.Credentials) > 0 {
			require.False(t, installed.Credentials[0].HasValue)
		}
		require.Len(t, store.created, 1)
		require.Equal(t, preview.Code, store.created[0].Code)
		require.Equal(t, preview.Language, store.created[0].Language)
		store.created = nil
	}
	request("/inspect", form.InspectFieldShortcut{Source: installableTypeScriptShortcut, Language: "unknown"}, http.StatusBadRequest)
	request("/install", form.InstallFieldShortcut{Source: installableTypeScriptShortcut, Language: "javascript"}, http.StatusBadRequest)
}
