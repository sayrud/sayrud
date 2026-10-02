package sso

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/wuhan005/sayrud/internal/db"
)

// Modes of the redirect sign-in.
const (
	ModeLogin = "login"
	ModeLink  = "link"
)

// State is the context of a redirect sign-in, saved once in Redis by the state.
type State struct {
	State      string `json:"-"`
	ProviderID int64  `json:"providerID"`
	// Redirect is the path of this site to go after signing in.
	Redirect string `json:"redirect"`
	Mode     string `json:"mode"`
	// UserID is the user who binds the account in the link mode.
	UserID int64 `json:"userID,omitempty"`
	// Verifier is the PKCE code verifier.
	Verifier string `json:"verifier,omitempty"`
	// Nonce is the OIDC nonce.
	Nonce string `json:"nonce,omitempty"`
	// RequestID is the ID of the SAML AuthnRequest.
	RequestID string `json:"requestID,omitempty"`
}

// StateTTL is the lifetime of a state.
const StateTTL = 10 * time.Minute

// NewState returns a sign-in context with a random state.
func NewState(providerID int64, mode, redirect string, userID int64) (*State, error) {
	token, err := randomToken()
	if err != nil {
		return nil, err
	}
	return &State{State: token, ProviderID: providerID, Mode: mode, Redirect: redirect, UserID: userID}, nil
}

// EncodeState serializes the state to save in Redis.
func EncodeState(s *State) ([]byte, error) {
	return json.Marshal(s)
}

// DecodeState deserializes the state from Redis.
func DecodeState(token string, data []byte) (*State, error) {
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, errors.Wrap(err, "unmarshal state")
	}
	s.State = token
	return &s, nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.Wrap(err, "generate random token")
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// RedirectProvider signs in by redirecting to the identity provider: OAuth 2.0, OIDC and SAML.
type RedirectProvider interface {
	// Begin returns the URL of the identity provider and writes PKCE, nonce and so on into the state.
	Begin(ctx context.Context, state *State) (string, error)
	// Complete verifies the callback and returns the account, the state is nil for IdP-initiated SAML sign-ins.
	Complete(ctx context.Context, r *http.Request, state *State) (*Identity, error)
}

// PasswordProvider signs in with a user name and password form: LDAP.
type PasswordProvider interface {
	// Authenticate verifies the user name and password, it returns ErrBadCredential if the user does not exist or the password is wrong.
	Authenticate(ctx context.Context, username, password string) (*Identity, error)
}

// Options are the dependencies of the providers.
type Options struct {
	// BaseURL is the external URL of the site without the trailing /.
	BaseURL string
	// HTTPClient requests the identity provider, the default client with a 10-second timeout is used if nil.
	HTTPClient *http.Client
	// MarkAssertion records the SAML assertion ID until the time against replays, it returns false if recorded.
	MarkAssertion func(ctx context.Context, key string, until time.Time) (bool, error)
}

func (o Options) httpClient() *http.Client {
	if o.HTTPClient != nil {
		return o.HTTPClient
	}
	return defaultHTTPClient
}

var defaultHTTPClient = &http.Client{Timeout: 10 * time.Second}

// LoadConfig parses the config of the provider and decrypts its secrets.
func LoadConfig(p *db.AuthProvider) (Config, Secrets, error) {
	var c Config
	if len(p.Config) > 0 {
		if err := json.Unmarshal(p.Config, &c); err != nil {
			return c, Secrets{}, errors.Wrap(err, "unmarshal config")
		}
	}
	s, err := DecodeSecrets(p.Secrets)
	if err != nil {
		return c, s, errors.Wrap(err, "decode secrets")
	}
	return c.Normalize(p.Type), s, nil
}

// NewRedirect builds a redirect provider.
func NewRedirect(p *db.AuthProvider, opts Options) (RedirectProvider, error) {
	c, s, err := LoadConfig(p)
	if err != nil {
		return nil, err
	}
	switch p.Type {
	case db.AuthProviderOAuth2:
		return newOAuth2(p, c, s, opts), nil
	case db.AuthProviderOIDC:
		return newOIDC(p, c, s, opts), nil
	case db.AuthProviderSAML:
		return newSAML(p, c, s, opts)
	}
	return nil, errors.Errorf("%s is not a redirect provider", p.Type)
}

// NewPassword builds a password provider.
func NewPassword(p *db.AuthProvider) (PasswordProvider, error) {
	c, s, err := LoadConfig(p)
	if err != nil {
		return nil, err
	}
	if p.Type != db.AuthProviderLDAP {
		return nil, errors.Errorf("%s is not a password provider", p.Type)
	}
	return newLDAP(c, s), nil
}

func ssoPath(baseURL, slug, action string) string {
	return baseURL + "/_/auth/sso/" + slug + "/" + action
}

// CallbackURL returns the callback URL of OAuth 2.0 and OIDC.
func CallbackURL(baseURL, slug string) string { return ssoPath(baseURL, slug, "callback") }

// ACSURL returns the SAML Assertion Consumer Service URL.
func ACSURL(baseURL, slug string) string { return ssoPath(baseURL, slug, "acs") }

// MetadataURL returns the SAML SP metadata URL, which is also the SP entity ID.
func MetadataURL(baseURL, slug string) string { return ssoPath(baseURL, slug, "metadata") }
