package sso

import (
	"context"
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
)

// fakeLDAP records the binds and searches, passwords map the DNs to the passwords.
type fakeLDAP struct {
	passwords map[string]string
	entries   []*ldap.Entry
	searchErr error
	binds     []string
	filters   []string
	closed    bool
}

func (f *fakeLDAP) Bind(username, password string) error {
	f.binds = append(f.binds, username)
	if p, ok := f.passwords[username]; ok && p == password {
		return nil
	}
	return ldap.NewError(ldap.LDAPResultInvalidCredentials, nil)
}

func (f *fakeLDAP) Search(req *ldap.SearchRequest) (*ldap.SearchResult, error) {
	f.filters = append(f.filters, req.Filter)
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	return &ldap.SearchResult{Entries: f.entries}, nil
}

func (f *fakeLDAP) Close() error {
	f.closed = true
	return nil
}

func withFakeLDAP(t *testing.T, f *fakeLDAP) {
	t.Helper()
	original := dialLDAP
	dialLDAP = func(Config) (ldapConn, error) { return f, nil }
	t.Cleanup(func() { dialLDAP = original })
}

func TestLDAP(t *testing.T) {
	withTestKey(t)
	aliceDN := "uid=alice,ou=people,dc=example,dc=com"
	f := &fakeLDAP{
		passwords: map[string]string{"cn=svc,dc=example,dc=com": "svc-pass", aliceDN: "alice-pass"},
		entries: []*ldap.Entry{ldap.NewEntry(aliceDN, map[string][]string{
			"uid":         {"alice"},
			"mail":        {"Alice@Example.com"},
			"displayName": {"Alice"},
			"memberOf":    {"cn=dev,ou=groups,dc=example,dc=com"},
		})},
	}
	withFakeLDAP(t, f)

	c := Config{URL: "ldaps://ldap.example.com", BindDN: "cn=svc,dc=example,dc=com", BaseDN: "dc=example,dc=com", TrustEmail: true}.Normalize(db.AuthProviderLDAP)
	p, err := NewPassword(newTestProvider(t, db.AuthProviderLDAP, c, Secrets{BindPassword: "svc-pass"}))
	require.NoError(t, err)

	id, err := p.Authenticate(context.Background(), " alice ", "alice-pass")
	require.NoError(t, err)
	assert.Equal(t, &Identity{
		Subject:       "alice",
		Email:         "alice@example.com",
		EmailVerified: true,
		Name:          "Alice",
		Groups:        []string{"cn=dev,ou=groups,dc=example,dc=com", "dev"},
	}, id)
	assert.Equal(t, []string{"cn=svc,dc=example,dc=com", aliceDN}, f.binds)
	assert.Equal(t, "(&(objectClass=person)(uid=alice))", f.filters[0])
	assert.True(t, f.closed)

	_, err = p.Authenticate(context.Background(), "alice", "wrong")
	assert.ErrorIs(t, err, ErrBadCredential)

	// An empty password never reaches the anonymous bind.
	f.binds = nil
	_, err = p.Authenticate(context.Background(), "alice", "")
	assert.ErrorIs(t, err, ErrBadCredential)
	assert.Empty(t, f.binds)

	// The filter characters in the user name are escaped.
	_, _ = p.Authenticate(context.Background(), "*)(uid=*", "x")
	assert.Equal(t, `(&(objectClass=person)(uid=\2a\29\28uid=\2a))`, f.filters[len(f.filters)-1])

	f.entries = nil
	_, err = p.Authenticate(context.Background(), "nobody", "x")
	assert.ErrorIs(t, err, ErrBadCredential)

	f.searchErr = ldap.NewError(ldap.LDAPResultSizeLimitExceeded, nil)
	_, err = p.Authenticate(context.Background(), "a*", "x")
	assert.ErrorIs(t, err, ErrBadCredential)

	f.searchErr = nil
	f.passwords["cn=svc,dc=example,dc=com"] = "changed"
	err = ProbeLDAP(c, Secrets{BindPassword: "svc-pass"}, "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrBadCredential)
}

func TestLDAPAttributes(t *testing.T) {
	assert.Equal(t, "0a0bff", attributeString([]byte{0x0a, 0x0b, 0xff}))
	assert.Equal(t, "alice", attributeString([]byte(" alice ")))
	assert.Equal(t, "", attributeString(nil))
	assert.Equal(t, "dev", firstRDNValue("CN=dev,OU=Groups,DC=corp"))
	assert.Equal(t, "", firstRDNValue("not a dn"))
}
