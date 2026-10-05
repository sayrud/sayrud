package api

import (
	stdcontext "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/conf"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/redis"
	"github.com/wuhan005/sayrud/internal/sso"
)

type shareTablesStore struct {
	db.SLTablesStore
	tables []*db.SLTable
}

func (s *shareTablesStore) GetByShareToken(_ stdcontext.Context, token string) (*db.SLTable, error) {
	for _, table := range s.tables {
		if token != "" && table.ShareToken == token && table.ShareEnabled && !table.DeletedAt.Valid {
			return table, nil
		}
	}
	return nil, db.ErrSLTableNotFound
}

func (s *shareTablesStore) GetByUID(_ stdcontext.Context, uid string) (*db.SLTable, error) {
	for _, table := range s.tables {
		if table.UID == uid && !table.DeletedAt.Valid {
			return table, nil
		}
	}
	return nil, db.ErrSLTableNotFound
}

func (s *shareTablesStore) Query(_ stdcontext.Context, opts db.QuerySLTableOptions) ([]*db.SLTable, int64, error) {
	var tables []*db.SLTable
	for _, table := range s.tables {
		if table.ProjectID == opts.ProjectID && !table.DeletedAt.Valid {
			tables = append(tables, table)
		}
	}
	total := int64(len(tables))
	limit, offset := opts.LimitOffset()
	if offset >= len(tables) {
		return nil, total, nil
	}
	return tables[offset:min(offset+limit, len(tables))], total, nil
}

func (s *shareTablesStore) Update(_ stdcontext.Context, id int64, opts db.UpdateSLTableOptions) error {
	for _, table := range s.tables {
		if table.ID == id {
			table.ShareToken = *opts.ShareToken
			table.ShareEnabled = *opts.ShareEnabled
			table.ShareIncludeChildren = *opts.ShareIncludeChildren
			table.SharePasswordHash = *opts.SharePasswordHash
			table.SharePasswordSealed = *opts.SharePasswordSealed
			return nil
		}
	}
	return db.ErrSLTableNotFound
}

type shareProjectsStore struct {
	db.ProjectsStore
	project *db.Project
}

func (s *shareProjectsStore) GetByID(_ stdcontext.Context, id int64) (*db.Project, error) {
	if s.project != nil && s.project.ID == id {
		return s.project, nil
	}
	return nil, db.ErrProjectNotFound
}

func (s *shareProjectsStore) GetByUID(_ stdcontext.Context, uid string) (*db.Project, error) {
	if s.project != nil && s.project.UID == uid && !s.project.DeletedAt.Valid {
		return s.project, nil
	}
	return nil, db.ErrProjectNotFound
}

type shareAttemptsStore struct {
	redis.AuthAttemptsStore
	err error
}

func (s *shareAttemptsStore) CheckSignIn(_ stdcontext.Context, _, _ string) error { return s.err }

