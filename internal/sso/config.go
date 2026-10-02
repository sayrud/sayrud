package sso

import (
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"regexp"
	"strings"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/i18n"
)

// Config is the non-sensitive config of a provider, each type only uses some of the fields.
type Config struct {
	// AuthURL, TokenURL and UserInfoURL are the authorization, token and user info URLs of OAuth 2.0.
	AuthURL     string `json:"authURL,omitempty"`
	TokenURL    string `json:"tokenURL,omitempty"`
	UserInfoURL string `json:"userInfoURL,omitempty"`
	// EmailsURL returns [{email, primary, verified}], the primary and verified email in it takes precedence if set (GitHub, Gitea).
	EmailsURL string `json:"emailsURL,omitempty"`
	// SubjectPath, EmailPath, NamePath and GroupsPath are the dot paths in the user info JSON, e.g. data.user.email.
	SubjectPath string `json:"subjectPath,omitempty"`
	EmailPath   string `json:"emailPath,omitempty"`
	NamePath    string `json:"namePath,omitempty"`
	GroupsPath  string `json:"groupsPath,omitempty"`

	// ClientID and Scopes are used by OAuth 2.0 and OIDC.
	ClientID string   `json:"clientID,omitempty"`
	Scopes   []string `json:"scopes,omitempty"`

	// Issuer is the OIDC issuer for discovery.
	Issuer      string `json:"issuer,omitempty"`
	NameClaim   string `json:"nameClaim,omitempty"`
	GroupsClaim string `json:"groupsClaim,omitempty"`

	// IdPMetadataURL or IdPMetadataXML is the metadata of the SAML IdP, only one of them is set.
	IdPMetadataURL    string `json:"idpMetadataURL,omitempty"`
	IdPMetadataXML    string `json:"idpMetadataXML,omitempty"`
	AllowIdPInitiated bool   `json:"allowIdPInitiated,omitempty"`
	// SPCertificate is the generated SP certificate in PEM, the private key is in Secrets.
	SPCertificate string `json:"spCertificate,omitempty"`

	// EmailAttribute, NameAttribute and GroupsAttribute are the attribute names of SAML or LDAP, the common SAML attributes are detected if empty.
	EmailAttribute  string `json:"emailAttribute,omitempty"`
	NameAttribute   string `json:"nameAttribute,omitempty"`
	GroupsAttribute string `json:"groupsAttribute,omitempty"`

	// URL is the LDAP server address, ldap:// or ldaps://.
	URL      string `json:"url,omitempty"`
	StartTLS bool   `json:"startTLS,omitempty"`
	// RootCA is the CA in PEM to verify the LDAP server certificate, the system CAs are used if empty.
	RootCA string `json:"rootCA,omitempty"`
	// BindDN is the service account to search the users, it binds anonymously if empty.
	BindDN string `json:"bindDN,omitempty"`
	BaseDN string `json:"baseDN,omitempty"`
	// The {username} in UserFilter is replaced with the escaped user name of the sign-in.
	UserFilter       string `json:"userFilter,omitempty"`
	SubjectAttribute string `json:"subjectAttribute,omitempty"`

	// TrustEmail treats the email from the IdP as verified, OIDC uses email_verified instead.
	TrustEmail bool `json:"trustEmail,omitempty"`
} // @name AuthProviderConfig

const (
	defaultLDAPUserFilter = "(&(objectClass=person)(uid={username}))"
	usernamePlaceholder   = "{username}"
)

var defaultOIDCScopes = []string{"openid", "email", "profile"}

