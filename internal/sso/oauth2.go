package sso

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/wuhan005/sayrud/internal/db"
)

type oauth2Provider struct {
	config Config
	oauth  *oauth2.Config
	client *http.Client
}

func newOAuth2(p *db.AuthProvider, c Config, s Secrets, opts Options) *oauth2Provider {
	return &oauth2Provider{
		config: c,
		oauth: &oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: s.ClientSecret,
			Endpoint:     oauth2.Endpoint{AuthURL: c.AuthURL, TokenURL: c.TokenURL},
			RedirectURL:  CallbackURL(opts.BaseURL, p.Slug),
			Scopes:       c.Scopes,
		},
		client: opts.httpClient(),
	}
}

func (p *oauth2Provider) Begin(_ context.Context, state *State) (string, error) {
	state.Verifier = oauth2.GenerateVerifier()
	return p.oauth.AuthCodeURL(state.State, oauth2.S256ChallengeOption(state.Verifier)), nil
}

// callbackCode returns the authorization code of the callback, or CodeIdPError if the IdP returns an error or the code is missing.
func callbackCode(r *http.Request) (string, error) {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		return "", Errorf(CodeIdPError, errors.Errorf("idp returned %s: %s", e, q.Get("error_description")))
	}
	code := q.Get("code")
	if code == "" {
		return "", Errorf(CodeIdPError, errors.New("missing code"))
	}
	return code, nil
}

func (p *oauth2Provider) Complete(ctx context.Context, r *http.Request, state *State) (*Identity, error) {
	code, err := callbackCode(r)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, p.client)
	token, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(state.Verifier))
	if err != nil {
		return nil, Errorf(CodeIdPError, errors.Wrap(err, "exchange token"))
	}

	raw, err := p.getJSON(ctx, p.config.UserInfoURL, token)
	if err != nil {
		return nil, Errorf(CodeIdPError, errors.Wrap(err, "get user info"))
	}
	info, err := decodeJSON(raw)
	if err != nil {
		return nil, Errorf(CodeIdPError, err)
	}
	id := &Identity{
		Subject:       pathString(info, p.config.SubjectPath),
		Email:         db.NormalizeEmail(pathString(info, p.config.EmailPath)),
		EmailVerified: p.config.TrustEmail,
		Name:          pathString(info, p.config.NamePath),
	}
	if p.config.GroupsPath != "" {
		id.Groups = pathStrings(info, p.config.GroupsPath)
	}
	if id.Subject == "" {
		return nil, Errorf(CodeIdPError, errors.Errorf("subject %q not found in user info", p.config.SubjectPath))
	}

	if p.config.EmailsURL != "" {
		raw, err := p.getJSON(ctx, p.config.EmailsURL, token)
		if err != nil {
			return nil, Errorf(CodeIdPError, errors.Wrap(err, "get emails"))
		}
		email, verified, err := pickEmail(raw)
		if err != nil {
			return nil, Errorf(CodeIdPError, err)
		}
		if email != "" {
			id.Email, id.EmailVerified = email, verified
		} else {
			id.EmailVerified = false
		}
	}
	return id, nil
}

const maxResponseSize = 1 << 20

func (p *oauth2Provider) getJSON(ctx context.Context, endpoint string, token *oauth2.Token) ([]byte, error) {
	if _, err := url.Parse(endpoint); err != nil {
		return nil, errors.Wrap(err, "parse URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.Wrap(err, "new request")
	}
	req.Header.Set("Accept", "application/json")
	token.SetAuthHeader(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, errors.Wrap(err, "read body")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("unexpected status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

// pickEmail picks the primary and verified email from [{email, primary, verified}], then any verified one, or else the primary one with false.
func pickEmail(raw []byte) (string, bool, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return "", false, err
	}
	list, ok := v.([]interface{})
	if !ok {
		return "", false, errors.New("emails response is not an array")
	}
	var primary, verified string
	for _, item := range list {
		email := db.NormalizeEmail(pathString(item, "email"))
		if email == "" {
			continue
		}
		isPrimary := pathString(item, "primary") == "true"
		isVerified := pathString(item, "verified") == "true"
		switch {
		case isPrimary && isVerified:
			return email, true, nil
		case isVerified && verified == "":
			verified = email
		case isPrimary && primary == "":
			primary = email
		}
	}
	if verified != "" {
		return verified, true, nil
	}
	return primary, false, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