func TestLinkShareLifecycleAndScope(t *testing.T) {
	sso.SetKey(make([]byte, 32))
	t.Cleanup(func() { sso.SetKey(nil) })
	previousTables, previousProjects, previousAttempts, previousRecords := db.SLTables, db.Projects, redis.AuthAttempts, db.SLRecords
	t.Cleanup(func() {
		db.SLTables, db.Projects, redis.AuthAttempts, db.SLRecords = previousTables, previousProjects, previousAttempts, previousRecords
	})

	table := &db.SLTable{Model: dbutil.Model{ID: 1}, ProjectID: 1, UID: "tblRoot"}
	tables := &shareTablesStore{tables: []*db.SLTable{table,
		{Model: dbutil.Model{ID: 2}, ProjectID: 1, UID: "tblSibling"},
		{Model: dbutil.Model{ID: 3}, ProjectID: 2, UID: "tblOther"},
	}}
	projects := &shareProjectsStore{project: &db.Project{Model: dbutil.Model{ID: 1}, UID: "prjShared"}}
	attempts := &shareAttemptsStore{}
	db.SLTables, db.Projects, redis.AuthAttempts, db.SLRecords = tables, projects, attempts, sharedReadRecords{}

	router := flamego.New()
	router.Use(i18n.Middleware(), context.Contexter(nil))
	router.Map(table, projects.project, db.ProjectRoleManager, collab.NewHub(nil))
	router.Get("/settings", Project.RequireRole(db.ProjectRoleManager), Share.Get)
	router.Put("/settings", Project.RequireRole(db.ProjectRoleManager), Share.LimitBody, form.Bind(form.UpdateLinkShare{}), Share.Update)
	router.Group("/shares/{shareToken}", func() {
		router.Post("/unlock", Share.LimitBody, form.Bind(form.UnlockLinkShare{}), Share.Unlock)
		router.Group("", func() {
			router.Get("", Share.Open)
			router.Get("/tables/{tableUID}", Share.Tabler, func(ctx context.Context) error { return ctx.ApiSuccess(nil) })
		}, Share.RequirePassword)
	}, Share.Loader)

	request := func(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Accept-Language", "en")
		if cookie != nil {
			r.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		return response
	}

	router.Map(db.ProjectRoleEditor)
	require.Equal(t, 403, request("PUT", "/settings", `{"enabled":true}`, nil).Code)
	require.Equal(t, 403, request("GET", "/settings", "", nil).Code)
	router.Map(db.ProjectRoleManager)
	require.Equal(t, 400, request("PUT", "/settings", `{"enabled":true,"password":"`+strings.Repeat("x", 5000)+`"}`, nil).Code)
	require.Equal(t, 200, request("PUT", "/settings", `{"enabled":true}`, nil).Code)
	token := table.ShareToken
	require.Len(t, token, 64)
	path := "/shares/" + token
	require.Equal(t, 200, request("GET", path, "", nil).Code)
	require.Equal(t, 200, request("GET", path+"/tables/tblRoot", "", nil).Code)
	require.Equal(t, 404, request("GET", path+"/tables/tblSibling", "", nil).Code)
	require.Equal(t, 404, request("POST", path+"/tables/tblRoot", `{}`, nil).Code)

	require.Equal(t, 400, request("PUT", "/settings", `{"enabled":true,"password":"short"}`, nil).Code)
	require.Equal(t, 400, request("PUT", "/settings", `{"enabled":true,"password":"      "}`, nil).Code)
	require.Equal(t, token, table.ShareToken)
	previousKey := conf.Auth.SecretKey
	t.Cleanup(func() { conf.Auth.SecretKey = previousKey })
	conf.Auth.SecretKey = ""
	sso.SetKey(nil)
	require.Equal(t, 400, request("PUT", "/settings", `{"enabled":true,"password":"secret123"}`, nil).Code)
	require.Empty(t, table.SharePasswordHash)
	require.Empty(t, table.SharePasswordSealed)
	sso.SetKey(make([]byte, 32))

	for _, password := range []string{"ABCD1234", "abcd1234", "ABCD@#&!", "1234@#&!", "Abc123@#&!()[]{}~:", "ABCD\\\"'`"} {
		body, err := json.Marshal(form.UpdateLinkShare{Enabled: true, Password: &password})
		require.NoError(t, err)
		response := request("PUT", "/settings", string(body), nil)
		require.Equal(t, 200, response.Code, password)
		require.NoError(t, bcrypt.CompareHashAndPassword([]byte(table.SharePasswordHash), []byte(password)))
	}

	response := request("PUT", "/settings", `{"enabled":true,"includeChildren":true,"password":"secret123"}`, nil)
	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Body.String(), `"password":"secret123"`)
	require.Contains(t, response.Body.String(), `"url":"/base/prjShared/tblRoot"`)
	require.NotEqual(t, "secret123", table.SharePasswordSealed)
	plain, err := sso.Open(table.SharePasswordSealed)
	require.NoError(t, err)
	require.Equal(t, "secret123", string(plain))
	require.Contains(t, request("GET", "/settings", "", nil).Body.String(), `"password":"secret123"`)
	require.Equal(t, "no-store", request("GET", "/settings", "", nil).Header().Get("Cache-Control"))
	require.NotContains(t, response.Body.String(), table.SharePasswordSealed)

	hashBefore, sealedBefore := table.SharePasswordHash, table.SharePasswordSealed
	for _, password := range []string{"ABCD123", strings.Repeat("A", 18) + "1", "12345678", "ABCDEFGH",
		"ABCDabcd", "@#&!()[]", "ABCD123 ", "ABCD1234\n", "ABCD123\t", "ABCD123\x00", "ABCD123\x7f",
		"ABCD123中", "ABCD123é", "ABCD123＠"} {
		body, err := json.Marshal(form.UpdateLinkShare{Enabled: true, Password: &password})
		require.NoError(t, err)
		invalid := request("PUT", "/settings", string(body), nil)
		require.Equal(t, 400, invalid.Code, password)
		require.Contains(t, invalid.Body.String(), "8–18")
		require.Equal(t, hashBefore, table.SharePasswordHash)
		require.Equal(t, sealedBefore, table.SharePasswordSealed)
		require.True(t, table.ShareIncludeChildren)
	}

	for _, role := range []db.ProjectRole{db.ProjectRoleViewer, db.ProjectRoleEditor} {
		router.Map(role)
		response := request("GET", "/settings", "", nil)
		require.Equal(t, 403, response.Code)
		require.NotContains(t, response.Body.String(), "secret123")
	}
	router.Map(db.ProjectRoleManager)
	require.NotContains(t, response.Body.String(), table.SharePasswordHash)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(table.SharePasswordHash), []byte("secret123")))
	require.Equal(t, 401, request("GET", path, "", nil).Code)
	require.Equal(t, 403, request("POST", path+"/unlock", `{"password":"wrong"}`, nil).Code)
	attempts.err = redis.ErrTooManyAttempts
	require.Equal(t, 429, request("POST", path+"/unlock", `{"password":"secret123"}`, nil).Code)
	attempts.err = nil
	response = request("POST", path+"/unlock", `{"password":"secret123"}`, nil)
	require.Equal(t, 200, response.Code, response.Body.String())
	var grant *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == shareCookieName {
			grant = cookie
		}
	}
	require.NotNil(t, grant)
	require.True(t, grant.HttpOnly)
	require.Equal(t, "/_/shares/"+token, grant.Path)
	require.Equal(t, http.SameSiteStrictMode, grant.SameSite)
	require.Equal(t, 200, request("GET", path+"/tables/tblSibling", "", grant).Code)
	require.Equal(t, 404, request("GET", path+"/tables/tblOther", "", grant).Code)

	// Visitors never receive the saved password or its encrypted representation.
	public := request("GET", path, "", grant)
	require.Equal(t, 200, public.Code)
	for _, secret := range []string{"secret123", table.SharePasswordHash, table.SharePasswordSealed} {
		require.NotContains(t, public.Body.String(), secret)
	}
	sealed := table.SharePasswordSealed
	table.SharePasswordSealed = "invalid-ciphertext"
	response = request("GET", "/settings", "", nil)
	require.Equal(t, 200, response.Code)
	require.NotContains(t, response.Body.String(), `"password":`)
	table.SharePasswordSealed = sealed

	// Legacy hash-only shares remain accessible and become viewable after saving the password again.
	table.SharePasswordSealed = ""
	require.Equal(t, 200, request("GET", path, "", grant).Code)
	require.NotContains(t, request("GET", "/settings", "", nil).Body.String(), `"password":`)
	response = request("PUT", "/settings", `{"enabled":true,"includeChildren":true,"password":"secret123"}`, nil)
	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Body.String(), `"password":"secret123"`)
	// Keep testing the newly issued password grant after resaving a legacy share.
	response = request("POST", path+"/unlock", `{"password":"secret123"}`, nil)
	require.Equal(t, 200, response.Code)
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == shareCookieName {
			grant = cookie
		}
	}

	// Narrowing the scope and rotating the password apply to existing visitors immediately.
	require.Equal(t, 200, request("PUT", "/settings", `{"enabled":true}`, nil).Code)
	require.Equal(t, 404, request("GET", path+"/tables/tblSibling", "", grant).Code)
	require.Equal(t, 200, request("PUT", "/settings", `{"enabled":true,"password":"changed123"}`, nil).Code)
	require.Equal(t, 401, request("GET", path, "", grant).Code)
	require.Equal(t, 403, request("POST", path+"/unlock", `{"password":"secret123"}`, nil).Code)
	require.Equal(t, 200, request("PUT", "/settings", `{"enabled":true,"password":""}`, nil).Code)
	require.Empty(t, table.SharePasswordSealed)
	require.Equal(t, 200, request("GET", path, "", nil).Code)

	require.Equal(t, 200, request("PUT", "/settings", `{"enabled":true,"password":"ABCD1234"}`, nil).Code)
	require.NotEmpty(t, table.SharePasswordSealed)
	require.Equal(t, 200, request("PUT", "/settings", `{"enabled":false}`, nil).Code)
	require.Empty(t, table.SharePasswordHash)
	require.Empty(t, table.SharePasswordSealed)
	require.Equal(t, 404, request("GET", path, "", grant).Code)
	require.Equal(t, 200, request("PUT", "/settings", `{"enabled":true}`, nil).Code)
	require.NotEqual(t, token, table.ShareToken)
	require.Equal(t, 404, request("GET", path, "", nil).Code)
	projects.project = nil
	require.Equal(t, 404, request("GET", "/shares/"+table.ShareToken, "", nil).Code)
}

