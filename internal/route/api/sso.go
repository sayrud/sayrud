package api

import (
	stdcontext "context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/redis"
	"github.com/wuhan005/sayrud/internal/sso"
)

var SSO ssoRoute

type ssoRoute struct{}

const (
	ssoStateCookieName = "sayrud_sso_state"
	ssoStateCookiePath = "/_/auth/sso/"
	linkDonePath       = "/settings/security"
)

// ssoOptions returns the dependencies of the providers, the SAML assertion IDs are recorded in Redis against replays.
func ssoOptions(settings *db.SystemSettings) sso.Options {
	return sso.Options{
		BaseURL: settings.ExternalURL,
		MarkAssertion: func(ctx stdcontext.Context, key string, until time.Time) (bool, error) {
			return redis.SSOStates.MarkOnce(ctx, "saml:"+key, time.Until(until))
		},
	}
}

// safeRedirect only accepts the paths of this site to prevent open redirects.
func safeRedirect(redirect string) string {
	if strings.HasPrefix(redirect, "/") && !strings.HasPrefix(redirect, "//") && !strings.HasPrefix(redirect, "/\\") {
		return redirect
	}
	return "/"
}

func redirectTo(ctx context.Context, location string) error {
	http.Redirect(ctx.ResponseWriter(), ctx.Request().Request, location, http.StatusFound)
	return nil
}

// fail logs the reason and redirects to the sign-in page (the security settings in the link mode) with the error code in the sso_error query.
func fail(ctx context.Context, mode string, err error) error {
	code := sso.ErrorCode(err)
	entry := logrus.WithContext(ctx.Request().Context()).WithError(err).WithField("code", code)
	if code == sso.CodeIdPError || code == sso.CodeNotConfigured {
		entry.Warn("Third-party sign-in failed")
	} else {
		entry.Info("Third-party sign-in rejected")
	}
	page := "/login"
	if mode == sso.ModeLink {
		page = linkDonePath
	}
	return redirectTo(ctx, page+"?"+url.Values{"sso_error": {code}}.Encode())
}

