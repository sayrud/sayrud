package api

import (
	stdcontext "context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/i18n"
)

type projecterMembersStore struct {
	db.ProjectMembersStore
	roles map[int64]db.ProjectRole
}

func (s *projecterMembersStore) GetRole(_ stdcontext.Context, _, userID int64) (db.ProjectRole, error) {
	if role, ok := s.roles[userID]; ok {
		return role, nil
	}
	return "", db.ErrProjectMemberNotFound
}

func TestProjecter(t *testing.T) {
	previousProjects, previousMembers := db.Projects, db.ProjectMembers
	t.Cleanup(func() { db.Projects, db.ProjectMembers = previousProjects, previousMembers })
	db.Projects = &shareProjectsStore{project: &db.Project{Model: dbutil.Model{ID: 1}, UID: "base", OwnerUserID: 1}}
	db.ProjectMembers = &projecterMembersStore{roles: map[int64]db.ProjectRole{2: db.ProjectRoleViewer, 3: db.ProjectRoleViewer}}

	for _, tc := range []struct {
		name     string
		user     *db.User
		want     int
		role     db.ProjectRole
		viaAdmin bool
	}{
		{name: "owner", user: &db.User{Model: dbutil.Model{ID: 1}, IsAdmin: true}, want: http.StatusNoContent, role: db.ProjectRoleOwner},
		{name: "collaborator", user: &db.User{Model: dbutil.Model{ID: 2}}, want: http.StatusNoContent, role: db.ProjectRoleViewer},
		{name: "admin collaborator", user: &db.User{Model: dbutil.Model{ID: 3}, IsAdmin: true}, want: http.StatusNoContent, role: db.ProjectRoleManager, viaAdmin: true},
		{name: "admin", user: &db.User{Model: dbutil.Model{ID: 4}, IsAdmin: true}, want: http.StatusNoContent, role: db.ProjectRoleManager, viaAdmin: true},
		{name: "stranger", user: &db.User{Model: dbutil.Model{ID: 5}}, want: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := flamego.New()
			f.Use(i18n.Middleware(), context.Contexter(nil), func(c flamego.Context) { c.Map(tc.user) })
			f.Get("/{projectUID}", Project.Projecter, func(ctx context.Context, role db.ProjectRole, access projectAccess) error {
				require.Equal(t, tc.role, role)
				require.Equal(t, tc.viaAdmin, access.ViaAdmin)
				return ctx.Status(http.StatusNoContent)
			})

			response := httptest.NewRecorder()
			f.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/base", nil))
			require.Equal(t, tc.want, response.Code)
		})
	}
}

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
			f.Use(i18n.Middleware(), context.Contexter(nil), func(c flamego.Context) { c.Map(tc.role) })
			f.Get("/", Project.RequireRole(tc.min), func(ctx context.Context) error {
				return ctx.Status(http.StatusNoContent)
			})

			response := httptest.NewRecorder()
			f.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
			require.Equal(t, tc.want, response.Code)
		})
	}
}
