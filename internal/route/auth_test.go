package route

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIRequiresSignIn(t *testing.T) {
	router := New(Options{})
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/_/auth/profile"},
		{http.MethodPut, "/_/auth/profile"},
		{http.MethodPut, "/_/auth/password"},
		{http.MethodPost, "/_/auth/avatar"},
		{http.MethodDelete, "/_/auth/avatar"},
		{http.MethodGet, "/_/projects"},
		{http.MethodPost, "/_/projects"},
		{http.MethodPut, "/_/projects/prjabc/navigation/move"},
		{http.MethodGet, "/_/projects/prjabc/tables"},
		{http.MethodGet, "/_/projects/prjabc/members"},
		{http.MethodGet, "/_/projects/prjabc/credentials"},
		{http.MethodGet, "/_/projects/prjabc/credentials/crdabc"},
		{http.MethodPost, "/_/projects/prjabc/credentials"},
		{http.MethodPut, "/_/projects/prjabc/credentials/crdabc"},
		{http.MethodDelete, "/_/projects/prjabc/credentials/crdabc"},
		{http.MethodGet, "/_/projects/prjabc/ws"},
		{http.MethodGet, "/_/projects/prjabc/tables/tblabc/share"},
		{http.MethodPut, "/_/projects/prjabc/tables/tblabc/share"},
		{http.MethodGet, "/_/projects/prjabc/attachments/filabc"},
		{http.MethodPost, "/_/projects/prjabc/tables/tblabc/fields/fldabc/attachments"},
		{http.MethodGet, "/_/auth/identities"},
		{http.MethodDelete, "/_/auth/identities/1"},
		{http.MethodGet, "/_/admin/auth-providers"},
		{http.MethodPost, "/_/admin/auth-providers"},
		{http.MethodPut, "/_/admin/auth-providers/positions"},
		{http.MethodPost, "/_/admin/auth-providers/test"},
		{http.MethodPut, "/_/admin/auth-providers/1"},
		{http.MethodDelete, "/_/admin/auth-providers/1"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
			require.Equal(t, http.StatusUnauthorized, response.Code)
		})
	}
}

func TestAccessTokensRejectedOnSessionRoutes(t *testing.T) {
	router := New(Options{})
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/_/auth/profile"},
		{http.MethodPut, "/_/auth/password"},
		{http.MethodPost, "/_/auth/avatar"},
		{http.MethodDelete, "/_/auth/avatar"},
		{http.MethodGet, "/_/auth/sessions"},
		{http.MethodDelete, "/_/auth/sessions"},
		{http.MethodDelete, "/_/auth/sessions/1"},
		{http.MethodGet, "/_/auth/settings"},
		{http.MethodDelete, "/_/auth/account"},
		{http.MethodGet, "/_/auth/identities"},
		{http.MethodDelete, "/_/auth/identities/1"},
		{http.MethodGet, "/_/auth/tokens"},
		{http.MethodPost, "/_/auth/tokens"},
		{http.MethodDelete, "/_/auth/tokens/1"},
		{http.MethodGet, "/_/auth/bots"},
		{http.MethodPost, "/_/auth/bots"},
		{http.MethodPost, "/_/auth/bots/1/token"},
		{http.MethodPut, "/_/auth/bots/1/access"},
		{http.MethodDelete, "/_/auth/bots/1"},
		{http.MethodGet, "/_/admin/overview"},
		{http.MethodGet, "/_/admin/users"},
		{http.MethodPost, "/_/admin/bots"},
		{http.MethodPost, "/_/admin/bots/1/token"},
		{http.MethodGet, "/_/admin/swagger/index.html"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, nil)
			r.Header.Set("Authorization", "Bearer sat_anything")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, r)
			require.Equal(t, http.StatusForbidden, response.Code)
		})
	}
}
