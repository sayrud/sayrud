package api

import (
	stdcontext "context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/hub"
	"github.com/wuhan005/sayrud/internal/i18n"
)

// signRelease returns the envelope JSON of the release of the source signed by the key, as Sayrud Hub signs it.
func signRelease(t *testing.T, key ed25519.PrivateKey, kind, id, version, source string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(source))
	payload, err := json.Marshal(hub.Statement{
		Issuer: hub.URL, Kind: kind, ID: id, Version: version, Language: "javascript",
		SHA256: hex.EncodeToString(sum[:]), Size: int64(len(source)), Official: true, IssuedAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	raw, err := json.Marshal(hub.Envelope{PayloadType: hub.PayloadType, Payload: payload, Signatures: []hub.Signature{{
		KeyID: hub.KeyID(key.Public().(ed25519.PublicKey)), Sig: ed25519.Sign(key, hub.PAE(hub.PayloadType, payload)),
	}}})
	require.NoError(t, err)
	return string(raw)
}

func newHubKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return key
}

func TestHubSignature(t *testing.T) {
	key := newHubKey(t)
	t.Cleanup(hub.UseForTest("http://127.0.0.1:0", key.Public().(ed25519.PublicKey)))
	const source = `export const manifest = {id: "greet", version: "1.0.0"}; export function execute() {}`
	signature := signRelease(t, key, hub.KindWorkflowPlugin, "greet", "1.0.0", source)
	expect := hub.Expect{Kind: hub.KindWorkflowPlugin, ID: "greet", Version: "1.0.0", Language: "javascript"}

	got, err := hubSignature(json.RawMessage(" "+signature+"\n"), "", expect, source)
	require.NoError(t, err)
	require.Equal(t, signature, got, "the given signature is stored compact")

	for name, tc := range map[string]struct {
		expect hub.Expect
		source string
	}{
		"modified source": {expect: expect, source: source + ";"},
		"another version": {expect: hub.Expect{Kind: hub.KindWorkflowPlugin, ID: "greet", Version: "1.0.1"}, source: source},
		"another kind":    {expect: hub.Expect{Kind: hub.KindFieldShortcut, ID: "greet"}, source: source},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := hubSignature(json.RawMessage(signature), "", tc.expect, tc.source)
			var localized *i18n.Error
			require.ErrorAs(t, err, &localized)
			require.Equal(t, "hub::invalid_signature", localized.Key)

			// Without a given signature, a stored one that no longer describes the source is dropped.
			got, err := hubSignature(nil, signature, tc.expect, tc.source)
			require.NoError(t, err)
			require.Empty(t, got)
		})
	}

	got, err = hubSignature(json.RawMessage("null"), signature, expect, source)
	require.NoError(t, err)
	require.Equal(t, signature, got, "the stored signature is kept while the source is unchanged")
	got, err = hubSignature(nil, "", expect, source)
	require.NoError(t, err)
	require.Empty(t, got)
	_, err = hubSignature(json.RawMessage(`{"payloadType":`), "", expect, source)
	require.Error(t, err)
}

// fakeHub serves the latest release of the uppercase_text shortcut and the update check.
func fakeHub(t *testing.T, key ed25519.PrivateKey, source string) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		sourcePath := "/open/v1/packages/field-shortcut/uppercase_text/versions/1.2.3/source.js"
		switch r.URL.Path {
		case "/open/v1/packages/field-shortcut/uppercase_text/versions/latest", "/open/v1/packages/field-shortcut/uppercase_text/versions/1.2.3":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": "success", "data": map[string]interface{}{
				"kind": hub.KindFieldShortcut, "id": "uppercase_text", "version": "1.2.3", "language": "javascript",
				"sourceURL": server.URL + sourcePath, "changelog": "Uppercase everything",
				"signature": json.RawMessage(signRelease(t, key, hub.KindFieldShortcut, "uppercase_text", "1.2.3", installableShortcut)),
			}})
		case sourcePath:
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = w.Write([]byte(source))
		case "/open/v1/updates":
			var body struct {
				Packages []hub.Installed `json:"packages"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			list := []map[string]interface{}{}
			for _, p := range body.Packages {
				list = append(list, map[string]interface{}{
					"kind": p.Kind, "id": p.ID, "version": p.Version, "found": true, "latest": map[string]interface{}{"version": "1.3.0"},
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"msg": "success", "data": map[string]interface{}{"packages": list}})
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"package::not_found","msg":"The package does not exist"}`))
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(hub.UseForTest(server.URL, key.Public().(ed25519.PublicKey)))
	return server
}

