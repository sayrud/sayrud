package dto

import (
	"time"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/sso"
)

// SiteAuthProvider is an enabled sign-in method shown on the sign-in page.
type SiteAuthProvider struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	Type string `json:"type" enums:"oauth2,oidc,saml,ldap"`
	Icon string `json:"icon"`
} // @name SiteAuthProvider

func ToSiteAuthProvider(p *db.AuthProvider) *SiteAuthProvider {
	return &SiteAuthProvider{Slug: p.Slug, Name: p.Name, Type: string(p.Type), Icon: p.Icon}
}

// AdminAuthProvider is a sign-in method in the admin console, the secrets are only reported as set or not.
type AdminAuthProvider struct {
	ID       int64      `json:"id"`
	Slug     string     `json:"slug"`
	Name     string     `json:"name"`
	Icon     string     `json:"icon"`
	Type     string     `json:"type" enums:"oauth2,oidc,saml,ldap"`
	Enabled  bool       `json:"enabled"`
	Position int        `json:"position"`
	Config   sso.Config `json:"config"`
	// HasClientSecret and HasBindPassword report whether the secrets have been saved.
	HasClientSecret     bool     `json:"hasClientSecret"`
	HasBindPassword     bool     `json:"hasBindPassword"`
	AutoCreateUser      bool     `json:"autoCreateUser"`
	LinkByEmail         bool     `json:"linkByEmail"`
	AllowedEmailDomains []string `json:"allowedEmailDomains"`
	AllowedGroups       []string `json:"allowedGroups"`
	// IdentityCount is the number of users bound to the sign-in method.
	IdentityCount int64 `json:"identityCount"`
	// CallbackURL, ACSURL and MetadataURL are filled in at the identity provider, empty if the external URL is not set.
	CallbackURL string    `json:"callbackURL"`
	ACSURL      string    `json:"acsURL"`
	MetadataURL string    `json:"metadataURL"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
} // @name AdminAuthProvider

// ToAdminAuthProvider converts the provider for the admin console, the secrets are reported as not set if decryption fails.
func ToAdminAuthProvider(p *db.AuthProvider, identityCount int64, baseURL string) *AdminAuthProvider {
	config, secrets, _ := sso.LoadConfig(p)
	resp := &AdminAuthProvider{
		ID:                  p.ID,
		Slug:                p.Slug,
		Name:                p.Name,
		Icon:                p.Icon,
		Type:                string(p.Type),
		Enabled:             p.Enabled,
		Position:            p.Position,
		Config:              config,
		HasClientSecret:     secrets.ClientSecret != "",
		HasBindPassword:     secrets.BindPassword != "",
		AutoCreateUser:      p.AutoCreateUser,
		LinkByEmail:         p.LinkByEmail,
		AllowedEmailDomains: nonNil(p.AllowedEmailDomains),
		AllowedGroups:       nonNil(p.AllowedGroups),
		IdentityCount:       identityCount,
		CreatedAt:           p.CreatedAt,
		UpdatedAt:           p.UpdatedAt,
	}
	if baseURL != "" {
		switch p.Type {
		case db.AuthProviderOAuth2, db.AuthProviderOIDC:
			resp.CallbackURL = sso.CallbackURL(baseURL, p.Slug)
		case db.AuthProviderSAML:
			resp.ACSURL = sso.ACSURL(baseURL, p.Slug)
			resp.MetadataURL = sso.MetadataURL(baseURL, p.Slug)
		}
	}
	return resp
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

type ListAdminAuthProvidersResp struct {
	Providers []*AdminAuthProvider `json:"providers"`
	// SecretsReady reports whether auth.secret_key is configured.
	SecretsReady bool   `json:"secretsReady"`
	ExternalURL  string `json:"externalURL"`
} // @name ListAdminAuthProvidersResp

// AuthProviderBrief is a bound sign-in method shown in the member list.
type AuthProviderBrief struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
	Icon string `json:"icon"`
} // @name AuthProviderBrief

// UserIdentity is a third-party account bound to the signed-in user.
type UserIdentity struct {
	ID           int64      `json:"id"`
	ProviderSlug string     `json:"providerSlug"`
	ProviderName string     `json:"providerName"`
	ProviderIcon string     `json:"providerIcon"`
	ProviderType string     `json:"providerType" enums:"oauth2,oidc,saml,ldap"`
	Email        string     `json:"email"`
	CreatedAt    time.Time  `json:"createdAt"`
	LastUsedAt   *time.Time `json:"lastUsedAt"`
} // @name UserIdentity

func ToUserIdentity(i *db.UserIdentity, p *db.AuthProvider) *UserIdentity {
	return &UserIdentity{
		ID:           i.ID,
		ProviderSlug: p.Slug,
		ProviderName: p.Name,
		ProviderIcon: p.Icon,
		ProviderType: string(p.Type),
		Email:        i.Email,
		CreatedAt:    i.CreatedAt,
		LastUsedAt:   i.LastUsedAt,
	}
}
