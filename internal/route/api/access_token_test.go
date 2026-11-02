package api

import (
	stdcontext "context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/redis"
)

type tokenAuthTokensStore struct {
	db.AccessTokensStore
	tokens  map[string]*db.AccessToken
	touched []int64
}

func (s *tokenAuthTokensStore) Verify(_ stdcontext.Context, plaintext string) (*db.AccessToken, error) {
	token, ok := s.tokens[plaintext]
	if !ok {
		return nil, db.ErrAccessTokenNotFound
	}
	if token.Expired(time.Now()) {
		return nil, db.ErrAccessTokenExpired
	}
	return token, nil
}

func (s *tokenAuthTokensStore) Touch(_ stdcontext.Context, token *db.AccessToken, _ string) error {
	s.touched = append(s.touched, token.ID)
	return nil
}

type tokenAuthUsersStore struct {
	db.UsersStore
	users map[int64]*db.User
}

func (s *tokenAuthUsersStore) GetByID(_ stdcontext.Context, id int64) (*db.User, error) {
	if u, ok := s.users[id]; ok {
		return u, nil
	}
	return nil, db.ErrUserNotFound
}

type tokenAuthSessionsStore struct {
	db.UserSessionsStore
	t *testing.T
}

func (s *tokenAuthSessionsStore) GetByToken(stdcontext.Context, string) (*db.UserSession, error) {
	s.t.Error("the session cookie must be ignored once a token is presented")
	return nil, db.ErrUserSessionNotFound
}

type tokenAuthAttempts struct {
	redis.AuthAttemptsStore
	blocked  bool
	failures int
}

func (s *tokenAuthAttempts) CheckToken(stdcontext.Context, string) error {
	if s.blocked {
		return redis.ErrTooManyAttempts
	}
	return nil
}

func (s *tokenAuthAttempts) CountTokenFailure(stdcontext.Context, string) error {
	s.failures++
	return nil
}