// Normalize trims the spaces, clears the fields of the other types and fills in the defaults.
func (c Config) Normalize(typ db.AuthProviderType) Config {
	trim := func(s string) string { return strings.TrimSpace(s) }
	scopes := make([]string, 0, len(c.Scopes))
	for _, s := range c.Scopes {
		for _, f := range strings.Fields(s) {
			if !contains(scopes, f) {
				scopes = append(scopes, f)
			}
		}
	}

	var n Config
	switch typ {
	case db.AuthProviderOAuth2:
		n = Config{
			AuthURL:     trim(c.AuthURL),
			TokenURL:    trim(c.TokenURL),
			UserInfoURL: trim(c.UserInfoURL),
			EmailsURL:   trim(c.EmailsURL),
			SubjectPath: orDefault(trim(c.SubjectPath), "id"),
			EmailPath:   orDefault(trim(c.EmailPath), "email"),
			NamePath:    orDefault(trim(c.NamePath), "name"),
			GroupsPath:  trim(c.GroupsPath),
			ClientID:    trim(c.ClientID),
			Scopes:      scopes,
			TrustEmail:  c.TrustEmail,
		}
	case db.AuthProviderOIDC:
		if len(scopes) == 0 {
			scopes = append(scopes, defaultOIDCScopes...)
		} else if !contains(scopes, "openid") {
			scopes = append([]string{"openid"}, scopes...)
		}
		n = Config{
			Issuer:      strings.TrimRight(trim(c.Issuer), "/"),
			ClientID:    trim(c.ClientID),
			Scopes:      scopes,
			NameClaim:   orDefault(trim(c.NameClaim), "name"),
			GroupsClaim: orDefault(trim(c.GroupsClaim), "groups"),
		}
	case db.AuthProviderSAML:
		n = Config{
			IdPMetadataURL:    trim(c.IdPMetadataURL),
			IdPMetadataXML:    trim(c.IdPMetadataXML),
			AllowIdPInitiated: c.AllowIdPInitiated,
			SPCertificate:     trim(c.SPCertificate),
			EmailAttribute:    trim(c.EmailAttribute),
			NameAttribute:     trim(c.NameAttribute),
			GroupsAttribute:   trim(c.GroupsAttribute),
			TrustEmail:        c.TrustEmail,
		}
	case db.AuthProviderLDAP:
		n = Config{
			URL:              trim(c.URL),
			StartTLS:         c.StartTLS,
			RootCA:           trim(c.RootCA),
			BindDN:           trim(c.BindDN),
			BaseDN:           trim(c.BaseDN),
			UserFilter:       orDefault(trim(c.UserFilter), defaultLDAPUserFilter),
			SubjectAttribute: orDefault(trim(c.SubjectAttribute), "uid"),
			EmailAttribute:   orDefault(trim(c.EmailAttribute), "mail"),
			NameAttribute:    orDefault(trim(c.NameAttribute), "displayName"),
			GroupsAttribute:  orDefault(trim(c.GroupsAttribute), "memberOf"),
			TrustEmail:       c.TrustEmail,
		}
	}
	return n
}

// Validate checks the normalized config and returns the *i18n.Error to be translated, the message argument is the JSON name of the field.
func (c Config) Validate(typ db.AuthProviderType, s Secrets) error {
	required := func(field, value string) error {
		if value == "" {
			return i18n.Errorf("sso::config_required", field)
		}
		return nil
	}
	httpURL := func(field, value string, optional bool) error {
		if value == "" && optional {
			return nil
		}
		if err := required(field, value); err != nil {
			return err
		}
		if !isHTTPURL(value) {
			return i18n.Errorf("sso::config_invalid", field)
		}
		return nil
	}

	var checks []error
	switch typ {
	case db.AuthProviderOAuth2:
		checks = []error{
			httpURL("authURL", c.AuthURL, false),
			httpURL("tokenURL", c.TokenURL, false),
			httpURL("userInfoURL", c.UserInfoURL, false),
			httpURL("emailsURL", c.EmailsURL, true),
			required("clientID", c.ClientID),
			required("clientSecret", s.ClientSecret),
		}
	case db.AuthProviderOIDC:
		checks = []error{
			httpURL("issuer", c.Issuer, false),
			required("clientID", c.ClientID),
			required("clientSecret", s.ClientSecret),
		}
	case db.AuthProviderSAML:
		switch {
		case c.IdPMetadataURL == "" && c.IdPMetadataXML == "":
			checks = append(checks, i18n.Errorf("sso::config_required", "idpMetadataURL"))
		case c.IdPMetadataURL != "" && c.IdPMetadataXML != "":
			checks = append(checks, i18n.Errorf("sso::saml_metadata_conflict"))
		case c.IdPMetadataURL != "":
			checks = append(checks, httpURL("idpMetadataURL", c.IdPMetadataURL, false))
		default:
			if _, err := parseIdPMetadata([]byte(c.IdPMetadataXML)); err != nil {
				checks = append(checks, i18n.Errorf("sso::config_invalid", "idpMetadataXML"))
			}
		}
	case db.AuthProviderLDAP:
		checks = []error{
			required("url", c.URL),
			ldapURL(c.URL),
			required("baseDN", c.BaseDN),
			userFilter(c.UserFilter),
			rootCA(c.RootCA),
		}
	default:
		return i18n.Errorf("sso::unsupported_type")
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}

func ldapURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "ldap" && u.Scheme != "ldaps") || u.Host == "" {
		return i18n.Errorf("sso::config_invalid", "url")
	}
	return nil
}

func userFilter(filter string) error {
	if !strings.Contains(filter, usernamePlaceholder) || !strings.HasPrefix(filter, "(") || !strings.HasSuffix(filter, ")") {
		return i18n.Errorf("sso::config_invalid", "userFilter")
	}
	return nil
}

func rootCA(pemData string) error {
	if pemData == "" {
		return nil
	}
	if _, err := parseCertPool(pemData); err != nil {
		return i18n.Errorf("sso::config_invalid", "rootCA")
	}
	return nil
}

func parseCertPool(pemData string) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	rest := []byte(pemData)
	found := false
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		pool.AddCert(cert)
		found = true
	}
	if !found {
		return nil, errNoCertificate
	}
	return pool, nil
}

func isHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,31}$`)

// ValidSlug reports whether the slug only has lowercase letters, digits and -, does not start with -, and is at most 32 characters.
func ValidSlug(slug string) bool {
	return slugRe.MatchString(slug)
}