func TestResolveNormalTableLinks(t *testing.T) {
	previousTables, previousProjects := db.SLTables, db.Projects
	t.Cleanup(func() { db.SLTables, db.Projects = previousTables, previousProjects })
	project := &db.Project{Model: dbutil.Model{ID: 1}, UID: "prjShared"}
	root := &db.SLTable{Model: dbutil.Model{ID: 1}, ProjectID: 1, UID: "tblRoot",
		ShareEnabled: true, ShareToken: "root-token", SharePasswordHash: "private-hash", SharePasswordSealed: "private-ciphertext"}
	sibling := &db.SLTable{Model: dbutil.Model{ID: 2}, ProjectID: 1, UID: "tblSibling"}
	other := &db.SLTable{Model: dbutil.Model{ID: 3}, ProjectID: 2, UID: "tblOther", ShareEnabled: true, ShareToken: "other-token"}
	db.SLTables = &shareTablesStore{tables: []*db.SLTable{root, sibling, other}}
	db.Projects = &shareProjectsStore{project: project}

	router := flamego.New()
	router.Use(i18n.Middleware(), context.Contexter(nil))
	router.Get("/shares/resolve", Share.Resolve)
	router.Get("/shares/{shareToken}", Share.Loader, Share.RequirePassword, func(ctx context.Context) error { return ctx.ApiSuccess(nil) })
	resolve := func(query string, status int, token string) {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/shares/resolve?"+query, nil))
		require.Equal(t, status, response.Code, query)
		require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		if token != "" {
			require.Contains(t, response.Body.String(), `"token":"`+token+`"`)
		}
		for _, secret := range []string{"private-hash", "private-ciphertext", "tblSibling", "tblOther"} {
			require.NotContains(t, response.Body.String(), secret)
		}
	}

	resolve("projectUID=prjShared&tableUID=tblRoot", 200, "root-token")
	resolve("projectUID=prjShared", 200, "root-token")
	resolve("projectUID=prjShared&tableUID=tblSibling", 404, "")
	resolve("projectUID=prjShared&tableUID=tblOther", 404, "")
	resolve("projectUID=missing&tableUID=tblRoot", 404, "")
	resolve("", 404, "")

	root.ShareIncludeChildren = true
	resolve("projectUID=prjShared&tableUID=tblSibling", 200, "root-token")
	resolve("projectUID=prjShared&tableUID=missing", 404, "")
	resolve("projectUID=prjShared&tableUID=tblOther", 404, "")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/shares/root-token", nil))
	require.Equal(t, 401, response.Code, "Resolving a share must not bypass its password")

	sibling.ShareEnabled, sibling.ShareToken = true, "sibling-token"
	resolve("projectUID=prjShared&tableUID=tblSibling", 200, "sibling-token")
	sibling.ShareEnabled = false
	root.ShareEnabled = false
	resolve("projectUID=prjShared&tableUID=tblRoot", 404, "")
	resolve("projectUID=prjShared&tableUID=tblSibling", 404, "")
	resolve("projectUID=prjShared", 404, "")
	root.ShareEnabled, root.ShareToken = true, "rotated-token"
	resolve("projectUID=prjShared&tableUID=tblRoot", 200, "rotated-token")
	sibling.DeletedAt.Valid = true
	resolve("projectUID=prjShared&tableUID=tblSibling", 404, "")
	project.DeletedAt.Valid = true
	resolve("projectUID=prjShared&tableUID=tblRoot", 404, "")

	project.DeletedAt.Valid, sibling.DeletedAt.Valid = false, false
	var paginated []*db.SLTable
	for i := 0; i < 1000; i++ {
		paginated = append(paginated, &db.SLTable{ProjectID: 1, UID: "tblPrivate" + strconv.Itoa(i)})
	}
	db.SLTables = &shareTablesStore{tables: append(paginated, root, sibling)}
	resolve("projectUID=prjShared&tableUID=tblRoot", 200, "rotated-token")
	resolve("projectUID=prjShared&tableUID=tblSibling", 200, "rotated-token")
}