func setStateCookie(ctx context.Context, value string, maxAge int, crossSite bool) {
	r := ctx.Request()
	secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	sameSite := http.SameSiteLaxMode
	// The SAML response is posted cross-site by the IdP page, only SameSite=None cookies are sent with it, which requires Secure.
	if crossSite && secure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(ctx.ResponseWriter(), &http.Cookie{
		Name:     ssoStateCookieName,
		Value:    value,
		Path:     ssoStateCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

// redirectProvider returns the enabled redirect provider of the slug, it is not configured if the external URL is not set.
func redirectProvider(ctx context.Context, settings *db.SystemSettings) (*db.AuthProvider, error) {
	p, err := db.AuthProviders.GetBySlug(ctx.Request().Context(), ctx.Param("slug"))
	if err != nil {
		if errors.Is(err, db.ErrAuthProviderNotFound) {
			return nil, sso.Errorf(sso.CodeProviderNotFound, err)
		}
		return nil, errors.Wrap(err, "get provider")
	}
	if !p.Enabled || !p.Type.Redirect() {
		return nil, sso.Errorf(sso.CodeProviderNotFound, errors.Errorf("provider %s is disabled or not a redirect provider", p.Slug))
	}
	if settings.ExternalURL == "" {
		return nil, sso.Errorf(sso.CodeNotConfigured, errors.New("external URL is not configured"))
	}
	return p, nil
}

// Start
// @Summary Start a third-party sign-in
// @Description Redirects the browser to the identity provider. In the link mode the signed-in user binds the third-party account. Failures redirect to the sign-in page (or the security settings in the link mode) with the sso_error query.
// @Param slug path string true "Provider slug"
// @Param mode query string false "login or link, defaults to login" Enums(login, link)
// @Param redirect query string false "Path to go after signing in"
// @Success 302 "Redirect to the identity provider"
// @ID startSSO
// @Router /auth/sso/{slug}/start [get]
func (ssoRoute) Start(ctx context.Context) error {
	mode := sso.ModeLogin
	if ctx.Query("mode") == sso.ModeLink {
		mode = sso.ModeLink
	}
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	provider, err := redirectProvider(ctx, settings)
	if err != nil {
		return fail(ctx, mode, err)
	}

	var userID int64
	if mode == sso.ModeLink {
		user, _, key, err := sessionUser(ctx)
		if err != nil {
			return fail(ctx, mode, err)
		}
		if key != "" {
			return redirectTo(ctx, "/login?"+url.Values{"redirect": {linkDonePath}}.Encode())
		}
		userID = user.ID
	}

	state, err := sso.NewState(provider.ID, mode, safeRedirect(ctx.Query("redirect")), userID)
	if err != nil {
		return fail(ctx, mode, err)
	}
	p, err := sso.NewRedirect(provider, ssoOptions(settings))
	if err != nil {
		return fail(ctx, mode, sso.Errorf(sso.CodeNotConfigured, err))
	}
	location, err := p.Begin(ctx.Request().Context(), state)
	if err != nil {
		return fail(ctx, mode, sso.Errorf(sso.CodeIdPError, err))
	}
	raw, err := sso.EncodeState(state)
	if err != nil {
		return fail(ctx, mode, err)
	}
	if err := redis.SSOStates.Save(ctx.Request().Context(), state.State, raw, sso.StateTTL); err != nil {
		return fail(ctx, mode, errors.Wrap(err, "save state"))
	}
	setStateCookie(ctx, state.State, int(sso.StateTTL.Seconds()), provider.Type == db.AuthProviderSAML)
	return redirectTo(ctx, location)
}

// takeState takes the one-time state and checks that it belongs to the provider and matches the browser cookie, a missing cookie is allowed if requireCookie is false.
func takeState(ctx context.Context, provider *db.AuthProvider, token string, requireCookie bool) (*sso.State, error) {
	raw, err := redis.SSOStates.Take(ctx.Request().Context(), token)
	if err != nil {
		if errors.Is(err, redis.ErrSSOStateNotFound) {
			return nil, sso.Errorf(sso.CodeInvalidState, err)
		}
		return nil, errors.Wrap(err, "take state")
	}
	state, err := sso.DecodeState(token, raw)
	if err != nil {
		return nil, err
	}
	if state.ProviderID != provider.ID {
		return nil, sso.Errorf(sso.CodeInvalidState, errors.New("state belongs to another provider"))
	}
	cookie, err := ctx.Request().Cookie(ssoStateCookieName)
	switch {
	case err == nil && cookie.Value != token:
		return nil, sso.Errorf(sso.CodeInvalidState, errors.New("state cookie mismatch"))
	case err != nil && requireCookie:
		return nil, sso.Errorf(sso.CodeInvalidState, errors.New("missing state cookie"))
	}
	return state, nil
}

// Callback
// @Summary OAuth 2.0 / OIDC callback
// @Description Signs in (or binds the account in the link mode) and redirects to the saved path. Failures redirect with the sso_error query.
// @Param slug path string true "Provider slug"
// @Param state query string true "State"
// @Param code query string false "Authorization code"
// @Success 302 "Redirect after signing in"
// @ID ssoCallback
// @Router /auth/sso/{slug}/callback [get]
func (ssoRoute) Callback(ctx context.Context) error {
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	setStateCookie(ctx, "", -1, false)
	provider, err := redirectProvider(ctx, settings)
	if err != nil {
		return fail(ctx, sso.ModeLogin, err)
	}
	state, err := takeState(ctx, provider, ctx.Query("state"), true)
	if err != nil {
		return fail(ctx, sso.ModeLogin, err)
	}
	p, err := sso.NewRedirect(provider, ssoOptions(settings))
	if err != nil {
		return fail(ctx, state.Mode, sso.Errorf(sso.CodeNotConfigured, err))
	}
	id, err := p.Complete(ctx.Request().Context(), ctx.Request().Request, state)
	if err != nil {
		return fail(ctx, state.Mode, err)
	}
	return finishSSO(ctx, settings, provider, state, id)
}

// ACS
// @Summary SAML Assertion Consumer Service
// @Description Receives the SAML response posted by the identity provider, signs in and redirects. Failures redirect with the sso_error query.
// @Accept x-www-form-urlencoded
// @Param slug path string true "Provider slug"
// @Success 302 "Redirect after signing in"
// @ID ssoACS
// @Router /auth/sso/{slug}/acs [post]
func (ssoRoute) ACS(ctx context.Context) error {
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	setStateCookie(ctx, "", -1, true)
	provider, err := redirectProvider(ctx, settings)
	if err != nil {
		return fail(ctx, sso.ModeLogin, err)
	}
	if provider.Type != db.AuthProviderSAML {
		return fail(ctx, sso.ModeLogin, sso.Errorf(sso.CodeProviderNotFound, errors.New("not a SAML provider")))
	}
	if err := ctx.Request().ParseForm(); err != nil {
		return fail(ctx, sso.ModeLogin, sso.Errorf(sso.CodeIdPError, err))
	}

	// Over HTTP the browser does not send SameSite=Lax cookies with cross-site POSTs, only the one-time state and InResponseTo are checked.
	requireCookie := strings.HasPrefix(settings.ExternalURL, "https://")
	var state *sso.State
	if relayState := ctx.Request().PostForm.Get("RelayState"); relayState != "" {
		state, err = takeState(ctx, provider, relayState, requireCookie)
	}
	if state == nil {
		config, _, loadErr := sso.LoadConfig(provider)
		switch {
		case loadErr != nil:
			return fail(ctx, sso.ModeLogin, sso.Errorf(sso.CodeNotConfigured, loadErr))
		case config.AllowIdPInitiated:
			err = nil
		case err == nil:
			err = sso.Errorf(sso.CodeInvalidState, errors.New("missing RelayState"))
		}
		if err != nil {
			return fail(ctx, sso.ModeLogin, err)
		}
	}

	mode := sso.ModeLogin
	if state != nil {
		mode = state.Mode
	}
	p, err := sso.NewRedirect(provider, ssoOptions(settings))
	if err != nil {
		return fail(ctx, mode, sso.Errorf(sso.CodeNotConfigured, err))
	}
	id, err := p.Complete(ctx.Request().Context(), ctx.Request().Request, state)
	if err != nil {
		return fail(ctx, mode, err)
	}
	if state == nil {
		state = &sso.State{ProviderID: provider.ID, Mode: sso.ModeLogin, Redirect: "/"}
	}
	return finishSSO(ctx, settings, provider, state, id)
}

func finishSSO(ctx context.Context, settings *db.SystemSettings, provider *db.AuthProvider, state *sso.State, id *sso.Identity) error {
	var linkUserID int64
	if state.Mode == sso.ModeLink {
		linkUserID = state.UserID
	}
	user, err := sso.Resolve(ctx.Request().Context(), provider, id, linkUserID)
	if err != nil {
		return fail(ctx, state.Mode, err)
	}
	if state.Mode == sso.ModeLink {
		return redirectTo(ctx, linkDonePath+"?sso_linked="+url.QueryEscape(provider.Slug))
	}
	if err := startSession(ctx, user, settings.SessionTTL()); err != nil {
		return fail(ctx, state.Mode, err)
	}
	return redirectTo(ctx, safeRedirect(state.Redirect))
}

// Metadata
// @Summary SAML service provider metadata
// @Description The entity ID of the service provider is the URL of this metadata. Available before the provider is enabled.
// @Produce xml
// @Param slug path string true "Provider slug"
// @Success 200 {string} string "SP metadata XML"
// @Failure 404 {string} string "Provider not found or not SAML"
// @Failure 500 {string} string "Internal server error"
// @ID ssoMetadata
// @Router /auth/sso/{slug}/metadata [get]
func (ssoRoute) Metadata(ctx context.Context) error {
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	provider, err := db.AuthProviders.GetBySlug(ctx.Request().Context(), ctx.Param("slug"))
	if err != nil {
		if errors.Is(err, db.ErrAuthProviderNotFound) {
			return ctx.ApiError(http.StatusNotFound, "sso::provider_not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get auth provider")
		return ctx.ApiServerError()
	}
	if provider.Type != db.AuthProviderSAML {
		return ctx.ApiError(http.StatusNotFound, "sso::provider_not_found")
	}
	if settings.ExternalURL == "" {
		return ctx.ApiError(http.StatusBadRequest, "sso::external_url_required")
	}
	metadata, err := sso.SAMLMetadata(provider, ssoOptions(settings))
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to build SAML metadata")
		return ctx.ApiServerError()
	}
	ctx.ResponseWriter().Header().Set("Content-Type", "application/samlmetadata+xml")
	ctx.ResponseWriter().WriteHeader(http.StatusOK)
	_, _ = ctx.ResponseWriter().Write(metadata)
	return nil
}

// LDAPSignIn
// @Summary Sign in with LDAP
// @Accept json
// @Produce json
// @Param slug path string true "Provider slug"
// @Param data body form.LDAPSignIn true "Credential"
// @Success 200 {object} dto.Profile
// @Failure 400 {string} string "Invalid request body"
// @Failure 401 {string} string "Wrong user name or password"
// @Failure 403 {string} string "The account is not allowed, not linked or disabled"
// @Failure 404 {string} string "Provider not found"
// @Failure 429 {string} string "Too many attempts"
// @Failure 500 {string} string "Internal server error"
// @Failure 502 {string} string "LDAP server unavailable"
// @ID ldapSignIn
// @Router /auth/ldap/{slug}/sign-in [post]
func (ssoRoute) LDAPSignIn(ctx context.Context, f form.LDAPSignIn) error {
	c := ctx.Request().Context()
	provider, err := db.AuthProviders.GetBySlug(c, ctx.Param("slug"))
	if err != nil && !errors.Is(err, db.ErrAuthProviderNotFound) {
		logrus.WithContext(c).WithError(err).Error("Failed to get auth provider")
		return ctx.ApiServerError()
	}
	if provider == nil || !provider.Enabled || provider.Type != db.AuthProviderLDAP {
		return ctx.ApiError(http.StatusNotFound, "sso::provider_not_found")
	}

	attemptKey := "ldap:" + provider.Slug + ":" + strings.ToLower(strings.TrimSpace(f.UserName))
	if err := redis.AuthAttempts.CheckSignIn(c, ctx.IP(), attemptKey); err != nil {
		if errors.Is(err, redis.ErrTooManyAttempts) {
			return ctx.ApiError(http.StatusTooManyRequests, "common::too_many_attempts")
		}
		logrus.WithContext(c).WithError(err).Warn("Failed to check sign-in attempts")
	}

	p, err := sso.NewPassword(provider)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to load LDAP provider")
		return ctx.ApiError(http.StatusInternalServerError, "sso::sso_not_configured")
	}
	id, err := p.Authenticate(c, f.UserName, f.Password)
	if err != nil {
		if errors.Is(err, sso.ErrBadCredential) {
			logrus.WithContext(c).WithError(err).Info("LDAP sign-in rejected")
			return ctx.ApiError(http.StatusUnauthorized, "auth::wrong_username_or_password")
		}
		logrus.WithContext(c).WithError(err).Error("Failed to authenticate with LDAP")
		return ctx.ApiError(http.StatusBadGateway, "sso::ldap_unavailable")
	}
	if err := redis.AuthAttempts.ResetSignIn(c, attemptKey); err != nil {
		logrus.WithContext(c).WithError(err).Warn("Failed to reset sign-in attempts")
	}

	user, err := sso.Resolve(c, provider, id, 0)
	if err != nil {
		var e *sso.Error
		if errors.As(err, &e) {
			logrus.WithContext(c).WithError(err).Info("LDAP sign-in rejected")
			return ctx.ApiError(http.StatusForbidden, "sso::"+e.Code)
		}
		logrus.WithContext(c).WithError(err).Error("Failed to resolve LDAP user")
		return ctx.ApiServerError()
	}
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	if err := startSession(ctx, user, settings.SessionTTL()); err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to start session")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToProfile(user))
}

// ListIdentities
// @Summary List the third-party accounts bound to the signed-in user
// @Produce json
// @Success 200 {array} dto.UserIdentity
// @Failure 401 {string} string "Not signed in"
// @Failure 500 {string} string "Internal server error"
// @ID listIdentities
// @Router /auth/identities [get]
func (ssoRoute) ListIdentities(ctx context.Context, user *db.User) error {
	c := ctx.Request().Context()
	identities, err := db.UserIdentities.ListByUserID(c, user.ID)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list identities")
		return ctx.ApiServerError()
	}
	providers, err := db.AuthProviders.List(c)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list auth providers")
		return ctx.ApiServerError()
	}
	byID := make(map[int64]*db.AuthProvider, len(providers))
	for _, p := range providers {
		byID[p.ID] = p
	}
	resp := make([]*dto.UserIdentity, 0, len(identities))
	for _, i := range identities {
		if p, ok := byID[i.ProviderID]; ok {
			resp = append(resp, dto.ToUserIdentity(i, p))
		}
	}
	return ctx.ApiSuccess(resp)
}

