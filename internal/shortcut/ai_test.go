package shortcut

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/ai"
	"github.com/wuhan005/sayrud/internal/ai/openai"
	"github.com/wuhan005/sayrud/internal/conf"
	"github.com/wuhan005/sayrud/internal/db"
)

type testAISettingsStore struct {
	db.SettingsStore
	settings db.AISettings
	reads    int
}

func (s *testAISettingsStore) GetAI(context.Context) (*db.AISettings, error) {
	s.reads++
	return &s.settings, nil
}

func TestRunCustomAI(t *testing.T) {
	oldStore, oldSecret := db.Settings, conf.Auth.SecretKey
	t.Cleanup(func() { db.Settings, conf.Auth.SecretKey = oldStore, oldSecret })
	conf.Auth.SecretKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	const apiKey = "global-model-key"

	sealed, err := ai.SealAPIKey(apiKey)
	require.NoError(t, err)

	type request struct {
		Model    string
		Messages []openai.Message
		Auth     string
	}
	requests := make(chan request, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got request
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		got.Auth = r.Header.Get("Authorization")
		requests <- got
		switch {
		case strings.HasPrefix(r.URL.Path, "/retry/"):
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"` + apiKey + `"}}`))
		case strings.HasPrefix(r.URL.Path, "/slow/"):
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
		default:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"answer ` + apiKey + `"}}]}`))
		}
	}))
	t.Cleanup(server.Close)

	store := &testAISettingsStore{settings: db.AISettings{Enabled: true, BaseURL: server.URL + "/v1", Model: "configured-model", SealedAPIKey: sealed, TimeoutSeconds: 5}}
	db.Settings = store

	custom := &db.CustomFieldShortcut{AIEnabled: true, Enabled: true, TimeoutSeconds: 5,
		Code: `async function execute(params, context) {
			await context.ai.complete({prompt: params.text, system: "Summarize"})
			return await context.ai.complete({prompt: params.text})
		}`,
	}
	params := map[string]interface{}{"text": "Record text"}
	result, err := RunCustom(context.Background(), custom, nil, params, Env{})
	require.NoError(t, err)
	require.Equal(t, "answer ******", result.Value)
	require.Equal(t, 1, store.reads, "load the model once per run")

	first, second := <-requests, <-requests
	require.Equal(t, "configured-model", first.Model)
	require.Equal(t, "Bearer "+apiKey, first.Auth)
	require.Equal(t, []openai.Message{{Role: "system", Content: "Summarize"}, {Role: "user", Content: "Record text"}}, first.Messages)
	require.Equal(t, []openai.Message{{Role: "user", Content: "Record text"}}, second.Messages)

	custom.Code = `async function execute(params, context) { return await context.ai.complete({prompt: params.text}) }`
	store.settings.BaseURL = server.URL + "/retry"
	_, err = RunCustom(context.Background(), custom, nil, params, Env{})
	var shortcutErr *Error
	require.ErrorAs(t, err, &shortcutErr)
	require.Equal(t, "shortcut::model_unavailable", shortcutErr.Key)
	require.True(t, shortcutErr.Transient)
	require.NotContains(t, err.Error(), apiKey)
	<-requests

	custom.Code = `async function execute(params, context) {
		try { await context.ai.complete({prompt: params.text}) }
		catch (e) { console.log(e.message, e.value); return e.message }
	}`
	result, err = RunCustom(context.Background(), custom, nil, params, Env{})
	require.NoError(t, err)
	require.NotContains(t, result.Value, apiKey)
	require.NotContains(t, strings.Join(result.Logs, "\n"), apiKey)
	<-requests

	custom.Code = `async function execute(params, context) { return await context.ai.complete({prompt: params.text}) }`
	store.settings.BaseURL = server.URL + "/slow"
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = RunCustom(ctx, custom, nil, params, Env{})
	require.ErrorAs(t, err, &shortcutErr)
	require.Equal(t, "shortcut::timeout", shortcutErr.Key)
	require.False(t, shortcutErr.Transient)
	<-requests

	for _, tc := range []struct {
		name, key string
		setup     func()
	}{
		{"global disabled", "shortcut::ai_unavailable", func() { store.settings.Enabled = false }},
		{"unreadable key", "shortcut::ai_key_unavailable", func() { store.settings.Enabled = true; store.settings.SealedAPIKey = "unreadable" }},
		{"no permission", "shortcut::ai_disabled", func() { custom.AIEnabled = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup()
			_, err := RunCustom(context.Background(), custom, nil, params, Env{})
			require.ErrorAs(t, err, &shortcutErr)
			require.Equal(t, tc.key, shortcutErr.Key)
		})
	}

	custom.Code = `function execute() { return "plain script" }`
	reads := store.reads
	result, err = RunCustom(context.Background(), custom, nil, params, Env{})
	require.NoError(t, err)
	require.Equal(t, "plain script", result.Value)
	require.Equal(t, reads, store.reads)
}
