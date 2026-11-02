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
	db.ProjectMembers = &projecterMembersStore{roles: map[int64]db.ProjectRole{
		2: db.ProjectRoleViewer, 3: db.ProjectRoleViewer,
		// Bots of the viewer 2, the owner 1 and the stranger 5.
		6: db.ProjectRoleEditor, 7: db.ProjectRoleEditor, 8: db.ProjectRoleManager,
	}}
	bot := func(id, owner int64) *db.User {
		return &db.User{Model: dbutil.Model{ID: id}, Kind: db.UserKindBot, OwnerID: owner}
	}

	unrestricted := &db.AccessToken{ProjectIDs: []int64{}}
	for _, tc := range []struct {
		name     string
		user     *db.User
		token    *db.AccessToken
		want     int
		role     db.ProjectRole
		viaAdmin bool
	}{
		{name: "owner", user: &db.User{Model: dbutil.Model{ID: 1}, IsAdmin: true}, want: http.StatusNoContent, role: db.ProjectRoleOwner},
		{name: "collaborator", user: &db.User{Model: dbutil.Model{ID: 2}}, want: http.StatusNoContent, role: db.ProjectRoleViewer},
		{name: "admin collaborator", user: &db.User{Model: dbutil.Model{ID: 3}, IsAdmin: true}, want: http.StatusNoContent, role: db.ProjectRoleManager, viaAdmin: true},
		{name: "admin", user: &db.User{Model: dbutil.Model{ID: 4}, IsAdmin: true}, want: http.StatusNoContent, role: db.ProjectRoleManager, viaAdmin: true},
		{name: "stranger", user: &db.User{Model: dbutil.Model{ID: 5}}, want: http.StatusForbidden},
		{name: "owner token", user: &db.User{Model: dbutil.Model{ID: 1}, IsAdmin: true}, token: unrestricted, want: http.StatusNoContent, role: db.ProjectRoleOwner},
		{name: "admin collaborator token", user: &db.User{Model: dbutil.Model{ID: 3}, IsAdmin: true}, token: unrestricted, want: http.StatusNoContent, role: db.ProjectRoleViewer},
		{name: "admin token", user: &db.User{Model: dbutil.Model{ID: 4}, IsAdmin: true}, token: unrestricted, want: http.StatusForbidden},
		{name: "allowed project", user: &db.User{Model: dbutil.Model{ID: 2}}, token: &db.AccessToken{ProjectIDs: []int64{1}}, want: http.StatusNoContent, role: db.ProjectRoleViewer},
		{name: "other project", user: &db.User{Model: dbutil.Model{ID: 2}}, token: &db.AccessToken{ProjectIDs: []int64{9}}, want: http.StatusForbidden},
		{name: "bot capped by its viewer owner", user: bot(6, 2), token: unrestricted, want: http.StatusNoContent, role: db.ProjectRoleViewer},
		{name: "bot of the project owner", user: bot(8, 1), token: unrestricted, want: http.StatusNoContent, role: db.ProjectRoleManager},
		{name: "bot whose owner left", user: bot(7, 5), token: unrestricted, want: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := flamego.New()
			f.Use(i18n.Middleware(), context.Contexter(nil), func(c flamego.Context) { c.Map(tc.user, tc.token) })
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

func TestVisibleCandidate(t *testing.T) {
	alice := &db.User{Model: dbutil.Model{ID: 1}}
	bob := &db.User{Model: dbutil.Model{ID: 2}}
	require.True(t, visibleCandidate(alice, bob))
	require.True(t, visibleCandidate(alice, &db.User{Model: dbutil.Model{ID: 4}, Kind: db.UserKindBot, OwnerID: 1}))
	require.False(t, visibleCandidate(alice, &db.User{Model: dbutil.Model{ID: 5}, Kind: db.UserKindBot, OwnerID: 2}))
}

func TestLowerRole(t *testing.T) {
	require.Equal(t, db.ProjectRoleViewer, db.LowerRole(db.ProjectRoleManager, db.ProjectRoleViewer))
	require.Equal(t, db.ProjectRoleEditor, db.LowerRole(db.ProjectRoleEditor, db.ProjectRoleOwner))
	require.Empty(t, db.LowerRole(db.ProjectRoleEditor, ""))
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
