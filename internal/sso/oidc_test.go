package sso

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
)

// fakeOIDC is a fake OIDC provider, idTokenClaims are the extra claims of the issued ID tokens.
type fakeOIDC struct {
	*httptest.Server
	key           *rsa.PrivateKey
	challenge     string
	nonce         string
	idTokenClaims map[string]interface{}
	userInfo      map[string]interface{}
}

func newFakeOIDC(t *testing.T) *fakeOIDC {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	f := &fakeOIDC{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"issuer":                                f.URL,
			"authorization_endpoint":                f.URL + "/authorize",
			"token_endpoint":                        f.URL + "/token",
			"jwks_uri":                              f.URL + "/jwks",
			"userinfo_endpoint":                     f.URL + "/userinfo",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		if r.Form.Get("code") != "good" || s256(r.Form.Get("code_verifier")) != f.challenge {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
			return
		}
		claims := map[string]interface{}{
			"iss":   f.URL,
			"sub":   "user-42",
			"aud":   "cid",
			"exp":   time.Now().Add(time.Hour).Unix(),
			"iat":   time.Now().Unix(),
			"nonce": f.nonce,
		}
		for k, v := range f.idTokenClaims {
			claims[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok",
			"token_type":   "Bearer",
			"id_token":     f.sign(t, claims),
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(f.userInfo)
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

func (f *fakeOIDC) sign(t *testing.T, claims map[string]interface{}) string {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: f.key, KeyID: "k1"}}, (&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(t, err)
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	obj, err := signer.Sign(payload)
	require.NoError(t, err)
	raw, err := obj.CompactSerialize()
	require.NoError(t, err)
	return raw
}

func (f *fakeOIDC) begin(t *testing.T, p RedirectProvider) *State {
	t.Helper()
	state, err := NewState(7, ModeLogin, "/", 0)
	require.NoError(t, err)
	redirect, err := p.Begin(context.Background(), state)
	require.NoError(t, err)
	u, err := url.Parse(redirect)
	require.NoError(t, err)
	q := u.Query()
	require.Equal(t, f.URL+"/authorize", u.Scheme+"://"+u.Host+u.Path)
	require.Equal(t, "openid email profile", q.Get("scope"))
	require.Equal(t, state.Nonce, q.Get("nonce"))
	f.challenge = q.Get("code_challenge")
	f.nonce = q.Get("nonce")
	return state
}

func TestOIDC(t *testing.T) {
	withTestKey(t)
	f := newFakeOIDC(t)
	c := Config{Issuer: f.URL, ClientID: "cid", GroupsClaim: "https://example.com/groups"}.Normalize(db.AuthProviderOIDC)
	p, err := NewRedirect(newTestProvider(t, db.AuthProviderOIDC, c, Secrets{ClientSecret: "cs"}), Options{BaseURL: testBaseURL})
	require.NoError(t, err)
	callback := func(state *State) *http.Request {
		return httptest.NewRequest(http.MethodGet, "/callback?code=good&state="+state.State, nil)
	}

	f.idTokenClaims = map[string]interface{}{
		"email":                      "Alice@Example.com",
		"email_verified":             true,
		"name":                       "Alice",
		"https://example.com/groups": []string{"dev", "ops"},
	}
	state := f.begin(t, p)
	id, err := p.Complete(context.Background(), callback(state), state)
	require.NoError(t, err)
	assert.Equal(t, &Identity{Subject: "user-42", Email: "alice@example.com", EmailVerified: true, Name: "Alice", Groups: []string{"dev", "ops"}}, id)

	// The email and the name missing in the ID token are taken from UserInfo.
	f.idTokenClaims = nil
	f.userInfo = map[string]interface{}{"sub": "user-42", "email": "bob@example.com", "email_verified": "true", "preferred_username": "bob"}
	state = f.begin(t, p)
	id, err = p.Complete(context.Background(), callback(state), state)
	require.NoError(t, err)
	assert.Equal(t, &Identity{Subject: "user-42", Email: "bob@example.com", EmailVerified: true, Name: "bob"}, id)

	// It is rejected if the subject of UserInfo differs from the ID token.
	f.userInfo = map[string]interface{}{"sub": "someone-else", "email": "eve@example.com"}
	state = f.begin(t, p)
	_, err = p.Complete(context.Background(), callback(state), state)
	assert.Equal(t, CodeIdPError, ErrorCode(err))

	// A mismatched nonce is an invalid state.
	f.idTokenClaims = map[string]interface{}{"email": "a@b.com", "name": "A"}
	state = f.begin(t, p)
	f.nonce = "forged"
	_, err = p.Complete(context.Background(), callback(state), state)
	assert.Equal(t, CodeInvalidState, ErrorCode(err))

	// Unverified email.
	f.idTokenClaims = map[string]interface{}{"email": "a@b.com", "name": "A", "email_verified": false}
	state = f.begin(t, p)
	id, err = p.Complete(context.Background(), callback(state), state)
	require.NoError(t, err)
	assert.False(t, id.EmailVerified)

	require.NoError(t, ProbeOIDC(context.Background(), f.URL))
	require.Error(t, ProbeOIDC(context.Background(), f.URL+"/nope"))
}
