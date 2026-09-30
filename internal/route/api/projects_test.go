package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
)

func TestRequireRole(t *testing.T) {
	for _, tc := range []struct {
		role db.ProjectRole
		min  db.ProjectRole
		want int
	}{
		{db.ProjectRoleViewer, db.ProjectRoleEditor, http.StatusForbidden},
		{db.ProjectRoleEditor, db.ProjectRoleEditor, http.StatusNoContent},
		{db.ProjectRoleEditor, db.ProjectRoleManager, http.StatusForbidden},
		{db.ProjectRoleManager, db.ProjectRoleManager, http.StatusNoContent},
		{db.ProjectRoleManager, db.ProjectRoleOwner, http.StatusForbidden},
		{db.ProjectRoleOwner, db.ProjectRoleOwner, http.StatusNoContent},
	} {
		t.Run(string(tc.role)+" requires "+string(tc.min), func(t *testing.T) {
			f := flamego.New()
			f.Use(context.Contexter(nil), func(c flamego.Context) { c.Map(tc.role) })
			f.Get("/", Project.RequireRole(tc.min), func(ctx context.Context) error {
				return ctx.Status(http.StatusNoContent)
			})

			response := httptest.NewRecorder()
			f.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
			require.Equal(t, tc.want, response.Code)
		})
	}
}
