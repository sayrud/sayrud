package api

import (
	stdcontext "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/i18n"
)

func TestSelfOperationError(t *testing.T) {
	require.NoError(t, selfOperationError(1, 2, "admin::cannot_disable_self"))

	var e *i18n.Error
	require.ErrorAs(t, selfOperationError(1, 1, "admin::cannot_disable_self"), &e)
	require.Equal(t, "admin::cannot_disable_self", e.Key)
}

type adminListProjectsStore struct {
	db.ProjectsStore
	projects []*db.Project
}

func (s adminListProjectsStore) ListAll(stdcontext.Context, db.ListAllProjectsOptions) ([]*db.Project, int64, error) {
	return s.projects, int64(len(s.projects)), nil
}

type adminListProjectUsersStore struct {
	db.UsersStore
}

func (adminListProjectUsersStore) ListByIDs(stdcontext.Context, []int64) ([]*db.User, error) {
	return nil, nil
}

type adminListProjectMembersStore struct {
	db.ProjectMembersStore
}

func (adminListProjectMembersStore) CountByProjectIDs(stdcontext.Context, []int64) (map[int64]int64, error) {
	return nil, nil
}

type adminListProjectTablesStore struct {
	db.SLTablesStore
}

func (adminListProjectTablesStore) CountByProjectIDs(stdcontext.Context, []int64) (map[int64]int64, error) {
	return nil, nil
}

func TestAdminListProjectsAppearance(t *testing.T) {
	previousProjects, previousUsers := db.Projects, db.Users
	previousMembers, previousTables := db.ProjectMembers, db.SLTables
	t.Cleanup(func() {
		db.Projects, db.Users = previousProjects, previousUsers
		db.ProjectMembers, db.SLTables = previousMembers, previousTables
	})

	db.Projects = adminListProjectsStore{projects: []*db.Project{
		{UID: "custom", Name: "Custom project", Icon: "calendar", Color: "teal"},
		{UID: "default", Name: "Default project"},
	}}
	db.Users = adminListProjectUsersStore{}
	db.ProjectMembers = adminListProjectMembersStore{}
	db.SLTables = adminListProjectTablesStore{}

	f := flamego.New()
	f.Use(i18n.Middleware(), context.Contexter(nil))
	f.Get("/", Admin.ListProjects)

	response := httptest.NewRecorder()
	f.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, response.Code)

	var body struct {
		Data dto.ListAdminProjectsResp `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.EqualValues(t, 2, body.Data.Total)
	require.Len(t, body.Data.Projects, 2)

	require.Equal(t, "calendar", body.Data.Projects[0].Icon)
	require.Equal(t, "teal", body.Data.Projects[0].Color)
	require.Empty(t, body.Data.Projects[1].Icon)
	require.Empty(t, body.Data.Projects[1].Color)
}