func TestTokenAuthenticator(t *testing.T) {
	previousTokens, previousUsers, previousSessions, previousAttempts := db.AccessTokens, db.Users, db.UserSessions, redis.AuthAttempts
	t.Cleanup(func() {
		db.AccessTokens, db.Users, db.UserSessions, redis.AuthAttempts = previousTokens, previousUsers, previousSessions, previousAttempts
	})
	past := time.Now().Add(-time.Minute)
	disabledAt := time.Now()
	tokens := &tokenAuthTokensStore{tokens: map[string]*db.AccessToken{
		"sat_read":       {ID: 1, UserID: 7, Scopes: []db.AccessTokenScope{db.AccessTokenScopeRead}},
		"sat_write":      {ID: 2, UserID: 7, Scopes: []db.AccessTokenScope{db.AccessTokenScopeWrite}},
		"sat_expired":    {ID: 3, UserID: 7, Scopes: []db.AccessTokenScope{db.AccessTokenScopeRead}, ExpiresAt: &past},
		"sat_disabled":   {ID: 4, UserID: 8, Scopes: []db.AccessTokenScope{db.AccessTokenScopeRead}},
		"sat_orphan":     {ID: 5, UserID: 9, Scopes: []db.AccessTokenScope{db.AccessTokenScopeRead}},
		"sat_owner_off":  {ID: 6, UserID: 10, Scopes: []db.AccessTokenScope{db.AccessTokenScopeRead}},
		"sat_owner_gone": {ID: 7, UserID: 12, Scopes: []db.AccessTokenScope{db.AccessTokenScopeRead}},
	}}
	attempts := &tokenAuthAttempts{}
	db.AccessTokens = tokens
	db.Users = &tokenAuthUsersStore{users: map[int64]*db.User{
		7:  {Model: dbutil.Model{ID: 7}, Kind: db.UserKindBot, OwnerID: 13},
		8:  {Model: dbutil.Model{ID: 8}, Kind: db.UserKindBot, OwnerID: 13, DisabledAt: &disabledAt},
		10: {Model: dbutil.Model{ID: 10}, Kind: db.UserKindBot, OwnerID: 11},
		11: {Model: dbutil.Model{ID: 11}, DisabledAt: &disabledAt},
		12: {Model: dbutil.Model{ID: 12}, Kind: db.UserKindBot, OwnerID: 99},
		13: {Model: dbutil.Model{ID: 13}},
	}}
	db.UserSessions = &tokenAuthSessionsStore{t: t}
	redis.AuthAttempts = attempts

	f := flamego.New()
	f.Use(i18n.Middleware(), context.Contexter(nil))
	handler := func(ctx context.Context, user *db.User, token *db.AccessToken) error {
		require.Equal(t, int64(7), user.ID)
		require.NotNil(t, token)
		return ctx.Status(http.StatusNoContent)
	}
	f.Get("/_/projects/prj", Auth.TokenAuthenticator, handler)
	f.Put("/_/projects/prj", Auth.TokenAuthenticator, handler)
	f.Post("/_/projects/prj/tables/tbl/records/query", Auth.TokenAuthenticator, handler)
	f.Post("/_/projects/prj/tables/tbl/records", Auth.TokenAuthenticator, handler)
	f.Get("/_/auth/sessions", Auth.Authenticator, handler)
	f.Get("/_/projects/prj/ws", Auth.TokenAuthenticator, Auth.SessionOnly, handler)

	request := func(method, path, authorization string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, nil)
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-of-another-user"})
		if authorization != "" {
			r.Header.Set("Authorization", authorization)
		}
		response := httptest.NewRecorder()
		f.ServeHTTP(response, r)
		return response
	}
	message := func(response *httptest.ResponseRecorder) string {
		var body struct {
			Msg string `json:"msg"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		return body.Msg
	}

	for _, tc := range []struct {
		name          string
		method, path  string
		authorization string
		want          int
	}{
		{"read token reads", http.MethodGet, "/_/projects/prj", "Bearer sat_read", http.StatusNoContent},
		{"scheme is case-insensitive", http.MethodGet, "/_/projects/prj", "bearer sat_read", http.StatusNoContent},
		{"read token queries", http.MethodPost, "/_/projects/prj/tables/tbl/records/query", "Bearer sat_read", http.StatusNoContent},
		{"read token can not write", http.MethodPut, "/_/projects/prj", "Bearer sat_read", http.StatusForbidden},
		{"read token can not create", http.MethodPost, "/_/projects/prj/tables/tbl/records", "Bearer sat_read", http.StatusForbidden},
		{"write token writes", http.MethodPost, "/_/projects/prj/tables/tbl/records", "Bearer sat_write", http.StatusNoContent},
		{"write token can not read", http.MethodGet, "/_/projects/prj", "Bearer sat_write", http.StatusForbidden},
		{"expired", http.MethodGet, "/_/projects/prj", "Bearer sat_expired", http.StatusUnauthorized},
		{"disabled user", http.MethodGet, "/_/projects/prj", "Bearer sat_disabled", http.StatusUnauthorized},
		{"deleted user", http.MethodGet, "/_/projects/prj", "Bearer sat_orphan", http.StatusUnauthorized},
		{"bot of a disabled owner", http.MethodGet, "/_/projects/prj", "Bearer sat_owner_off", http.StatusUnauthorized},
		{"bot of a deleted owner", http.MethodGet, "/_/projects/prj", "Bearer sat_owner_gone", http.StatusUnauthorized},
		{"session-only route", http.MethodGet, "/_/auth/sessions", "Bearer sat_read", http.StatusForbidden},
		{"websocket", http.MethodGet, "/_/projects/prj/ws", "Bearer sat_read", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := request(tc.method, tc.path, tc.authorization)
			require.Equal(t, tc.want, response.Code, response.Body.String())
		})
	}
	assert.Equal(t, 1, attempts.failures, "only the token of the deleted user is counted, valid, expired and disabled ones are not guesses")
	assert.Contains(t, tokens.touched, int64(1))
	assert.NotContains(t, tokens.touched, int64(4))

	response := request(http.MethodPut, "/_/projects/prj", "Bearer sat_read")
	assert.True(t, strings.Contains(message(response), "write"), message(response))

	response = request(http.MethodGet, "/_/projects/prj", "Bearer sat_unknown")
	require.Equal(t, http.StatusUnauthorized, response.Code)
	assert.Equal(t, 2, attempts.failures)

	attempts.blocked = true
	response = request(http.MethodGet, "/_/projects/prj", "Bearer sat_read")
	require.Equal(t, http.StatusTooManyRequests, response.Code)
}

type createTokenTokensStore struct {
	db.AccessTokensStore
	existing int
	created  *db.CreateAccessTokenOptions
}

func (s *createTokenTokensStore) ListByUserID(stdcontext.Context, int64) ([]*db.AccessToken, error) {
	return make([]*db.AccessToken, s.existing), nil
}

func (s *createTokenTokensStore) Create(_ stdcontext.Context, opts db.CreateAccessTokenOptions) (*db.AccessToken, string, error) {
	s.created = &opts
	return &db.AccessToken{ID: 1, UserID: opts.UserID, Name: opts.Name, Prefix: "sat_12345678", Scopes: opts.Scopes, ProjectIDs: opts.ProjectIDs, ExpiresAt: opts.ExpiresAt}, "sat_secret", nil
}

type createTokenProjectsStore struct {
	db.ProjectsStore
}

func (createTokenProjectsStore) ListByUIDs(_ stdcontext.Context, uids []string) ([]*db.Project, error) {
	var projects []*db.Project
	for _, uid := range uids {
		if uid == "prj1" {
			projects = append(projects, &db.Project{Model: dbutil.Model{ID: 1}, UID: "prj1", Name: "Tasks"})
		}
	}
	return projects, nil
}

func TestCreateAccessToken(t *testing.T) {
	previousTokens, previousProjects := db.AccessTokens, db.Projects
	t.Cleanup(func() { db.AccessTokens, db.Projects = previousTokens, previousProjects })
	tokens := &createTokenTokensStore{}
	db.AccessTokens, db.Projects = tokens, createTokenProjectsStore{}

	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)
	for _, tc := range []struct {
		name string
		form form.CreateAccessToken
		want int
	}{
		{"valid", form.CreateAccessToken{Name: " CI ", Scopes: []string{"write", "read", "read"}, Projects: []string{"prj1", "prj1"}, ExpiresAt: &future}, http.StatusOK},
		{"blank name", form.CreateAccessToken{Name: "  ", Scopes: []string{"read"}}, http.StatusBadRequest},
		{"no scope", form.CreateAccessToken{Name: "CI"}, http.StatusBadRequest},
		{"unknown scope", form.CreateAccessToken{Name: "CI", Scopes: []string{"admin"}}, http.StatusBadRequest},
		{"expired", form.CreateAccessToken{Name: "CI", Scopes: []string{"read"}, ExpiresAt: &past}, http.StatusBadRequest},
		{"unknown project", form.CreateAccessToken{Name: "CI", Scopes: []string{"read"}, Projects: []string{"prj1", "prj2"}}, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tokens.created = nil
			f := flamego.New()
			f.Use(i18n.Middleware(), context.Contexter(nil))
			f.Post("/", func(ctx context.Context) error { return createAccessToken(ctx, 7, 1, tc.form) })
			response := httptest.NewRecorder()
			f.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))
			require.Equal(t, tc.want, response.Code, response.Body.String())
			if tc.want != http.StatusOK {
				assert.Nil(t, tokens.created)
				return
			}
			require.NotNil(t, tokens.created)
			assert.Equal(t, "CI", tokens.created.Name)
			assert.Equal(t, []db.AccessTokenScope{db.AccessTokenScopeRead, db.AccessTokenScopeWrite}, tokens.created.Scopes)
			assert.Equal(t, []int64{1}, tokens.created.ProjectIDs)
			assert.Equal(t, int64(7), tokens.created.UserID)
			assert.Equal(t, int64(1), tokens.created.CreatedBy)

			var body struct {
				Data dto.CreateAccessTokenResp `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			assert.Equal(t, "sat_secret", body.Data.Secret)
			require.Len(t, body.Data.Token.Projects, 1)
			assert.Equal(t, "Tasks", body.Data.Token.Projects[0].Name)
		})
	}

	tokens.existing = maxAccessTokensPerUser
	f := flamego.New()
	f.Use(i18n.Middleware(), context.Contexter(nil))
	f.Post("/", func(ctx context.Context) error {
		return createAccessToken(ctx, 7, 7, form.CreateAccessToken{Name: "CI", Scopes: []string{"read"}})
	})
	response := httptest.NewRecorder()
	f.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/", nil))
	require.Equal(t, http.StatusConflict, response.Code)
}

