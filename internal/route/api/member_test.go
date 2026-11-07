package api

import (
	stdcontext "context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flamego/flamego"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
)

type resolvingMembersStore struct{ db.ProjectMembersStore }

func (resolvingMembersStore) UserIDs(stdcontext.Context, int64) ([]int64, error) {
	return []int64{1, 4}, nil
}

func (resolvingMembersStore) FormerUserIDs(stdcontext.Context, int64) ([]int64, error) {
	return []int64{2, 5}, nil
}

type resolvingRecordsStore struct {
	db.SLRecordsStore
	t *testing.T
}

func (s resolvingRecordsStore) ReferencedMemberIDs(_ stdcontext.Context, tableID int64, users, workflows []int64) ([]int64, []int64, error) {
	require.Equal(s.t, int64(20), tableID)
	return lo.Intersect(users, []int64{1, 2, 3}), lo.Intersect(workflows, []int64{7}), nil
}

func (resolvingRecordsStore) WriterUserIDs(_ stdcontext.Context, _ int64, users []int64) ([]int64, error) {
	return lo.Intersect(users, []int64{3, 6}), nil
}

type resolvingUsersStore struct{ db.UsersStore }

func (resolvingUsersStore) ListByIDs(_ stdcontext.Context, ids []int64) ([]*db.User, error) {
	return lo.Map(ids, func(id int64, _ int) *db.User {
		return &db.User{Model: dbutil.Model{ID: id}, Email: "private@example.com", EmailMd5: "private-hash", UserName: "Member"}
	}), nil
}

type resolvingWorkflowsStore struct{ db.WorkflowsStore }

func (resolvingWorkflowsStore) NamesByIDs(_ stdcontext.Context, _ int64, ids []int64) (map[int64]string, error) {
	names := make(map[int64]string)
	for _, id := range ids {
		names[id] = "Workflow"
	}
	return names, nil
}

func TestResolveMembersScope(t *testing.T) {
	previousMembers, previousRecords, previousUsers, previousWorkflows := db.ProjectMembers, db.SLRecords, db.Users, db.Workflows
	t.Cleanup(func() {
		db.ProjectMembers, db.SLRecords, db.Users, db.Workflows = previousMembers, previousRecords, previousUsers, previousWorkflows
	})
	db.ProjectMembers, db.SLRecords, db.Users, db.Workflows = resolvingMembersStore{}, resolvingRecordsStore{t: t}, resolvingUsersStore{}, resolvingWorkflowsStore{}
	router := flamego.New()
	router.Use(i18n.Middleware(), context.Contexter(nil))
	router.Map(&db.Project{Model: dbutil.Model{ID: 10}}, &db.SLTable{Model: dbutil.Model{ID: 20}})
	router.Post("/public", form.Bind(form.ResolveMembers{}), Share.ResolveMembers)
	router.Post("/private", form.Bind(form.ResolveMembers{}), Project.ResolveMembers)
	for _, path := range []string{"/public", "/private"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest("POST", path, strings.NewReader(`{"userIds":[1,2,3,4,5,6],"workflowIds":[7,8]}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, 200, response.Code, response.Body.String())
			var envelope struct{ Data dto.ResolvedMembers }
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
			ids := lo.Map(envelope.Data.Users, func(u *dto.ResolvedUser, _ int) int64 { return u.User.ID })
			workflows := lo.Map(envelope.Data.Workflows, func(w *dto.WorkflowName, _ int) int64 { return w.ID })
			if path == "/public" {
				require.ElementsMatch(t, []int64{1, 2, 3}, ids)
				require.Equal(t, []int64{7}, workflows)
				for _, u := range envelope.Data.Users {
					require.Empty(t, u.User.Email)
					require.Empty(t, u.User.EmailMd5)
					require.Equal(t, u.User.ID == 2, u.Left)
				}
			} else {
				require.ElementsMatch(t, []int64{1, 2, 3, 4, 5, 6}, ids)
				require.ElementsMatch(t, []int64{7, 8}, workflows)
				require.NotEmpty(t, envelope.Data.Users[0].User.Email)
			}
		})
	}
	request := httptest.NewRequest("POST", "/public", strings.NewReader(`{"userIds":[`+strings.Repeat("1,", maxResolvedMembers)+`1]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, 400, response.Code, "the ID limit is checked before filtering references")
}
