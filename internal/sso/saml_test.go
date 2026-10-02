package sso

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
)

type staticSP struct{ entity *saml.EntityDescriptor }

func (s staticSP) GetServiceProvider(*http.Request, string) (*saml.EntityDescriptor, error) {
	return s.entity, nil
}

type samlFixture struct {
	idp      *saml.IdentityProvider
	provider *db.AuthProvider
	sp       RedirectProvider
	used     map[string]bool
	session  *saml.Session
}

func newSAMLFixture(t *testing.T, allowIdPInitiated bool) *samlFixture {
	t.Helper()
	withTestKey(t)

	idpCertPEM, idpKeyPEM, err := GenerateSPCertificate("idp.test")
	require.NoError(t, err)
	idpCert, idpKey, err := parseKeyPair(idpCertPEM, idpKeyPEM)
	require.NoError(t, err)
	metadataURL, _ := url.Parse("https://idp.test/metadata")
	ssoURL, _ := url.Parse("https://idp.test/sso")
	idp := &saml.IdentityProvider{
		Key:         idpKey,
		Certificate: idpCert,
		Logger:      logger.DefaultLogger,
		MetadataURL: *metadataURL,
		SSOURL:      *ssoURL,
	}
	idpMetadata, err := xml.Marshal(idp.Metadata())
	require.NoError(t, err)

	spCert, spKey, err := GenerateSPCertificate("sayrud")
	require.NoError(t, err)
	c := Config{IdPMetadataXML: string(idpMetadata), SPCertificate: spCert, GroupsAttribute: "eduPersonAffiliation", TrustEmail: true, AllowIdPInitiated: allowIdPInitiated}.Normalize(db.AuthProviderSAML)
	require.NoError(t, c.Validate(db.AuthProviderSAML, Secrets{SPPrivateKey: spKey}))
	provider := newTestProvider(t, db.AuthProviderSAML, c, Secrets{SPPrivateKey: spKey})

	f := &samlFixture{idp: idp, provider: provider, used: map[string]bool{}}
	f.sp, err = NewRedirect(provider, Options{BaseURL: testBaseURL, MarkAssertion: func(_ context.Context, key string, until time.Time) (bool, error) {
		assert.True(t, until.After(time.Now()))
		if f.used[key] {
			return false, nil
		}
		f.used[key] = true
		return true, nil
	}})
	require.NoError(t, err)

	spMetadataXML, err := SAMLMetadata(provider, Options{BaseURL: testBaseURL})
	require.NoError(t, err)
	spMetadata := &saml.EntityDescriptor{}
	require.NoError(t, xml.Unmarshal(spMetadataXML, spMetadata))
	assert.Equal(t, testBaseURL+"/_/auth/sso/test/metadata", spMetadata.EntityID)
	idp.ServiceProviderProvider = staticSP{spMetadata}

	f.session = &saml.Session{
		ID:             "session-1",
		NameID:         "alice-persistent-id",
		NameIDFormat:   string(saml.PersistentNameIDFormat),
		UserEmail:      "Alice@Example.com",
		UserCommonName: "Alice",
		Groups:         []string{"dev", "ops"},
	}
	return f
}

// respond lets the IdP handle the SP redirect and returns the request posted to the ACS.
func (f *samlFixture) respond(t *testing.T, redirect string) *http.Request {
	t.Helper()
	idpReq, err := saml.NewIdpAuthnRequest(f.idp, httptest.NewRequest(http.MethodGet, redirect, nil))
	require.NoError(t, err)
	require.NoError(t, idpReq.Validate())
	return f.post(t, idpReq)
}