type botQuotaUsersStore struct {
	db.UsersStore
	count int64
}

func (s botQuotaUsersStore) CountBotsByOwner(stdcontext.Context, int64) (int64, error) {
	return s.count, nil
}

func TestBotQuota(t *testing.T) {
	previous := db.Users
	t.Cleanup(func() { db.Users = previous })
	f := flamego.New()
	f.Use(i18n.Middleware(), context.Contexter(nil))

	for _, tc := range []struct {
		name      string
		user      *db.User
		settings  db.SystemSettings
		count     int64
		limit     int
		canCreate bool
	}{
		{"admins are not limited", &db.User{IsAdmin: true}, db.SystemSettings{AllowMemberBots: false, MemberBotLimit: 1}, 9, 0, true},
		{"below the limit", &db.User{}, db.SystemSettings{AllowMemberBots: true, MemberBotLimit: 2}, 1, 2, true},
		{"at the limit", &db.User{}, db.SystemSettings{AllowMemberBots: true, MemberBotLimit: 2}, 2, 2, false},
		{"turned off", &db.User{}, db.SystemSettings{AllowMemberBots: false, MemberBotLimit: 5}, 0, 5, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db.Users = botQuotaUsersStore{count: tc.count}
			f.Get("/"+strings.ReplaceAll(tc.name, " ", "-"), func(ctx context.Context) error {
				limit, canCreate, err := botQuota(ctx, tc.user, &tc.settings)
				require.NoError(t, err)
				assert.Equal(t, tc.limit, limit)
				assert.Equal(t, tc.canCreate, canCreate)
				return ctx.Status(http.StatusNoContent)
			})
			response := httptest.NewRecorder()
			f.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/"+strings.ReplaceAll(tc.name, " ", "-"), nil))
			require.Equal(t, http.StatusNoContent, response.Code)
		})
	}
}

