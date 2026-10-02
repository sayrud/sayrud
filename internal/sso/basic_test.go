package sso

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/i18n"
)

func withTestKey(t *testing.T) {
	t.Helper()
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	SetKey(key)
	t.Cleanup(func() { SetKey(nil) })
}

func TestSecrets(t *testing.T) {
	SetKey(nil)
	assert.False(t, SecretsReady())
	_, err := Seal([]byte("x"))
	require.ErrorIs(t, err, ErrNoSecretKey)

	withTestKey(t)
	assert.True(t, SecretsReady())

	sealed, err := Seal([]byte("hello"))
	require.NoError(t, err)
	plain, err := Open(sealed)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(plain))

	other, err := Seal([]byte("hello"))
	require.NoError(t, err)
	assert.NotEqual(t, sealed, other, "nonce must be random")

	tampered := []byte(sealed)
	tampered[len(tampered)-2] ^= 1
	_, err = Open(string(tampered))
	require.Error(t, err)

	encoded, err := EncodeSecrets(Secrets{})
	require.NoError(t, err)
	assert.Empty(t, encoded)
	decoded, err := DecodeSecrets("")
	require.NoError(t, err)
	assert.True(t, decoded.IsZero())

	encoded, err = EncodeSecrets(Secrets{ClientSecret: "s3cr3t"})
	require.NoError(t, err)
	assert.NotContains(t, encoded, "s3cr3t")
	decoded, err = DecodeSecrets(encoded)
	require.NoError(t, err)
	assert.Equal(t, Secrets{ClientSecret: "s3cr3t"}, decoded)
}

func TestLookupPath(t *testing.T) {
	v, err := decodeJSON([]byte(`{"id": 12345678901234567, "data": {"user": {"email": "a@b.com", "groups": ["x", 2]}}, "list": [{"n": "first"}], "flag": true, "nil": null}`))
	require.NoError(t, err)

	assert.Equal(t, "12345678901234567", pathString(v, "id"))
	assert.Equal(t, "a@b.com", pathString(v, "data.user.email"))
	assert.Equal(t, "first", pathString(v, "list.0.n"))
	assert.Equal(t, "true", pathString(v, "flag"))
	assert.Equal(t, "", pathString(v, "nil"))
	assert.Equal(t, "", pathString(v, "data.user"))
	assert.Equal(t, "", pathString(v, "missing.path"))
	assert.Equal(t, "", pathString(v, "list.3.n"))
	assert.Equal(t, "", pathString(v, ""))

	assert.Equal(t, []string{"x", "2"}, pathStrings(v, "data.user.groups"))
	assert.Equal(t, []string{"a@b.com"}, pathStrings(v, "data.user.email"))
	assert.Nil(t, pathStrings(v, "data.none"))
}

func TestCheckAccess(t *testing.T) {
	id := &Identity{Email: "Alice@Example.COM", Groups: []string{"dev", "ops"}}
	assert.True(t, CheckAccess(id, nil, nil))
	assert.True(t, CheckAccess(id, []string{"example.com"}, nil))
	assert.False(t, CheckAccess(id, []string{"other.com"}, nil))
	assert.True(t, CheckAccess(id, nil, []string{"ops"}))
	assert.False(t, CheckAccess(id, nil, []string{"Ops"}))
	assert.False(t, CheckAccess(id, []string{"example.com"}, []string{"admin"}))
	assert.False(t, CheckAccess(&Identity{}, []string{"example.com"}, nil))

	assert.Equal(t, []string{"example.com", "b.org"}, NormalizeDomains([]string{" @Example.com ", "example.com", "", "B.org"}))
	assert.Equal(t, []string{"Dev", "dev"}, NormalizeGroups([]string{" Dev", "dev", "Dev", " "}))
}

func TestDeriveUserName(t *testing.T) {
	assert.Equal(t, "Alice", DeriveUserName(&Identity{Name: " Alice ", Email: "a@b.com"}))
	assert.Equal(t, "alice", DeriveUserName(&Identity{Email: "alice@b.com", Subject: "1"}))
	assert.Equal(t, "42", DeriveUserName(&Identity{Subject: "42"}))
	long := DeriveUserName(&Identity{Name: "这是一个非常非常非常非常非常非常非常非常非常非常长的名字超过三十二个字符"})
	assert.Equal(t, 32, len([]rune(long)))
}

func TestValidSlug(t *testing.T) {
	for _, s := range []string{"github", "corp-sso", "a", "0abc"} {
		assert.True(t, ValidSlug(s), s)
	}
	for _, s := range []string{"", "-abc", "GitHub", "a_b", "a/b", "abcdefghijklmnopqrstuvwxyz0123456"} {
		assert.False(t, ValidSlug(s), s)
	}
}