func TestShareGrantRejectsTamperingExpiryAndOtherShares(t *testing.T) {
	table := &db.SLTable{ShareToken: "token", SharePasswordHash: "hash"}
	value := shareGrant(table, strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
	require.True(t, validShareGrant(table, value))
	require.False(t, validShareGrant(table, value+"x"))
	require.False(t, validShareGrant(table, "invalid"))
	require.False(t, validShareGrant(table, shareGrant(table, strconv.FormatInt(time.Now().Add(-time.Second).Unix(), 10))))
	require.False(t, validShareGrant(table, shareGrant(table, strconv.FormatInt(time.Now().Add(48*time.Hour).Unix(), 10))))
	table.ShareToken = "another-token"
	require.False(t, validShareGrant(table, value))
}

func TestSharedSnapshotExcludesPrivateConfiguration(t *testing.T) {
	var files []interface{}
	require.NoError(t, json.Unmarshal([]byte(`[{"uid":"filOne","name":"photo.png","url":"/_/projects/prjPrivate/attachments/filOne"}]`), &files))
	snapshot := &dto.TableSnapshot{
		Table: &dto.Table{UID: "tblOne"},
		Fields: []*dto.Field{{UID: "fldOne", Type: "attachment"}, {
			UID: "fldSecret", Shortcut: &db.FieldShortcut{ID: "secret"},
			Metadata: map[string]interface{}{"optionsReference": "private", "options": []string{"visible"}},
		}},
		Records: []*dto.Record{{TableUID: "tblOne", Data: map[string]interface{}{"fldOne": files}}},
	}
	prepareSharedFields(snapshot.Fields, "token")
	prepareSharedRecords(snapshot.Records, "token")
	require.Nil(t, snapshot.Fields[1].Shortcut)
	require.NotContains(t, snapshot.Fields[1].Metadata, "optionsReference")
	require.Contains(t, snapshot.Fields[1].Metadata, "options")
	require.Equal(t, "/_/shares/token/tables/tblOne/attachments/filOne", files[0].(map[string]interface{})["url"])
}

type sharedReadFields struct{ db.SLFieldsStore }

func (sharedReadFields) ListByTableID(_ stdcontext.Context, _ int64) (db.SLFieldList, error) {
	return db.SLFieldList{{UID: "fldOne", Type: db.TextFieldType, Shortcut: &db.FieldShortcut{ID: "private-shortcut"},
		Metadata: datatypes.NewJSONType[db.SLFieldMetadata](map[string]interface{}{"optionsReference": "private-reference", "options": []string{"visible"}})}}, nil
}

type sharedReadRecords struct{ db.SLRecordsStore }

func (sharedReadRecords) CountByTableID(_ stdcontext.Context, _ int64) (int64, error) { return 0, nil }

func (sharedReadRecords) ListByUIDs(_ stdcontext.Context, _ int64, _ []string) ([]*db.SLRecord, error) {
	return []*db.SLRecord{{UID: "recOne", Data: datatypes.JSON(`{"fldOne":"visible","files":[{"uid":"filOne","url":"/_/projects/private/attachments/filOne"}]}`)}}, nil
}

type sharedReadViews struct{ db.SLViewsStore }

func (sharedReadViews) ListByTableID(_ stdcontext.Context, _ int64) ([]*db.SLView, error) {
	return []*db.SLView{{UID: "viwOne", Name: "Visible view", Type: db.GridViewType, Config: datatypes.JSON(`{"hiddenFields":["fldOne"]}`)}}, nil
}

type sharedReadChangesets struct{ db.SLChangesetsStore }

func (sharedReadChangesets) ListSince(_ stdcontext.Context, _ int64, since int64, limit int) ([]*db.SLChangeset, error) {
	changesets := make([]*db.SLChangeset, 0, limit)
	for rev := since + 1; rev <= 202 && len(changesets) < limit; rev++ {
		operations := `[{"command":"secret-command","actions":[{"action":"field.setShortcut","fieldUID":"fldOne","shortcut":{"id":"private-shortcut"}}]}]`
		if rev == 202 {
			operations = `[{"command":"SetRecord","actions":[{"action":"field.set","fieldUID":"fldOne","field":{"metadata":{"optionsReference":"private-reference","options":["visible"]}}},{"action":"record.set","recordUID":"recOne","values":{"fldOne":"live","files":[{"uid":"filOne","url":"private-url"}]}}]}]`
		}
		changesets = append(changesets, &db.SLChangeset{Rev: rev, Operations: datatypes.JSON(operations)})
	}
	return changesets, nil
}

func TestSharedReadsReuseCollaborationHandlersWithAccessAndPrivacyFiltering(t *testing.T) {
	previousTables, previousProjects := db.SLTables, db.Projects
	previousFields, previousRecords, previousViews, previousChangesets := db.SLFields, db.SLRecords, db.SLViews, db.SLChangesets
	t.Cleanup(func() {
		db.SLTables, db.Projects = previousTables, previousProjects
		db.SLFields, db.SLRecords, db.SLViews, db.SLChangesets = previousFields, previousRecords, previousViews, previousChangesets
	})
	root := &db.SLTable{Model: dbutil.Model{ID: 1}, UID: "tblRoot", ProjectID: 1, ShareToken: "token", ShareEnabled: true}
	db.SLTables = &shareTablesStore{tables: []*db.SLTable{root, {Model: dbutil.Model{ID: 2}, UID: "tblSibling", ProjectID: 1}}}
	db.Projects = &shareProjectsStore{project: &db.Project{Model: dbutil.Model{ID: 1}}}
	db.SLFields, db.SLRecords, db.SLViews, db.SLChangesets = sharedReadFields{}, sharedReadRecords{}, sharedReadViews{}, sharedReadChangesets{}

	router := flamego.New()
	router.Use(i18n.Middleware(), context.Contexter(nil))
	router.Group("/shares/{shareToken}", func() {
		router.Group("/tables/{tableUID}", func() {
			router.Get("/changesets", Share.Changesets)
			router.Get("/fields", Share.Fields)
			router.Get("/views", Share.Views)
			router.Post("/records/fetch", Collab.LimitFetchBody, form.Bind(form.FetchRecords{}), Share.FetchRecords)
		}, Share.Tabler)
	}, Share.Loader, Share.RequirePassword)
	router.Group("/member", func() {
		router.Get("/changesets", Collab.ListChangesets)
		router.Get("/fields", Schemaless.ListFields)
		router.Post("/records/fetch", Collab.LimitFetchBody, form.Bind(form.FetchRecords{}), Collab.FetchRecords)
	}, func(c flamego.Context) { c.Map(root) })
	request := func(method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Accept-Language", "en")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, r)
		return response
	}

	for _, read := range []struct{ method, suffix, body string }{
		{"GET", "/changesets", ""}, {"GET", "/fields", ""}, {"GET", "/views", ""}, {"POST", "/records/fetch", `{"uids":["recOne"]}`},
	} {
		t.Run(read.suffix, func(t *testing.T) {
			path := "/shares/token/tables/tblRoot" + read.suffix
			response := request(read.method, path, read.body)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			require.NotContains(t, response.Body.String(), "private")
			require.Equal(t, http.StatusNotFound, request(read.method, "/shares/token/tables/tblSibling"+read.suffix, read.body).Code)
			root.ShareIncludeChildren = true
			require.Equal(t, http.StatusOK, request(read.method, "/shares/token/tables/tblSibling"+read.suffix, read.body).Code)
			root.ShareIncludeChildren = false
			root.SharePasswordHash = "protected"
			require.Equal(t, http.StatusUnauthorized, request(read.method, path, read.body).Code)
			root.SharePasswordHash = ""
			root.ShareEnabled = false
			require.Equal(t, http.StatusNotFound, request(read.method, path, read.body).Code)
			root.ShareEnabled = true
		})
	}

	var envelope struct {
		Data dto.ListChangesetsResp `json:"data"`
	}
	response := request("GET", "/shares/token/tables/tblRoot/changesets?since=0", "")
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	page := envelope.Data
	require.True(t, page.HasMore)
	require.Len(t, page.Changesets, 200)
	require.Equal(t, int64(200), page.Changesets[199].Rev)
	require.NotNil(t, page.Changesets[0].Operations)
	require.Empty(t, page.Changesets[0].Operations)
	response = request("GET", "/shares/token/tables/tblRoot/changesets?since=200", "")
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &envelope))
	page = envelope.Data
	require.False(t, page.HasMore)
	require.Len(t, page.Changesets, 2)
	require.Equal(t, "live", page.Changesets[1].Operations[0].Actions[1].Values["fldOne"])
	require.Contains(t, response.Body.String(), "/_/shares/token/tables/tblRoot/attachments/filOne")
	require.Contains(t, request("GET", "/shares/token/tables/tblRoot/fields", "").Body.String(), "visible")
	require.Contains(t, request("GET", "/shares/token/tables/tblRoot/views", "").Body.String(), "hiddenFields")
	require.Contains(t, request("POST", "/shares/token/tables/tblRoot/records/fetch", `{"uids":["recOne"]}`).Body.String(), "/_/shares/token/tables/tblRoot/attachments/filOne")

	// Member responses retain their private configuration after serving public requests.
	require.Contains(t, request("GET", "/member/changesets", "").Body.String(), "private-shortcut")
	require.Contains(t, request("GET", "/member/fields", "").Body.String(), "private-reference")
	require.Contains(t, request("POST", "/member/records/fetch", `{"uids":["recOne"]}`).Body.String(), "/_/projects/private/")
	require.Equal(t, http.StatusBadRequest, request("POST", "/shares/token/tables/tblRoot/records/fetch", `{"uids":["`+strings.Repeat("x", 64<<10)+`"]}`).Code)
}
