package sso

import (
	"context"
	"crypto/tls"
	"encoding/hex"
	"net"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-ldap/ldap/v3"
	"github.com/pkg/errors"

	"github.com/wuhan005/sayrud/internal/db"
)

const ldapTimeout = 10 * time.Second

// ldapConn is the methods of *ldap.Conn in use, which tests can replace.
type ldapConn interface {
	Bind(username, password string) error
	Search(req *ldap.SearchRequest) (*ldap.SearchResult, error)
	Close() error
}

// dialLDAP connects to the LDAP server and upgrades ldap:// to TLS if StartTLS is on.
var dialLDAP = func(c Config) (ldapConn, error) {
	u, err := url.Parse(c.URL)
	if err != nil {
		return nil, errors.Wrap(err, "parse URL")
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: u.Hostname()}
	if c.RootCA != "" {
		if tlsConfig.RootCAs, err = parseCertPool(c.RootCA); err != nil {
			return nil, errors.Wrap(err, "parse root CA")
		}
	}
	conn, err := ldap.DialURL(c.URL, ldap.DialWithDialer(&net.Dialer{Timeout: ldapTimeout}), ldap.DialWithTLSConfig(tlsConfig))
	if err != nil {
		return nil, errors.Wrap(err, "dial")
	}
	conn.SetTimeout(ldapTimeout)
	if c.StartTLS && u.Scheme == "ldap" {
		if err := conn.StartTLS(tlsConfig); err != nil {
			_ = conn.Close()
			return nil, errors.Wrap(err, "start TLS")
		}
	}
	return conn, nil
}

type ldapProvider struct {
	config  Config
	secrets Secrets
}

func newLDAP(c Config, s Secrets) *ldapProvider {
	return &ldapProvider{config: c, secrets: s}
}

// connect connects and binds the service account, it searches anonymously if BindDN is empty.
func (p *ldapProvider) connect() (ldapConn, error) {
	conn, err := dialLDAP(p.config)
	if err != nil {
		return nil, err
	}
	if p.config.BindDN != "" {
		if err := conn.Bind(p.config.BindDN, p.secrets.BindPassword); err != nil {
			_ = conn.Close()
			return nil, errors.Wrap(err, "bind service account")
		}
	}
	return conn, nil
}

// userFilter returns the filter with the escaped user name.
func (p *ldapProvider) userFilter(username string) string {
	return strings.ReplaceAll(p.config.UserFilter, usernamePlaceholder, ldap.EscapeFilter(username))
}

// findUser searches the user, it returns ErrBadCredential unless exactly one entry matches.
func (p *ldapProvider) findUser(conn ldapConn, username string) (*ldap.Entry, error) {
	attrs := []string{p.config.SubjectAttribute, p.config.EmailAttribute, p.config.NameAttribute, p.config.GroupsAttribute}
	req := ldap.NewSearchRequest(p.config.BaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 2, int(ldapTimeout/time.Second), false, p.userFilter(username), attrs, nil)
	result, err := conn.Search(req)
	if err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded) {
			return nil, errors.Wrap(ErrBadCredential, "multiple users matched")
		}
		if ldap.IsErrorWithCode(err, ldap.LDAPResultNoSuchObject) {
			return nil, errors.Wrap(ErrBadCredential, "base DN not found")
		}
		return nil, errors.Wrap(err, "search user")
	}
	if len(result.Entries) != 1 {
		return nil, errors.Wrapf(ErrBadCredential, "%d users matched", len(result.Entries))
	}
	return result.Entries[0], nil
}

func (p *ldapProvider) Authenticate(_ context.Context, username, password string) (*Identity, error) {
	username = strings.TrimSpace(username)
	// An empty password is an anonymous bind that succeeds on most LDAP servers, it must be rejected.
	if username == "" || password == "" {
		return nil, ErrBadCredential
	}
	conn, err := p.connect()
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	entry, err := p.findUser(conn, username)
	if err != nil {
		return nil, err
	}
	if err := conn.Bind(entry.DN, password); err != nil {
		if ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
			return nil, ErrBadCredential
		}
		return nil, errors.Wrap(err, "bind user")
	}
	return p.identity(entry), nil
}

func (p *ldapProvider) identity(entry *ldap.Entry) *Identity {
	subject := attributeString(entry.GetEqualFoldRawAttributeValue(p.config.SubjectAttribute))
	if subject == "" {
		subject = entry.DN
	}
	var groups []string
	for _, g := range entry.GetEqualFoldAttributeValues(p.config.GroupsAttribute) {
		groups = append(groups, g)
		if cn := firstRDNValue(g); cn != "" && cn != g {
			groups = append(groups, cn)
		}
	}
	return &Identity{
		Subject:       subject,
		Email:         db.NormalizeEmail(entry.GetEqualFoldAttributeValue(p.config.EmailAttribute)),
		EmailVerified: p.config.TrustEmail,
		Name:          strings.TrimSpace(entry.GetEqualFoldAttributeValue(p.config.NameAttribute)),
		Groups:        groups,
	}
}

// attributeString converts the binary attributes (e.g. objectGUID of AD) to hex, text attributes are returned as is.
func attributeString(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if utf8.Valid(raw) {
		return strings.TrimSpace(string(raw))
	}
	return hex.EncodeToString(raw)
}

// firstRDNValue returns the value of the first RDN of the DN, e.g. dev for cn=dev,ou=groups,dc=example, so the allowed groups can be group names.
func firstRDNValue(dn string) string {
	parsed, err := ldap.ParseDN(dn)
	if err != nil || len(parsed.RDNs) == 0 || len(parsed.RDNs[0].Attributes) == 0 {
		return ""
	}
	return parsed.RDNs[0].Attributes[0].Value
}

// ProbeLDAP connects with the service account and searches the username if not empty, to test the connection in the admin console.
func ProbeLDAP(c Config, s Secrets, username string) error {
	p := newLDAP(c, s)
	conn, err := p.connect()
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if username = strings.TrimSpace(username); username != "" {
		if _, err := p.findUser(conn, username); err != nil {
			return err
		}
	}
	return nil
}
