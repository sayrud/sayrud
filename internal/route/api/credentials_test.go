package api

import (
	stdcontext "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/sso"
	"github.com/wuhan005/sayrud/internal/workflow"
)

type projectCredentialsStore struct {
	db.CredentialsStore
	items map[string]*db.Credential
}

func (s *projectCredentialsStore) GetByUID(_ stdcontext.Context, uid string) (*db.Credential, error) {
	if c := s.items[uid]; c != nil {
		copy := *c
		return &copy, nil
	}
	return nil, db.ErrCredentialNotFound
}

func (s *projectCredentialsStore) List(_ stdcontext.Context, projectID int64, credType string) ([]*db.Credential, error) {
	var list []*db.Credential
	for _, c := range s.items {
		if c.ProjectID == projectID && (credType == "" || c.Type == credType) {
			list = append(list, c)
		}
	}
	return list, nil
}

func (s *projectCredentialsStore) Update(_ stdcontext.Context, id int64, opts db.UpdateCredentialOptions) error {
	for _, c := range s.items {
		if c.ID == id {
			c.Name, c.Data, c.Sealed = opts.Name, datatypes.JSON(opts.Data), opts.Sealed
			return nil
		}
	}
	return db.ErrCredentialNotFound
}

func TestProjectCredentialDetailsAndEditing(t *testing.T) {
	previous := db.Credentials
	t.Cleanup(func() { db.Credentials = previous; sso.SetKey(nil) })
	sso.SetKey(make([]byte, 32))
	sealed, err := workflow.SealSecrets(map[string]string{"password": "saved-secret"})
	require.NoError(t, err)
	store := &projectCredentialsStore{items: map[string]*db.Credential{
		"own":   {Model: dbutil.Model{ID: 1}, UID: "own", ProjectID: 1, Type: "smtp", Name: "Mailer", Data: datatypes.JSON(`{"host":"smtp.example.com","fromAddress":"bot@example.com"}`), Sealed: sealed},
		"other": {Model: dbutil.Model{ID: 2}, UID: "other", ProjectID: 2, Type: "smtp", Name: "Private", Data: datatypes.JSON(`{}`)},
	}}
	db.Credentials = store
	request := func(role db.ProjectRole, method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		router := flamego.New()
		router.Use(i18n.Middleware(), context.Contexter(nil))
		router.Map(&db.Project{Model: dbutil.Model{ID: 1}}, &db.User{Model: dbutil.Model{ID: 7}}, role)
		router.Group("/credentials", func() {
			router.Get("", Credential.List)
			router.Group("/{credentialUID}", func() {
				router.Get("", Credential.Get)
				router.Put("", Project.RequireRole(db.ProjectRoleManager), form.Bind(form.SaveCredential{}), Credential.Update)
			}, Credential.Credentialer)
		}, Project.RequireRole(db.ProjectRoleEditor))
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		require.NotContains(t, response.Body.String(), "saved-secret")
		require.NotContains(t, response.Body.String(), sealed)
		return response
	}

	for _, role := range []db.ProjectRole{db.ProjectRoleViewer, db.ProjectRoleEditor, db.ProjectRoleManager, db.ProjectRoleOwner} {
		t.Run(string(role), func(t *testing.T) {
			readStatus := http.StatusOK
			if role == db.ProjectRoleViewer {
				readStatus = http.StatusForbidden
			}
			list := request(role, http.MethodGet, "/credentials", "")
			require.Equal(t, readStatus, list.Code)
			details := request(role, http.MethodGet, "/credentials/own", "")
			require.Equal(t, readStatus, details.Code)
			if readStatus == http.StatusOK {
				var body struct{ Data dto.Credential }
				require.NoError(t, json.Unmarshal(details.Body.Bytes(), &body))
				require.Equal(t, []string{"password"}, body.Data.Secrets)
				require.Equal(t, "smtp.example.com", body.Data.Data["host"])
				require.NotContains(t, list.Body.String(), "Private")
				require.Equal(t, http.StatusNotFound, request(role, http.MethodGet, "/credentials/other", "").Code)
				require.Equal(t, http.StatusNotFound, request(role, http.MethodGet, "/credentials/missing", "").Code)
			}

			updateStatus := http.StatusOK
			if !role.AtLeast(db.ProjectRoleManager) {
				updateStatus = http.StatusForbidden
			}
			updated := request(role, http.MethodPut, "/credentials/own", `{"type":"smtp","name":"Updated mailer","data":{"host":"smtp.example.com","fromAddress":"new@example.com"},"secrets":{"password":""}}`)
			require.Equal(t, updateStatus, updated.Code, updated.Body.String())
			secrets, err := workflow.OpenSecrets(store.items["own"].Sealed)
			require.NoError(t, err)
			require.Equal(t, "saved-secret", secrets["password"], "blank secret inputs preserve the saved value")
		})
	}

	response := request(db.ProjectRoleManager, http.MethodPut, "/credentials/other", `{"name":"Cross project","data":{}}`)
	require.Equal(t, http.StatusNotFound, response.Code)
	require.Equal(t, "Private", store.items["other"].Name)
	response = request(db.ProjectRoleManager, http.MethodPut, "/credentials/own", `{"name":"Mailer","data":{"host":"smtp.example.com","fromAddress":"bot@example.com"},"clearSecrets":["password"]}`)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	secrets, err := workflow.OpenSecrets(store.items["own"].Sealed)
	require.NoError(t, err)
	require.Empty(t, secrets)
}
