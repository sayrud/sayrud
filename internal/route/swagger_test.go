package route

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
)

type swaggerSessionsStore struct {
	db.UserSessionsStore
}

func (swaggerSessionsStore) GetByToken(context.Context, string) (*db.UserSession, error) {
	return &db.UserSession{UserID: 1}, nil
}

type swaggerUsersStore struct {
	db.UsersStore
	admin bool
}

func (s swaggerUsersStore) GetByID(context.Context, int64) (*db.User, error) {
	return &db.User{IsAdmin: s.admin}, nil
}

func TestAdminSwagger(t *testing.T) {
	previousSessions, previousUsers := db.UserSessions, db.Users
	t.Cleanup(func() { db.UserSessions, db.Users = previousSessions, previousUsers })
	db.UserSessions = swaggerSessionsStore{}
	router := New(Options{})

	paths := []struct {
		path        string
		contentType string
	}{
		{"index.html", "text/html"},
		{"doc.json", "application/json"},
		{"swagger-ui.css", "text/css"},
		{"swagger-ui-bundle.js", "javascript"},
		{"swagger-ui-standalone-preset.js", "javascript"},
	}
	for _, role := range []struct {
		name   string
		signed bool
		admin  bool
		status int
	}{
		{"guest", false, false, http.StatusUnauthorized},
		{"member", true, false, http.StatusForbidden},
		{"admin", true, true, http.StatusOK},
	} {
		t.Run(role.name, func(t *testing.T) {
			db.Users = swaggerUsersStore{admin: role.admin}
			for _, resource := range paths {
				t.Run(resource.path, func(t *testing.T) {
					request := httptest.NewRequest(http.MethodGet, "/_/admin/swagger/"+resource.path, nil)
					if role.signed {
						request.AddCookie(&http.Cookie{Name: "sayrud_session", Value: "test-session"})
					}
					response := httptest.NewRecorder()
					router.ServeHTTP(response, request)
					require.Equal(t, role.status, response.Code)
					if role.status != http.StatusOK {
						return
					}
					require.Contains(t, response.Header().Get("Content-Type"), resource.contentType)
					require.NotEmpty(t, response.Body.Bytes())
					if resource.path == "doc.json" {
						var spec struct {
							Swagger  string                     `json:"swagger"`
							BasePath string                     `json:"basePath"`
							Paths    map[string]json.RawMessage `json:"paths"`
						}
						require.NoError(t, json.Unmarshal(response.Body.Bytes(), &spec))
						require.Equal(t, "2.0", spec.Swagger)
						require.Equal(t, "/_", spec.BasePath)
						require.Contains(t, spec.Paths, "/auth/profile")
						require.Contains(t, spec.Paths, "/admin/overview")
					}
				})
			}
		})
	}
}
