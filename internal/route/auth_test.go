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
		{http.MethodGet, "/_/projects"},
		{http.MethodPost, "/_/projects"},
		{http.MethodGet, "/_/projects/prjabc/tables"},
		{http.MethodGet, "/_/projects/prjabc/members"},
		{http.MethodGet, "/_/projects/prjabc/ws"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
			require.Equal(t, http.StatusUnauthorized, response.Code)
		})
	}
}
