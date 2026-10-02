package form

import "github.com/wuhan005/sayrud/internal/sso"

// SaveAuthProvider is the fields shared by creating and updating a sign-in method.
type SaveAuthProvider struct {
	Name    string     `json:"name" valid:"required;maxlen:32"`
	Icon    string     `json:"icon"`
	Enabled bool       `json:"enabled"`
	Config  sso.Config `json:"config"`
	// ClientSecret is the client secret of OAuth 2.0 or OIDC, empty keeps the saved one when updating.
	ClientSecret string `json:"clientSecret,omitempty"`
	// BindPassword is the password of the LDAP service account, empty keeps the saved one when updating.
	BindPassword        string   `json:"bindPassword,omitempty"`
	AutoCreateUser      bool     `json:"autoCreateUser"`
	LinkByEmail         bool     `json:"linkByEmail"`
	AllowedEmailDomains []string `json:"allowedEmailDomains"`
	AllowedGroups       []string `json:"allowedGroups"`
}

type CreateAuthProvider struct {
	SaveAuthProvider
	// Slug is part of the callback URL and can not be changed after creating.
	Slug string `json:"slug"`
	Type string `json:"type" enums:"oauth2,oidc,saml,ldap"`
} // @name CreateAuthProvider

type UpdateAuthProvider struct {
	SaveAuthProvider
} // @name UpdateAuthProvider

type SetAuthProviderPositions struct {
	IDs []int64 `json:"ids"`
} // @name SetAuthProviderPositions

type TestAuthProvider struct {
	// ID is the saved provider whose secrets are used if empty, 0 when creating.
	ID     int64      `json:"id,omitempty"`
	Type   string     `json:"type" enums:"oauth2,oidc,saml,ldap"`
	Config sso.Config `json:"config"`
	// BindPassword is the password of the LDAP service account.
	BindPassword string `json:"bindPassword,omitempty"`
	// Username is the LDAP user to search, optional.
	Username string `json:"username,omitempty"`
} // @name TestAuthProvider
