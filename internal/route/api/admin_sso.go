package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/sso"
)

// AuthProviderer maps the provider of the providerID path parameter as *db.AuthProvider, and responds 404 if not found.
func (adminRoute) AuthProviderer(ctx context.Context) error {
	p, err := db.AuthProviders.GetByID(ctx.Request().Context(), ctx.ParamInt64("providerID"))
	if err != nil {
		if errors.Is(err, db.ErrAuthProviderNotFound) {
			return ctx.ApiError(http.StatusNotFound, "sso::provider_not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get auth provider")
		return ctx.ApiServerError()
	}
	ctx.Map(p)
	return nil
}

// ListAuthProviders
// @Summary List the sign-in methods
// @Description Requires the admin. The secrets are not returned.
// @Produce json
// @Success 200 {object} dto.ListAdminAuthProvidersResp
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID listAdminAuthProviders
// @Router /admin/auth-providers [get]
func (adminRoute) ListAuthProviders(ctx context.Context) error {
	c := ctx.Request().Context()
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	providers, err := db.AuthProviders.List(c)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list auth providers")
		return ctx.ApiServerError()
	}
	ids := make([]int64, 0, len(providers))
	for _, p := range providers {
		ids = append(ids, p.ID)
	}
	counts, err := db.UserIdentities.CountByProviderIDs(c, ids)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to count identities")
		return ctx.ApiServerError()
	}
	resp := dto.ListAdminAuthProvidersResp{
		Providers:    make([]*dto.AdminAuthProvider, 0, len(providers)),
		SecretsReady: sso.SecretsReady(),
		ExternalURL:  settings.ExternalURL,
	}
	for _, p := range providers {
		resp.Providers = append(resp.Providers, dto.ToAdminAuthProvider(p, counts[p.ID], settings.ExternalURL))
	}
	return ctx.ApiSuccess(resp)
}

// applyProvider validates the form and writes it into p, secrets are the saved (or generated) ones, it returns the *i18n.Error to be translated.
func applyProvider(p *db.AuthProvider, f form.SaveAuthProvider, secrets sso.Secrets, settings *db.SystemSettings) error {
	name := strings.TrimSpace(f.Name)
	if name == "" || utf8.RuneCountInString(name) > 32 {
		return i18n.Errorf("sso::name_required")
	}
	icon := strings.TrimSpace(f.Icon)
	if len(icon) > 32 {
		icon = icon[:32]
	}

	if f.ClientSecret != "" {
		secrets.ClientSecret = f.ClientSecret
	}
	if f.BindPassword != "" {
		secrets.BindPassword = f.BindPassword
	}
	// Only keep the secrets used by the type.
	switch p.Type {
	case db.AuthProviderOAuth2, db.AuthProviderOIDC:
		secrets = sso.Secrets{ClientSecret: secrets.ClientSecret}
	case db.AuthProviderLDAP:
		secrets = sso.Secrets{BindPassword: secrets.BindPassword}
	case db.AuthProviderSAML:
		secrets = sso.Secrets{SPPrivateKey: secrets.SPPrivateKey}
	}

	config := f.Config.Normalize(p.Type)
	if err := config.Validate(p.Type, secrets); err != nil {
		return err
	}
	if f.Enabled && p.Type.Redirect() && settings.ExternalURL == "" {
		return i18n.Errorf("sso::external_url_required")
	}
	rawConfig, err := jsonMarshal(config)
	if err != nil {
		return err
	}
	sealed, err := sso.EncodeSecrets(secrets)
	if err != nil {
		return i18n.Errorf("sso::secret_key_missing")
	}

	p.Name = name
	p.Icon = icon
	p.Enabled = f.Enabled
	p.Config = rawConfig
	p.Secrets = sealed
	p.AutoCreateUser = f.AutoCreateUser
	p.LinkByEmail = f.LinkByEmail
	p.AllowedEmailDomains = sso.NormalizeDomains(f.AllowedEmailDomains)
	p.AllowedGroups = sso.NormalizeGroups(f.AllowedGroups)
	return nil
}

func jsonMarshal(v interface{}) (datatypes.JSON, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, errors.Wrap(err, "marshal")
	}
	return datatypes.JSON(raw), nil
}

// ensureNotLastEnabled reports false if password sign-in is off and it is the last enabled sign-in method, which can not be disabled or deleted.
func ensureNotLastEnabled(ctx context.Context, settings *db.SystemSettings) (bool, error) {
	if settings.AllowPasswordSignIn {
		return true, nil
	}
	count, err := db.AuthProviders.CountEnabled(ctx.Request().Context())
	if err != nil {
		return false, err
	}
	return count > 1, nil
}

