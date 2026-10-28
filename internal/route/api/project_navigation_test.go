package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/collab"
	appcontext "github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
)

type navigationProjects struct {
	db.ProjectsStore
	called bool
	err    error
}

func (s *navigationProjects) MoveNavigation(_ context.Context, projectID int64, uid, targetUID string, after bool) ([]db.NavigationPosition, error) {
	s.called = projectID == 7 && uid == "tblOne" && targetUID == "wflTwo" && after

	return []db.NavigationPosition{{UID: targetUID, Position: 0}, {UID: uid, Position: 1}}, s.err
}

func TestMoveProjectNavigationAPI(t *testing.T) {
	previous := db.Projects
	t.Cleanup(func() { db.Projects = previous })

	for _, tc := range []struct {
		name   string
		role   db.ProjectRole
		body   string
		err    error
		status int
		called bool
	}{
		{"editor", db.ProjectRoleEditor, `{"uid":"tblOne","targetUID":"wflTwo","after":true}`, nil, 200, true},
		{"viewer", db.ProjectRoleViewer, `{"uid":"tblOne","targetUID":"wflTwo","after":true}`, nil, 403, false},
		{"missing source", db.ProjectRoleEditor, `{"targetUID":"wflTwo"}`, nil, 400, false},
		{"missing target", db.ProjectRoleEditor, `{"uid":"tblOne"}`, nil, 400, false},
		{"item outside project", db.ProjectRoleEditor, `{"uid":"tblOne","targetUID":"wflTwo","after":true}`, db.ErrNavigationItemNotFound, 400, true},
		{"deleted project", db.ProjectRoleEditor, `{"uid":"tblOne","targetUID":"wflTwo","after":true}`, db.ErrProjectNotFound, 404, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &navigationProjects{err: tc.err}
			db.Projects = store

			router := flamego.New()
			router.Use(i18n.Middleware(), appcontext.Contexter(nil))
			router.Map(&db.Project{Model: dbutil.Model{ID: 7}, UID: "project"}, tc.role, collab.NewHub(nil))
			router.Put("/", Project.RequireRole(db.ProjectRoleEditor), form.Bind(form.MoveProjectNavigation{}), Project.MoveNavigation)

			request := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", "application/json")

			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, tc.status, response.Code, response.Body.String())
			require.Equal(t, tc.called, store.called)
			if tc.status == http.StatusOK {
				require.Contains(t, response.Body.String(), `"data":[{"uid":"wflTwo","position":0},{"uid":"tblOne","position":1}]`)
			}
		})
	}
}