func TestBearerToken(t *testing.T) {
	for _, tc := range []struct {
		header string
		token  string
		ok     bool
	}{
		{"", "", false},
		{"Basic dXNlcjpwYXNz", "", false},
		{"Bearer", "", false},
		{"Bearer sat_abc", "sat_abc", true},
		{"BEARER  sat_abc ", "sat_abc", true},
		{"Bearer ", "", true},
	} {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.header != "" {
			r.Header.Set("Authorization", tc.header)
		}
		token, ok := bearerToken(r)
		assert.Equal(t, tc.ok, ok, tc.header)
		assert.Equal(t, tc.token, token, tc.header)
	}
}

func TestRequiredScope(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         db.AccessTokenScope
	}{
		{http.MethodGet, "/_/projects/prj/tables", db.AccessTokenScopeRead},
		{http.MethodHead, "/_/projects/prj", db.AccessTokenScopeRead},
		{http.MethodPost, "/_/projects/prj/tables/tbl/records/fetch", db.AccessTokenScopeRead},
		{http.MethodPost, "/_/projects/prj/tables/tbl/fields/options/resolve", db.AccessTokenScopeRead},
		{http.MethodPost, "/_/projects/prj/tables/tbl/records/query/x", db.AccessTokenScopeWrite},
		{http.MethodPut, "/_/projects/prj/tables/tbl/records/query", db.AccessTokenScopeWrite},
		{http.MethodPost, "/_/projects/prj/tables/tbl/records/batch", db.AccessTokenScopeWrite},
		{http.MethodDelete, "/_/projects/prj", db.AccessTokenScopeWrite},
	} {
		assert.Equal(t, tc.want, requiredScope(httptest.NewRequest(tc.method, tc.path, nil)), tc.method+" "+tc.path)
	}
}
