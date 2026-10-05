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
		{http.MethodGet, "/_/projects/prjabc/tables"},
		{http.MethodGet, "/_/projects/prjabc/members"},
		{http.MethodGet, "/_/projects/prjabc/ws"},
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