func (f *samlFixture) post(t *testing.T, idpReq *saml.IdpAuthnRequest) *http.Request {
	t.Helper()
	require.NoError(t, saml.DefaultAssertionMaker{}.MakeAssertion(idpReq, f.session))
	form, err := idpReq.PostBinding()
	require.NoError(t, err)
	assert.Equal(t, testBaseURL+"/_/auth/sso/test/acs", form.URL)
	body := url.Values{"SAMLResponse": {form.SAMLResponse}, "RelayState": {form.RelayState}}
	r := httptest.NewRequest(http.MethodPost, "/_/auth/sso/test/acs", strings.NewReader(body.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

// idpInitiated builds an IdP-initiated response without an AuthnRequest.
func (f *samlFixture) idpInitiated(t *testing.T) *http.Request {
	t.Helper()
	entity, err := f.idp.ServiceProviderProvider.GetServiceProvider(nil, "")
	require.NoError(t, err)
	idpReq := &saml.IdpAuthnRequest{IDP: f.idp, HTTPRequest: httptest.NewRequest(http.MethodGet, "/", nil), Now: saml.TimeNow(), ServiceProviderMetadata: entity}
	idpReq.SPSSODescriptor = &entity.SPSSODescriptors[0]
	for i, acs := range idpReq.SPSSODescriptor.AssertionConsumerServices {
		if acs.Binding == saml.HTTPPostBinding {
			idpReq.ACSEndpoint = &idpReq.SPSSODescriptor.AssertionConsumerServices[i]
		}
	}
	return f.post(t, idpReq)
}

func TestSAML(t *testing.T) {
	f := newSAMLFixture(t, false)
	state, err := NewState(7, ModeLogin, "/", 0)
	require.NoError(t, err)
	redirect, err := f.sp.Begin(context.Background(), state)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(redirect, "https://idp.test/sso?"))
	assert.NotEmpty(t, state.RequestID)
	u, _ := url.Parse(redirect)
	assert.Equal(t, state.State, u.Query().Get("RelayState"))

	acs := f.respond(t, redirect)
	id, err := f.sp.Complete(context.Background(), acs, state)
	require.NoError(t, err)
	assert.Equal(t, &Identity{Subject: "alice-persistent-id", Email: "alice@example.com", EmailVerified: true, Name: "Alice", Groups: []string{"dev", "ops"}}, id)

	// The same assertion can not be used twice.
	acs2 := f.respond(t, redirect)
	_, err = f.sp.Complete(context.Background(), acs2, state)
	require.NoError(t, err, "a new assertion of the same request is accepted")
	_, err = f.sp.Complete(context.Background(), acs, state)
	assert.Equal(t, CodeInvalidState, ErrorCode(err), "replayed assertion")

	// InResponseTo differs from the saved request ID.
	other := *state
	other.RequestID = "id-forged"
	_, err = f.sp.Complete(context.Background(), f.respond(t, redirect), &other)
	assert.Equal(t, CodeIdPError, ErrorCode(err))

	// IdP-initiated sign-in is not allowed.
	_, err = f.sp.Complete(context.Background(), f.idpInitiated(t), nil)
	assert.Equal(t, CodeInvalidState, ErrorCode(err))
}

func TestSAML_IdPInitiated(t *testing.T) {
	f := newSAMLFixture(t, true)
	id, err := f.sp.Complete(context.Background(), f.idpInitiated(t), nil)
	require.NoError(t, err)
	assert.Equal(t, "alice-persistent-id", id.Subject)
}

func TestSAMLIdentity_Fallbacks(t *testing.T) {
	p := &samlProvider{config: Config{}}
	id, err := p.identity(&saml.Assertion{
		Subject: &saml.Subject{NameID: &saml.NameID{Value: "bob@example.com", Format: string(saml.EmailAddressNameIDFormat)}},
		AttributeStatements: []saml.AttributeStatement{{Attributes: []saml.Attribute{
			{Name: "http://schemas.microsoft.com/identity/claims/displayname", Values: []saml.AttributeValue{{Value: "Bob"}}},
			{Name: "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups", Values: []saml.AttributeValue{{Value: "g1"}, {Value: "g2"}}},
		}}},
	})
	require.NoError(t, err)
	assert.Equal(t, &Identity{Subject: "bob@example.com", Email: "bob@example.com", Name: "Bob", Groups: []string{"g1", "g2"}}, id)

	_, err = p.identity(&saml.Assertion{})
	assert.Equal(t, CodeIdPError, ErrorCode(err))
}
