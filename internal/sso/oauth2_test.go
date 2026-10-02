package sso

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
)

const testBaseURL = "https://sayrud.test"

func newTestProvider(t *testing.T, typ db.AuthProviderType, c Config, s Secrets) *db.AuthProvider {
	t.Helper()
	raw, err := json.Marshal(c)
	require.NoError(t, err)
	sealed, err := EncodeSecrets(s)
	require.NoError(t, err)
	return &db.AuthProvider{ID: 7, Slug: "test", Type: typ, Enabled: true, Config: raw, Secrets: sealed}
}

func s256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// fakeOAuth2 is a fake OAuth 2.0 server in the style of GitHub.
type fakeOAuth2 struct {
	*httptest.Server
	challenge string
	user      string
	emails    string
}

func newFakeOAuth2(t *testing.T) *fakeOAuth2 {
	f := &fakeOAuth2{
		user:   `{"id": 9001, "login": "octocat", "name": "The Octocat", "email": null, "teams": ["dev"]}`,
		emails: `[{"email": "old@example.com", "primary": false, "verified": true}, {"email": "Octo@Example.com", "primary": true, "verified": true}]`,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		if r.Form.Get("code") != "good" || s256(r.Form.Get("code_verifier")) != f.challenge {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		user, pass, _ := r.BasicAuth()
		if user == "" {
			user, pass = r.Form.Get("client_id"), r.Form.Get("client_secret")
		}
		require.Equal(t, "cid", user)
		require.Equal(t, "csecret", pass)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok","token_type":"bearer"}`))
	})
	auth := func(next string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer tok" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(next))
		}
	}
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) { auth(f.user)(w, r) })
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) { auth(f.emails)(w, r) })
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeOAuth2) config(withEmails bool) Config {
	c := Config{
		AuthURL:     f.URL + "/authorize",
		TokenURL:    f.URL + "/token",
		UserInfoURL: f.URL + "/user",
		ClientID:    "cid",
		Scopes:      []string{"read:user", "user:email"},
		GroupsPath:  "teams",
	}
	if withEmails {
		c.EmailsURL = f.URL + "/user/emails"
	}
	return c.Normalize(db.AuthProviderOAuth2)
}

func beginOAuth2(t *testing.T, p RedirectProvider, f *fakeOAuth2) (*State, url.Values) {
	t.Helper()
	state, err := NewState(7, ModeLogin, "/", 0)
	require.NoError(t, err)
	redirect, err := p.Begin(context.Background(), state)
	require.NoError(t, err)
	u, err := url.Parse(redirect)
	require.NoError(t, err)
	q := u.Query()
	if f != nil {
		f.challenge = q.Get("code_challenge")
	}
	return state, q
}

func TestOAuth2(t *testing.T) {
	withTestKey(t)
	f := newFakeOAuth2(t)
	p, err := NewRedirect(newTestProvider(t, db.AuthProviderOAuth2, f.config(true), Secrets{ClientSecret: "csecret"}), Options{BaseURL: testBaseURL})
	require.NoError(t, err)

	state, q := beginOAuth2(t, p, f)
	assert.Equal(t, "cid", q.Get("client_id"))
	assert.Equal(t, testBaseURL+"/_/auth/sso/test/callback", q.Get("redirect_uri"))
	assert.Equal(t, state.State, q.Get("state"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.Equal(t, "read:user user:email", q.Get("scope"))
	assert.NotEmpty(t, state.Verifier)

	id, err := p.Complete(context.Background(), httptest.NewRequest(http.MethodGet, "/callback?code=good&state="+state.State, nil), state)
	require.NoError(t, err)
	assert.Equal(t, &Identity{Subject: "9001", Email: "octo@example.com", EmailVerified: true, Name: "The Octocat", Groups: []string{"dev"}}, id)

	// A wrong code or an error from the IdP is idp_error.
	_, err = p.Complete(context.Background(), httptest.NewRequest(http.MethodGet, "/callback?code=bad", nil), state)
	assert.Equal(t, CodeIdPError, ErrorCode(err))
	_, err = p.Complete(context.Background(), httptest.NewRequest(http.MethodGet, "/callback?error=access_denied", nil), state)
	assert.Equal(t, CodeIdPError, ErrorCode(err))
}

func TestOAuth2_WithoutEmails(t *testing.T) {
	withTestKey(t)
	f := newFakeOAuth2(t)
	f.user = `{"id": "u-1", "email": "A@B.com"}`
	c := f.config(false)
	c.TrustEmail = true
	p, err := NewRedirect(newTestProvider(t, db.AuthProviderOAuth2, c, Secrets{ClientSecret: "csecret"}), Options{BaseURL: testBaseURL})
	require.NoError(t, err)

	state, _ := beginOAuth2(t, p, f)
	id, err := p.Complete(context.Background(), httptest.NewRequest(http.MethodGet, "/callback?code=good", nil), state)
	require.NoError(t, err)
	assert.Equal(t, &Identity{Subject: "u-1", Email: "a@b.com", EmailVerified: true}, id)

	f.user = `{"email": "a@b.com"}`
	state, _ = beginOAuth2(t, p, f)
	_, err = p.Complete(context.Background(), httptest.NewRequest(http.MethodGet, "/callback?code=good", nil), state)
	assert.Equal(t, CodeIdPError, ErrorCode(err), "missing subject")
}

func TestPickEmail(t *testing.T) {
	email, verified, err := pickEmail([]byte(`[{"email":"a@x.com","primary":true,"verified":false},{"email":"b@x.com","verified":true}]`))
	require.NoError(t, err)
	assert.Equal(t, "b@x.com", email)
	assert.True(t, verified)

	email, verified, err = pickEmail([]byte(`[{"email":"a@x.com","primary":true,"verified":false}]`))
	require.NoError(t, err)
	assert.Equal(t, "a@x.com", email)
	assert.False(t, verified)

	_, _, err = pickEmail([]byte(`{"email":"a@x.com"}`))
	require.Error(t, err)
}