// DeleteIdentity
// @Summary Unbind a third-party account of the signed-in user
// @Description The last way to sign in can not be unbound if the user has no password.
// @Param identityID path int true "Identity ID"
// @Success 204 "No Content"
// @Failure 401 {string} string "Not signed in"
// @Failure 404 {string} string "Identity not found"
// @Failure 409 {string} string "It is the last way to sign in"
// @Failure 500 {string} string "Internal server error"
// @ID deleteIdentity
// @Router /auth/identities/{identityID} [delete]
func (ssoRoute) DeleteIdentity(ctx context.Context, user *db.User) error {
	c := ctx.Request().Context()
	if !user.HasPassword() {
		count, err := db.UserIdentities.CountByUserID(c, user.ID)
		if err != nil {
			logrus.WithContext(c).WithError(err).Error("Failed to count identities")
			return ctx.ApiServerError()
		}
		if count <= 1 {
			return ctx.ApiError(http.StatusConflict, "account::last_sign_in_method")
		}
	}
	if err := db.UserIdentities.Delete(c, user.ID, ctx.ParamInt64("identityID")); err != nil {
		if errors.Is(err, db.ErrUserIdentityNotFound) {
			return ctx.ApiError(http.StatusNotFound, "account::identity_not_found")
		}
		logrus.WithContext(c).WithError(err).Error("Failed to delete identity")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}