func TestFieldShortcutHubRoutes(t *testing.T) {
	previousStore := db.FieldShortcuts
	t.Cleanup(func() { db.FieldShortcuts = previousStore })
	store := &installShortcutsStore{}
	db.FieldShortcuts = store

	f := flamego.New()
	f.Use(i18n.Middleware(), context.Contexter(nil))
	f.Post("/inspect", form.Bind(form.InspectFieldShortcut{}), Admin.InspectFieldShortcut)
	f.Post("/install", form.Bind(form.InstallFieldShortcut{}), Admin.InstallFieldShortcut)
	request := func(path string, body interface{}, status int) (*dto.AdminFieldShortcut, string) {
		t.Helper()
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, path+"?lang=en", strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		f.ServeHTTP(response, req)
		require.Equal(t, status, response.Code, response.Body.String())
		var payload struct {
			Data *dto.AdminFieldShortcut `json:"data"`
			Msg  string                  `json:"msg"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
		return payload.Data, payload.Msg
	}

	key := newHubKey(t)
	server := fakeHub(t, key, installableShortcut)
	sourceURL := server.URL + "/open/v1/packages/field-shortcut/uppercase_text/versions/1.2.3/source.js"
	for _, input := range []form.InspectFieldShortcut{{HubID: "uppercase_text"}, {URL: sourceURL}} {
		preview, _ := request("/inspect", input, http.StatusOK)
		require.Equal(t, installableShortcut, preview.Code)
		require.NotNil(t, preview.Hub)
		require.Equal(t, "1.2.3", preview.Hub.Version)
		require.True(t, preview.Hub.Official)
		require.Equal(t, "Uppercase everything", preview.Hub.Changelog)
		require.Equal(t, hub.PageURL(hub.KindFieldShortcut, "uppercase_text"), preview.Hub.URL)
		require.NotEmpty(t, preview.Signature)

		_, msg := request("/install", form.InstallFieldShortcut{Source: preview.Code + "\n", Signature: preview.Signature}, http.StatusBadRequest)
		require.Contains(t, msg, "signature")
		require.Empty(t, store.created, "a signature must describe the installed source")

		installed, _ := request("/install", form.InstallFieldShortcut{Source: preview.Code, Language: preview.Language, Signature: preview.Signature}, http.StatusOK)
		require.NotNil(t, installed.Hub)
		require.Empty(t, installed.Signature, "the signature is only returned when inspecting")
		require.Len(t, store.created, 1)
		require.NotEmpty(t, store.created[0].HubSignature)
		store.created = nil
	}

	// A source of another kind, a missing package and a tampered source are refused.
	_, msg := request("/inspect", form.InspectFieldShortcut{URL: server.URL + "/open/v1/packages/workflow-plugin/uppercase_text/versions/1.2.3/source.js"}, http.StatusBadRequest)
	require.Contains(t, msg, "another kind")
	_, msg = request("/inspect", form.InspectFieldShortcut{HubID: "missing"}, http.StatusBadRequest)
	require.Contains(t, msg, "not available")

	fakeHub(t, key, strings.Replace(installableShortcut, "toUpperCase", "toLowerCase", 1))
	_, msg = request("/inspect", form.InspectFieldShortcut{HubID: "uppercase_text"}, http.StatusBadRequest)
	require.Contains(t, msg, "the source has been modified")

	// Installing the plain source does not claim it comes from the Hub.
	installed, _ := request("/install", form.InstallFieldShortcut{Source: installableShortcut}, http.StatusOK)
	require.Nil(t, installed.Hub)
	require.Empty(t, store.created[0].HubSignature)
}

type hubPluginsStore struct {
	db.WorkflowPluginsStore
	list []*db.WorkflowPlugin
}

func (s *hubPluginsStore) List(stdcontext.Context) ([]*db.WorkflowPlugin, error) {
	return s.list, nil
}

type hubShortcutsStore struct {
	db.FieldShortcutsStore
	list []*db.CustomFieldShortcut
}

func (s *hubShortcutsStore) List(stdcontext.Context) ([]*db.CustomFieldShortcut, error) {
	return s.list, nil
}

func TestCheckHubUpdates(t *testing.T) {
	previousPlugins, previousShortcuts := db.WorkflowPlugins, db.FieldShortcuts
	t.Cleanup(func() { db.WorkflowPlugins, db.FieldShortcuts = previousPlugins, previousShortcuts })

	key := newHubKey(t)
	fakeHub(t, key, installableShortcut)
	db.WorkflowPlugins = &hubPluginsStore{list: []*db.WorkflowPlugin{
		{PluginID: "local"},
		{PluginID: "greet", HubSignature: signRelease(t, key, hub.KindWorkflowPlugin, "greet", "1.0.0", "source")},
		{PluginID: "forged", HubSignature: signRelease(t, newHubKey(t), hub.KindWorkflowPlugin, "forged", "1.0.0", "source")},
	}}
	db.FieldShortcuts = &hubShortcutsStore{list: []*db.CustomFieldShortcut{
		{UID: "fscAAAAAAA", HubSignature: signRelease(t, key, hub.KindFieldShortcut, "uppercase_text", "1.2.3", installableShortcut)},
	}}

	f := flamego.New()
	f.Use(i18n.Middleware(), context.Contexter(nil))
	f.Get("/updates", Admin.CheckHubUpdates)
	response := httptest.NewRecorder()
	f.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/updates?refresh=true", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var payload struct {
		Data dto.HubUpdatesResp `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.False(t, payload.Data.CheckedAt.IsZero())

	got := map[string]string{}
	for _, p := range payload.Data.Packages {
		got[p.Kind+"/"+p.ID+"@"+p.Version] = p.Latest.Version
	}
	require.Equal(t, map[string]string{
		"workflow_plugin/greet@1.0.0":         "1.3.0",
		"field_shortcut/uppercase_text@1.2.3": "1.3.0",
	}, got, "only the packages verified as installed from the Hub are checked")
}