func errKey(t *testing.T, err error) string {
	t.Helper()
	var e *i18n.Error
	require.ErrorAs(t, err, &e)
	return e.Key + ":" + func() string {
		if len(e.Args) > 0 {
			return e.Args[0].(string)
		}
		return ""
	}()
}

func TestConfigNormalize(t *testing.T) {
	c := Config{Issuer: " https://idp.example.com/ ", ClientID: " cid ", Scopes: []string{"email profile", "email"}, AuthURL: "x"}.Normalize(db.AuthProviderOIDC)
	assert.Equal(t, "https://idp.example.com", c.Issuer)
	assert.Equal(t, "cid", c.ClientID)
	assert.Equal(t, []string{"openid", "email", "profile"}, c.Scopes)
	assert.Empty(t, c.AuthURL, "fields of other types are cleared")
	assert.Equal(t, "name", c.NameClaim)

	assert.Equal(t, defaultOIDCScopes, Config{}.Normalize(db.AuthProviderOIDC).Scopes)

	o := Config{}.Normalize(db.AuthProviderOAuth2)
	assert.Equal(t, "id", o.SubjectPath)
	assert.Equal(t, "email", o.EmailPath)
	assert.Equal(t, "name", o.NamePath)

	l := Config{}.Normalize(db.AuthProviderLDAP)
	assert.Equal(t, defaultLDAPUserFilter, l.UserFilter)
	assert.Equal(t, "uid", l.SubjectAttribute)
}

func TestConfigValidate(t *testing.T) {
	oauth := Config{AuthURL: "https://github.com/login/oauth/authorize", TokenURL: "https://github.com/login/oauth/access_token", UserInfoURL: "https://api.github.com/user", ClientID: "id"}.Normalize(db.AuthProviderOAuth2)
	require.NoError(t, oauth.Validate(db.AuthProviderOAuth2, Secrets{ClientSecret: "s"}))
	assert.Equal(t, "sso::config_required:clientSecret", errKey(t, oauth.Validate(db.AuthProviderOAuth2, Secrets{})))
	bad := oauth
	bad.TokenURL = "github.com/token"
	assert.Equal(t, "sso::config_invalid:tokenURL", errKey(t, bad.Validate(db.AuthProviderOAuth2, Secrets{ClientSecret: "s"})))
	bad = oauth
	bad.EmailsURL = "ftp://x"
	assert.Equal(t, "sso::config_invalid:emailsURL", errKey(t, bad.Validate(db.AuthProviderOAuth2, Secrets{ClientSecret: "s"})))

	oidc := Config{Issuer: "https://accounts.google.com", ClientID: "id"}.Normalize(db.AuthProviderOIDC)
	require.NoError(t, oidc.Validate(db.AuthProviderOIDC, Secrets{ClientSecret: "s"}))
	assert.Equal(t, "sso::config_required:issuer", errKey(t, Config{ClientID: "id"}.Validate(db.AuthProviderOIDC, Secrets{ClientSecret: "s"})))

	assert.Equal(t, "sso::config_required:idpMetadataURL", errKey(t, Config{}.Validate(db.AuthProviderSAML, Secrets{})))
	assert.Equal(t, "sso::saml_metadata_conflict:", errKey(t, Config{IdPMetadataURL: "https://a", IdPMetadataXML: "<x/>"}.Validate(db.AuthProviderSAML, Secrets{})))
	assert.Equal(t, "sso::config_invalid:idpMetadataXML", errKey(t, Config{IdPMetadataXML: "not xml"}.Validate(db.AuthProviderSAML, Secrets{})))
	require.NoError(t, Config{IdPMetadataURL: "https://idp.example.com/metadata"}.Validate(db.AuthProviderSAML, Secrets{}))

	ldap := Config{URL: "ldaps://ldap.example.com", BaseDN: "dc=example,dc=com"}.Normalize(db.AuthProviderLDAP)
	require.NoError(t, ldap.Validate(db.AuthProviderLDAP, Secrets{}))
	bad = ldap
	bad.URL = "http://ldap.example.com"
	assert.Equal(t, "sso::config_invalid:url", errKey(t, bad.Validate(db.AuthProviderLDAP, Secrets{})))
	bad = ldap
	bad.UserFilter = "(uid=admin)"
	assert.Equal(t, "sso::config_invalid:userFilter", errKey(t, bad.Validate(db.AuthProviderLDAP, Secrets{})))
	bad = ldap
	bad.RootCA = "not a pem"
	assert.Equal(t, "sso::config_invalid:rootCA", errKey(t, bad.Validate(db.AuthProviderLDAP, Secrets{})))

	assert.Equal(t, "sso::unsupported_type:", errKey(t, Config{}.Validate("cas", Secrets{})))
}

func TestErrorCode(t *testing.T) {
	assert.Equal(t, CodeAccessDenied, ErrorCode(Errorf(CodeAccessDenied, nil)))
	assert.Equal(t, CodeIdPError, ErrorCode(bytes.ErrTooLarge))
}