// CreateAuthProvider
// @Summary Create a sign-in method
// @Description Requires the admin and auth.secret_key in the config file. The SAML service provider certificate is generated automatically.
// @Accept json
// @Produce json
// @Param data body form.CreateAuthProvider true "Sign-in method"
// @Success 200 {object} dto.AdminAuthProvider
// @Failure 400 {string} string "Invalid config"
// @Failure 403 {string} string "Not an admin"
// @Failure 409 {string} string "The slug has been used"
// @Failure 500 {string} string "Internal server error"
// @ID createAdminAuthProvider
// @Router /admin/auth-providers [post]
func (adminRoute) CreateAuthProvider(ctx context.Context, f form.CreateAuthProvider) error {
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	if !sso.SecretsReady() {
		return ctx.ApiError(http.StatusBadRequest, "sso::secret_key_missing")
	}
	slug := strings.TrimSpace(f.Slug)
	if !sso.ValidSlug(slug) {
		return ctx.ApiError(http.StatusBadRequest, "sso::invalid_slug")
	}
	typ := db.AuthProviderType(f.Type)
	if !typ.Valid() {
		return ctx.ApiError(http.StatusBadRequest, "sso::unsupported_type")
	}

	p := &db.AuthProvider{Slug: slug, Type: typ}
	var secrets sso.Secrets
	if typ == db.AuthProviderSAML {
		cert, key, err := sso.GenerateSPCertificate(slug)
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to generate SP certificate")
			return ctx.ApiServerError()
		}
		f.Config.SPCertificate = cert
		secrets.SPPrivateKey = key
	}
	if err := applyProvider(p, f.SaveAuthProvider, secrets, settings); err != nil {
		return ctx.ApiErrorFrom(http.StatusBadRequest, err)
	}
	if err := db.AuthProviders.Create(ctx.Request().Context(), p); err != nil {
		if errors.Is(err, db.ErrAuthProviderSlugTaken) {
			return ctx.ApiError(http.StatusConflict, "sso::slug_taken")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create auth provider")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToAdminAuthProvider(p, 0, settings.ExternalURL))
}

// UpdateAuthProvider
// @Summary Update a sign-in method
// @Description Requires the admin. The slug and the type can not be changed, the empty secrets keep the saved ones.
// @Accept json
// @Produce json
// @Param providerID path int true "Provider ID"
// @Param data body form.UpdateAuthProvider true "Sign-in method"
// @Success 200 {object} dto.AdminAuthProvider
// @Failure 400 {string} string "Invalid config"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Provider not found"
// @Failure 409 {string} string "It is the last enabled sign-in method while password sign-in is disabled"
// @Failure 500 {string} string "Internal server error"
// @ID updateAdminAuthProvider
// @Router /admin/auth-providers/{providerID} [put]
func (adminRoute) UpdateAuthProvider(ctx context.Context, p *db.AuthProvider, f form.UpdateAuthProvider) error {
	c := ctx.Request().Context()
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	if !sso.SecretsReady() {
		return ctx.ApiError(http.StatusBadRequest, "sso::secret_key_missing")
	}
	saved, secrets, err := sso.LoadConfig(p)
	if err != nil {
		// The secrets can not be decrypted after the key changes, they must be filled in again.
		logrus.WithContext(c).WithError(err).Warn("Failed to load saved auth provider config")
		secrets = sso.Secrets{}
	}
	if p.Type == db.AuthProviderSAML {
		f.Config.SPCertificate = saved.SPCertificate
		if secrets.SPPrivateKey == "" || saved.SPCertificate == "" {
			cert, key, err := sso.GenerateSPCertificate(p.Slug)
			if err != nil {
				logrus.WithContext(c).WithError(err).Error("Failed to generate SP certificate")
				return ctx.ApiServerError()
			}
			f.Config.SPCertificate, secrets.SPPrivateKey = cert, key
		}
	}

	wasEnabled := p.Enabled
	if err := applyProvider(p, f.SaveAuthProvider, secrets, settings); err != nil {
		return ctx.ApiErrorFrom(http.StatusBadRequest, err)
	}
	if wasEnabled && !p.Enabled {
		ok, err := ensureNotLastEnabled(ctx, settings)
		if err != nil {
			logrus.WithContext(c).WithError(err).Error("Failed to count enabled auth providers")
			return ctx.ApiServerError()
		}
		if !ok {
			return ctx.ApiError(http.StatusConflict, "sso::last_enabled_provider")
		}
	}
	if err := db.AuthProviders.Update(c, p); err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to update auth provider")
		return ctx.ApiServerError()
	}
	counts, err := db.UserIdentities.CountByProviderIDs(c, []int64{p.ID})
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to count identities")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToAdminAuthProvider(p, counts[p.ID], settings.ExternalURL))
}

