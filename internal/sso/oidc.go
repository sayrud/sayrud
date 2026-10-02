package sso

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/wuhan005/sayrud/internal/db"
)

type oidcProvider struct {
	config      Config
	secrets     Secrets
	redirectURL string
	client      *http.Client
}

func newOIDC(p *db.AuthProvider, c Config, s Secrets, opts Options) *oidcProvider {
	return &oidcProvider{config: c, secrets: s, redirectURL: CallbackURL(opts.BaseURL, p.Slug), client: opts.httpClient()}
}

type cachedDiscovery struct {
	provider  *oidc.Provider
	expiresAt time.Time
}

var (
	discoveryMu    sync.Mutex
	discoveryCache = map[string]cachedDiscovery{}
)

const discoveryTTL = 10 * time.Minute

// discover returns the discovery of the issuer, cached for 10 minutes to reuse its JWKS cache.
func discover(ctx context.Context, client *http.Client, issuer string) (*oidc.Provider, error) {
	discoveryMu.Lock()
	cached, ok := discoveryCache[issuer]
	discoveryMu.Unlock()
	if ok && time.Now().Before(cached.expiresAt) {
		return cached.provider, nil
	}
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, client), issuer)
	if err != nil {
		return nil, errors.Wrap(err, "discover")
	}
	discoveryMu.Lock()
	discoveryCache[issuer] = cachedDiscovery{provider: provider, expiresAt: time.Now().Add(discoveryTTL)}
	discoveryMu.Unlock()
	return provider, nil
}

// ProbeOIDC fetches the discovery document of the issuer to test the connection in the admin console.
func ProbeOIDC(ctx context.Context, issuer string) error {
	_, err := oidc.NewProvider(oidc.ClientContext(ctx, defaultHTTPClient), issuer)
	return err
}

func (p *oidcProvider) oauth2Config(provider *oidc.Provider) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.config.ClientID,
		ClientSecret: p.secrets.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  p.redirectURL,
		Scopes:       p.config.Scopes,
	}
}

func (p *oidcProvider) Begin(ctx context.Context, state *State) (string, error) {
	provider, err := discover(ctx, p.client, p.config.Issuer)
	if err != nil {
		return "", err
	}
	nonce, err := randomToken()
	if err != nil {
		return "", err
	}
	state.Verifier = oauth2.GenerateVerifier()
	state.Nonce = nonce
	return p.oauth2Config(provider).AuthCodeURL(state.State, oauth2.S256ChallengeOption(state.Verifier), oidc.Nonce(nonce)), nil
}

func (p *oidcProvider) Complete(ctx context.Context, r *http.Request, state *State) (*Identity, error) {
	code, err := callbackCode(r)
	if err != nil {
		return nil, err
	}
	provider, err := discover(ctx, p.client, p.config.Issuer)
	if err != nil {
		return nil, Errorf(CodeIdPError, err)
	}
	ctx = oidc.ClientContext(ctx, p.client)
	oc := p.oauth2Config(provider)
	token, err := oc.Exchange(ctx, code, oauth2.VerifierOption(state.Verifier))
	if err != nil {
		return nil, Errorf(CodeIdPError, errors.Wrap(err, "exchange token"))
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, Errorf(CodeIdPError, errors.New("missing id_token"))
	}
	idToken, err := provider.Verifier(&oidc.Config{ClientID: p.config.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, Errorf(CodeIdPError, errors.Wrap(err, "verify id_token"))
	}
	if idToken.Nonce != state.Nonce {
		return nil, Errorf(CodeInvalidState, errors.New("nonce mismatch"))
	}

	claims := map[string]interface{}{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, Errorf(CodeIdPError, errors.Wrap(err, "parse claims"))
	}
	id := p.identityFromClaims(idToken.Subject, claims)

	if (id.Email == "" || id.Name == "") && provider.UserInfoEndpoint() != "" {
		info, err := provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
		if err != nil {
			return nil, Errorf(CodeIdPError, errors.Wrap(err, "get user info"))
		}
		if info.Subject != idToken.Subject {
			return nil, Errorf(CodeIdPError, errors.New("user info subject mismatch"))
		}
		extra := map[string]interface{}{}
		if err := info.Claims(&extra); err != nil {
			return nil, Errorf(CodeIdPError, errors.Wrap(err, "parse user info claims"))
		}
		for k, v := range extra {
			if _, exists := claims[k]; !exists {
				claims[k] = v
			}
		}
		id = p.identityFromClaims(idToken.Subject, claims)
	}
	return id, nil
}

func (p *oidcProvider) identityFromClaims(subject string, claims map[string]interface{}) *Identity {
	name := claimString(claims, p.config.NameClaim)
	if name == "" {
		name = claimString(claims, "preferred_username")
	}
	return &Identity{
		Subject:       subject,
		Email:         db.NormalizeEmail(claimString(claims, "email")),
		EmailVerified: claimString(claims, "email_verified") == "true",
		Name:          name,
		Groups:        claimStrings(claims, p.config.GroupsClaim),
	}
}

// claimString looks up the whole claim name first (namespaced claims may contain dots), then the dot path.
func claimString(claims map[string]interface{}, name string) string {
	if v, ok := claims[name]; ok {
		return scalarString(v)
	}
	return pathString(claims, name)
}

func claimStrings(claims map[string]interface{}, name string) []string {
	if name == "" {
		return nil
	}
	if v, ok := claims[name]; ok {
		return pathStrings(map[string]interface{}{"v": v}, "v")
	}
	return pathStrings(claims, name)
}
