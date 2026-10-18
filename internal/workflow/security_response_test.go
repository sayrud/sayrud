package workflow

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
)

func TestCheckSecurityResponses(t *testing.T) {
	for _, tt := range []struct {
		name     string
		response *ErrorResponse
		code     string
	}{
		{name: "default"},
		{name: "custom", response: &ErrorResponse{Status: 403, Body: `{"success":false,"code":1001}`}},
		{name: "business error", response: &ErrorResponse{Status: 200, Body: `{"code":1001}`}},
		{name: "array", response: &ErrorResponse{Status: 429, Body: `[false,1,null]`}},
		{name: "null", response: &ErrorResponse{Status: 401, Body: `null`}},
		{name: "empty body", response: &ErrorResponse{Status: 401}, code: "security.invalid_response_body"},
		{name: "invalid JSON", response: &ErrorResponse{Status: 401, Body: `{"code":`}, code: "security.invalid_response_body"},
		{name: "informational", response: &ErrorResponse{Status: 100, Body: `{}`}, code: "security.invalid_response_status"},
		{name: "no content", response: &ErrorResponse{Status: 204, Body: `{}`}, code: "security.invalid_response_status"},
		{name: "reset content", response: &ErrorResponse{Status: 205, Body: `{}`}, code: "security.invalid_response_status"},
		{name: "not modified", response: &ErrorResponse{Status: 304, Body: `{}`}, code: "security.invalid_response_status"},
		{name: "out of range", response: &ErrorResponse{Status: 600, Body: `{}`}, code: "security.invalid_response_status"},
	} {
		for _, target := range []string{"auth", "rate_limit"} {
			t.Run(target+"/"+tt.name, func(t *testing.T) {
				api := &APITrigger{Method: "POST", Path: "/test", Auth: Auth{Type: AuthNone}}
				if target == "auth" {
					api.Auth = Auth{Type: AuthAPIKey, ErrorResponse: tt.response}
				} else {
					api.RateLimit = RateLimit{Enabled: true, Limit: 1, Window: 60, ErrorResponse: tt.response}
				}
				c := &checker{spec: &Spec{Trigger: Trigger{Type: TriggerAPI, API: api}}}
				c.checkRoute()
				if tt.code == "" {
					assert.Empty(t, c.issues)
				} else {
					require.Len(t, c.issues, 1)
					assert.Equal(t, Issue{Code: tt.code, Target: target}, c.issues[0])
				}
			})
		}
	}

	t.Run("disabled checks preserve unfinished settings", func(t *testing.T) {
		unfinished := &ErrorResponse{Body: `{"code":`}
		spec := &Spec{Trigger: Trigger{Type: TriggerAPI, API: &APITrigger{
			Method: "POST", Path: "/test", Auth: Auth{Type: AuthNone, ErrorResponse: unfinished},
			RateLimit: RateLimit{ErrorResponse: unfinished},
		}}}
		raw, err := json.Marshal(spec)
		require.NoError(t, err)
		parsed, err := ParseSpec(raw)
		require.NoError(t, err)
		assert.Equal(t, unfinished, parsed.Trigger.API.Auth.ErrorResponse)
		assert.Equal(t, unfinished, parsed.Trigger.API.RateLimit.ErrorResponse)

		c := &checker{spec: parsed}
		c.checkRoute()
		assert.Empty(t, c.issues)
	})
}

