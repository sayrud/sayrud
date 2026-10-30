package route

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/route/api"
	"github.com/wuhan005/sayrud/internal/setup"
)

func newSetupTestRouter(t *testing.T) (http.Handler, *setup.Token) {
	t.Helper()
	token, err := setup.NewToken()
	require.NoError(t, err)
	wizard := setup.New(setup.Options{Token: token, Stage: setup.StageDatabase})
	return NewSetup(wizard), token
}

func setupRequest(router http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		request.Header.Set(api.SetupTokenHeader, token)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestSetupStatus(t *testing.T) {
	router, _ := newSetupTestRouter(t)
	response := setupRequest(router, http.MethodGet, "/_/setup/status", "", "")
	require.Equal(t, http.StatusOK, response.Code)

	var body struct {
		Data dto.SetupStatus `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, dto.SetupStatus{Stage: "database", PasswordMinLength: 8}, body.Data)

	installed := setupRequest(New(Options{}), http.MethodGet, "/_/setup/status", "", "")
	require.Equal(t, http.StatusOK, installed.Code)
	require.JSONEq(t, `{"msg":"success","data":{"installed":true}}`, installed.Body.String())
}

func TestSetupToken(t *testing.T) {
	router, token := newSetupTestRouter(t)

	assert.Equal(t, http.StatusUnauthorized, setupRequest(router, http.MethodPost, "/_/setup/verify", "", "").Code)
	assert.Equal(t, http.StatusUnauthorized, setupRequest(router, http.MethodPost, "/_/setup/verify", "WRONG-TOKEN", "").Code)
	assert.Equal(t, http.StatusNoContent, setupRequest(router, http.MethodPost, "/_/setup/verify", token.String(), "").Code)
	assert.Equal(t, http.StatusNoContent, setupRequest(router, http.MethodPost, "/_/setup/verify", strings.ToLower(token.String()), "").Code)

	admin := `{"email":"admin@example.com","userName":"Admin","password":"password123"}`
	assert.Equal(t, http.StatusUnauthorized, setupRequest(router, http.MethodPost, "/_/setup/admin", "", admin).Code, "the token is checked before the body")
	assert.Equal(t, http.StatusConflict, setupRequest(router, http.MethodPost, "/_/setup/admin", token.String(), admin).Code, "the admin step requires the database")

	database := `{"postgres":{"host":"127.0.0.1","port":5432,"database":"sayrud","username":"postgres","password":"","sslMode":"disable"},"redis":{"address":"redis","username":"","password":""}}`
	assert.Equal(t, http.StatusUnauthorized, setupRequest(router, http.MethodPost, "/_/setup/database/test", "", database).Code)
	assert.Equal(t, http.StatusBadRequest, setupRequest(router, http.MethodPost, "/_/setup/database/test", token.String(), database).Code, "the Redis address requires a port")
	invalidSSL := strings.Replace(database, `"disable"`, `"prefer"`, 1)
	assert.Equal(t, http.StatusBadRequest, setupRequest(router, http.MethodPost, "/_/setup/database", token.String(), invalidSSL).Code)

	for range 10 {
		setupRequest(router, http.MethodPost, "/_/setup/verify", "WRONG-TOKEN", "")
	}
	assert.Equal(t, http.StatusTooManyRequests, setupRequest(router, http.MethodPost, "/_/setup/verify", token.String(), "").Code)
}

func TestSetupUnavailable(t *testing.T) {
	router, _ := newSetupTestRouter(t)

	for _, path := range []string{"/_/site", "/_/auth/profile", "/_/setup/unknown", "/api/project/hook"} {
		response := setupRequest(router, http.MethodGet, path, "", "")
		assert.Equal(t, http.StatusServiceUnavailable, response.Code, path)
		assert.Equal(t, "required", response.Header().Get(api.SetupRequiredHeader), path)
	}

	for _, path := range []string{"/", "/login", "/admin/users", "/base/project/table"} {
		response := setupRequest(router, http.MethodGet, path, "", "")
		assert.Equal(t, http.StatusFound, response.Code, path)
		assert.Equal(t, SetupPath, response.Header().Get("Location"), path)
	}

	page := setupRequest(router, http.MethodGet, "/setup?token=ABCD", "", "")
	require.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Header().Get("Content-Type"), "text/html")
	assert.Equal(t, "no-referrer", page.Header().Get("Referrer-Policy"), "the token in the URL must not leak in the Referer header")

	for _, path := range []string{"/-/healthz", "/-/readyz"} {
		assert.Equal(t, http.StatusOK, setupRequest(router, http.MethodGet, path, "", "").Code, path)
	}
}