// DeleteAuthProvider
// @Summary Delete a sign-in method
// @Description Requires the admin. The bound third-party accounts of the users are unbound.
// @Param providerID path int true "Provider ID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Provider not found"
// @Failure 409 {string} string "It is the last enabled sign-in method while password sign-in is disabled"
// @Failure 500 {string} string "Internal server error"
// @ID deleteAdminAuthProvider
// @Router /admin/auth-providers/{providerID} [delete]
func (adminRoute) DeleteAuthProvider(ctx context.Context, p *db.AuthProvider) error {
	c := ctx.Request().Context()
	if p.Enabled {
		settings, ok := loadSettings(ctx)
		if !ok {
			return nil
		}
		ok, err := ensureNotLastEnabled(ctx, settings)
		if err != nil {
			logrus.WithContext(c).WithError(err).Error("Failed to count enabled auth providers")
			return ctx.ApiServerError()
		}
		if !ok {
			return ctx.ApiError(http.StatusConflict, "sso::last_enabled_provider")
		}
	}
	if err := db.AuthProviders.Delete(c, p.ID); err != nil && !errors.Is(err, db.ErrAuthProviderNotFound) {
		logrus.WithContext(c).WithError(err).Error("Failed to delete auth provider")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// SetAuthProviderPositions
// @Summary Reorder the sign-in methods
// @Description Requires the admin. The sign-in page shows them in this order.
// @Accept json
// @Param data body form.SetAuthProviderPositions true "Provider IDs in order"
// @Success 204 "No Content"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID setAdminAuthProviderPositions
// @Router /admin/auth-providers/positions [put]
func (adminRoute) SetAuthProviderPositions(ctx context.Context, f form.SetAuthProviderPositions) error {
	if err := db.AuthProviders.SetPositions(ctx.Request().Context(), f.IDs); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to set auth provider positions")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// TestAuthProvider
// @Summary Test the connection of a sign-in method
// @Description Requires the admin. OIDC fetches the discovery document, SAML loads the IdP metadata, LDAP binds the service account and optionally searches the user. OAuth 2.0 is not supported.
// @Accept json
// @Param data body form.TestAuthProvider true "Config to test"
// @Success 204 "No Content"
// @Failure 400 {string} string "The connection failed or the type is not supported"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID testAdminAuthProvider
// @Router /admin/auth-providers/test [post]
func (adminRoute) TestAuthProvider(ctx context.Context, f form.TestAuthProvider) error {
	c := ctx.Request().Context()
	typ := db.AuthProviderType(f.Type)
	config := f.Config.Normalize(typ)
	secrets := sso.Secrets{BindPassword: f.BindPassword}
	if f.ID != 0 && secrets.BindPassword == "" {
		p, err := db.AuthProviders.GetByID(c, f.ID)
		if err != nil && !errors.Is(err, db.ErrAuthProviderNotFound) {
			logrus.WithContext(c).WithError(err).Error("Failed to get auth provider")
			return ctx.ApiServerError()
		}
		if p != nil && p.Type == typ {
			if _, saved, err := sso.LoadConfig(p); err == nil {
				secrets.BindPassword = saved.BindPassword
			}
		}
	}

	var err error
	switch typ {
	case db.AuthProviderOIDC:
		err = sso.ProbeOIDC(c, config.Issuer)
	case db.AuthProviderSAML:
		err = sso.ProbeSAML(c, config)
	case db.AuthProviderLDAP:
		err = sso.ProbeLDAP(config, secrets, f.Username)
	default:
		return ctx.ApiError(http.StatusBadRequest, "sso::test_unsupported")
	}
	if err != nil {
		if errors.Is(err, sso.ErrBadCredential) {
			err = errors.New("user not found or matched more than one entry")
		}
		msg := err.Error()
		if utf8.RuneCountInString(msg) > 300 {
			msg = string([]rune(msg)[:300])
		}
		return ctx.ApiError(http.StatusBadRequest, "sso::test_failed", msg)
	}
	return ctx.Status(http.StatusNoContent)
}