func TestWriteSecurityError(t *testing.T) {
	for _, tt := range []struct {
		name     string
		response *ErrorResponse
		status   int
		body     string
	}{
		{name: "default", status: 429, body: `{"error":{"code":"rate_limited","message":"too many requests"}}`},
		{name: "custom object", response: &ErrorResponse{Status: 200, Body: `{"ok":false,"code":1002,"data":null}`}, status: 200, body: `{"ok":false,"code":1002,"data":null}`},
		{name: "custom scalar", response: &ErrorResponse{Status: 403, Body: `false`}, status: 403, body: `false`},
		{name: "custom null", response: &ErrorResponse{Status: 429, Body: `null`}, status: 429, body: `null`},
		{name: "invalid body fallback", response: &ErrorResponse{Status: 200, Body: `{`}, status: 429, body: `{"error":{"code":"rate_limited","message":"too many requests"}}`},
		{name: "invalid status fallback", response: &ErrorResponse{Status: 204, Body: `{}`}, status: 429, body: `{"error":{"code":"rate_limited","message":"too many requests"}}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.Header().Set("Retry-After", "10")
			writeSecurityError(w, tt.response, http.StatusTooManyRequests, "rate_limited", "too many requests")
			assert.Equal(t, tt.status, w.Code)
			assert.JSONEq(t, tt.body, w.Body.String())
			assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
			assert.Equal(t, "10", w.Header().Get("Retry-After"))
		})
	}
}

func TestServeSecurityResponses(t *testing.T) {
	f := newLeadFixture(t)
	workflows := db.NewWorkflowsStore(f.db)
	publish := func() {
		raw, err := json.Marshal(f.spec)
		require.NoError(t, err)
		version, err := workflows.UpdateDraft(t.Context(), f.api.ID, f.api.DraftVersion, db.UpdateWorkflowDraftOptions{Name: f.api.Name, Draft: raw})
		require.NoError(t, err)
		opts, err := PublishOptions(t.Context(), f.db, f.spec, version, 0, time.Now())
		require.NoError(t, err)
		_, err = workflows.Publish(t.Context(), f.api.ID, opts)
		require.NoError(t, err)
		f.api, err = workflows.GetByID(t.Context(), f.api.ID)
		require.NoError(t, err)
	}

	f.spec.Trigger.API.Auth = Auth{Type: AuthAPIKey}
	publish()
	w, _ := f.call(t, `{}`, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.JSONEq(t, `{"error":{"code":"unauthorized","message":"a valid API key is required"}}`, w.Body.String())

	f.spec.Trigger.API.Auth.ErrorResponse = &ErrorResponse{Status: 403, Body: `{"success":false,"code":"INVALID_KEY"}`}
	publish()
	for _, headers := range []map[string]string{nil, {"X-API-Key": "invalid"}, {"Authorization": "Bearer invalid"}} {
		w, _ := f.call(t, `{}`, headers)
		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.JSONEq(t, f.spec.Trigger.API.Auth.ErrorResponse.Body, w.Body.String())
	}
	assert.Zero(t, f.count(t, &db.WorkflowRun{}, "workflow_id = ?", f.api.ID))

	t.Run("rate limit", func(t *testing.T) {
		address := os.Getenv("REDIS_TEST_ADDRESS")
		if address == "" {
			t.Skip("REDIS_TEST_ADDRESS is not set")
		}
		rdb := redis.NewClient(&redis.Options{Addr: address})
		t.Cleanup(func() { _ = rdb.Close() })
		f.engine.redis = rdb
		f.spec.Trigger.API.Auth = Auth{Type: AuthNone}
		f.spec.Trigger.API.RateLimit = RateLimit{Enabled: true, Limit: 1, Window: 60}

		for _, response := range []*ErrorResponse{nil, {Status: 200, Body: `{"success":false,"code":"TOO_FAST"}`}} {
			f.spec.Trigger.API.RateLimit.ErrorResponse = response
			publish()
			// Exhaust the counter before exercising the HTTP rejection.
			for i := 0; i < 2; i++ {
				allowed, _ := f.engine.AllowRate(t.Context(), f.api.ID, "127.0.0.1", f.spec.Trigger.API.RateLimit)
				if !allowed {
					break
				}
			}
			w, _ := f.call(t, `{}`, nil)
			if response == nil {
				assert.Equal(t, http.StatusTooManyRequests, w.Code)
				assert.JSONEq(t, `{"error":{"code":"rate_limited","message":"too many requests"}}`, w.Body.String())
			} else {
				assert.Equal(t, response.Status, w.Code)
				assert.JSONEq(t, response.Body, w.Body.String())
			}
			retry, err := strconv.Atoi(w.Header().Get("Retry-After"))
			require.NoError(t, err)
			assert.Greater(t, retry, 0)
			assert.LessOrEqual(t, retry, 60)
			assert.Zero(t, f.count(t, &db.WorkflowRun{}, "workflow_id = ?", f.api.ID))
		}
	})
}
