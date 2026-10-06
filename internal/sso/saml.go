package sso

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"encoding/xml"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/crewjam/saml"

	"github.com/wuhan005/sayrud/internal/db"
)

type samlProvider struct {
	provider *db.AuthProvider
	config   Config
	secrets  Secrets
	opts     Options
}

func newSAML(p *db.AuthProvider, c Config, s Secrets, opts Options) (*samlProvider, error) {
	if c.SPCertificate == "" || s.SPPrivateKey == "" {
		return nil, errors.New("SP certificate is not generated")
	}
	return &samlProvider{provider: p, config: c, secrets: s, opts: opts}, nil
}

// Common attributes tried in order if the attribute name is not set, matching both Name and FriendlyName.
var (
	samlEmailAttributes = []string{
		"email", "mail", "emailAddress",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
		"urn:oid:0.9.2342.19200300.100.1.3",
		"urn:oid:1.2.840.113549.1.9.1",
	}
	samlNameAttributes = []string{
		"displayName", "name", "cn",
		"http://schemas.microsoft.com/identity/claims/displayname",
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
		"urn:oid:2.16.840.1.113730.3.1.241",
		"urn:oid:2.5.4.3",
	}
	samlGroupsAttributes = []string{
		"groups", "memberOf", "Group",
		"http://schemas.microsoft.com/ws/2008/06/identity/claims/groups",
		"http://schemas.xmlsoap.org/claims/Group",
	}
)

type cachedMetadata struct {
	entity    *saml.EntityDescriptor
	expiresAt time.Time
}

var (
	metadataMu    sync.Mutex
	metadataCache = map[string]cachedMetadata{}
)

const metadataTTL = 10 * time.Minute

func (p *samlProvider) idpMetadata(ctx context.Context) (*saml.EntityDescriptor, error) {
	if p.config.IdPMetadataXML != "" {
		return parseIdPMetadata([]byte(p.config.IdPMetadataXML))
	}
	metadataMu.Lock()
	cached, ok := metadataCache[p.config.IdPMetadataURL]
	metadataMu.Unlock()
	if ok && time.Now().Before(cached.expiresAt) {
		return cached.entity, nil
	}
	entity, err := fetchIdPMetadata(ctx, p.opts.httpClient(), p.config.IdPMetadataURL)
	if err != nil {
		return nil, err
	}
	metadataMu.Lock()
	metadataCache[p.config.IdPMetadataURL] = cachedMetadata{entity: entity, expiresAt: time.Now().Add(metadataTTL)}
	metadataMu.Unlock()
	return entity, nil
}

// serviceProvider builds the SP, the IdP metadata is not loaded if withIdP is false (only for the SP metadata).
func (p *samlProvider) serviceProvider(ctx context.Context, withIdP, idpInitiated bool) (*saml.ServiceProvider, error) {
	cert, key, err := parseKeyPair(p.config.SPCertificate, p.secrets.SPPrivateKey)
	if err != nil {
		return nil, err
	}
	acs, err := url.Parse(ACSURL(p.opts.BaseURL, p.provider.Slug))
	if err != nil {
		return nil, errors.Wrap(err, "parse ACS URL")
	}
	metadata, err := url.Parse(MetadataURL(p.opts.BaseURL, p.provider.Slug))
	if err != nil {
		return nil, errors.Wrap(err, "parse metadata URL")
	}
	sp := &saml.ServiceProvider{
		EntityID:          metadata.String(),
		Key:               key,
		Certificate:       cert,
		HTTPClient:        p.opts.httpClient(),
		MetadataURL:       *metadata,
		AcsURL:            *acs,
		AuthnNameIDFormat: saml.UnspecifiedNameIDFormat,
		AllowIDPInitiated: idpInitiated,
	}
	if withIdP {
		if sp.IDPMetadata, err = p.idpMetadata(ctx); err != nil {
			return nil, errors.Wrap(err, "load IdP metadata")
		}
	}
	return sp, nil
}

// Metadata returns the SP metadata XML.
func (p *samlProvider) Metadata() ([]byte, error) {
	sp, err := p.serviceProvider(context.Background(), false, false)
	if err != nil {
		return nil, err
	}
	raw, err := xml.MarshalIndent(sp.Metadata(), "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "marshal metadata")
	}
	return append([]byte(xml.Header), raw...), nil
}

func (p *samlProvider) Begin(ctx context.Context, state *State) (string, error) {
	sp, err := p.serviceProvider(ctx, true, false)
	if err != nil {
		return "", err
	}
	location := sp.GetSSOBindingLocation(saml.HTTPRedirectBinding)
	if location == "" {
		return "", errors.New("IdP has no HTTP-Redirect SSO endpoint")
	}
	req, err := sp.MakeAuthenticationRequest(location, saml.HTTPRedirectBinding, saml.HTTPPostBinding)
	if err != nil {
		return "", errors.Wrap(err, "make authn request")
	}
	state.RequestID = req.ID
	u, err := req.Redirect(state.State, sp)
	if err != nil {
		return "", errors.Wrap(err, "build redirect")
	}
	return u.String(), nil
}

func (p *samlProvider) Complete(ctx context.Context, r *http.Request, state *State) (*Identity, error) {
	if err := r.ParseForm(); err != nil {
		return nil, Errorf(CodeIdPError, errors.Wrap(err, "parse form"))
	}
	idpInitiated := state == nil
	if idpInitiated && !p.config.AllowIdPInitiated {
		return nil, Errorf(CodeInvalidState, errors.New("IdP-initiated sign-in is not allowed"))
	}
	sp, err := p.serviceProvider(ctx, true, idpInitiated)
	if err != nil {
		return nil, Errorf(CodeIdPError, err)
	}
	var requestIDs []string
	if state != nil {
		requestIDs = []string{state.RequestID}
	}
	assertion, err := sp.ParseResponse(r, requestIDs)
	if err != nil {
		var invalid *saml.InvalidResponseError
		if errors.As(err, &invalid) && invalid.PrivateErr != nil {
			err = invalid.PrivateErr
		}
		return nil, Errorf(CodeIdPError, errors.Wrap(err, "parse SAML response"))
	}

	if p.opts.MarkAssertion != nil {
		until := time.Now().Add(time.Hour)
		if assertion.Conditions != nil && !assertion.Conditions.NotOnOrAfter.IsZero() {
			until = assertion.Conditions.NotOnOrAfter
		}
		fresh, err := p.opts.MarkAssertion(ctx, strconv.FormatInt(p.provider.ID, 10)+":"+assertion.ID, until)
		if err != nil {
			return nil, errors.Wrap(err, "mark assertion")
		}
		if !fresh {
			return nil, Errorf(CodeInvalidState, errors.New("assertion has been used"))
		}
	}
	return p.identity(assertion)
}

func (p *samlProvider) identity(assertion *saml.Assertion) (*Identity, error) {
	if assertion.Subject == nil || assertion.Subject.NameID == nil || strings.TrimSpace(assertion.Subject.NameID.Value) == "" {
		return nil, Errorf(CodeIdPError, errors.New("missing NameID"))
	}
	nameID := assertion.Subject.NameID
	attrs := map[string][]string{}
	for _, statement := range assertion.AttributeStatements {
		for _, attr := range statement.Attributes {
			values := make([]string, 0, len(attr.Values))
			for _, v := range attr.Values {
				if s := strings.TrimSpace(v.Value); s != "" {
					values = append(values, s)
				}
			}
			for _, key := range []string{attr.Name, attr.FriendlyName} {
				if key != "" {
					attrs[key] = append(attrs[key], values...)
				}
			}
		}
	}
	lookup := func(configured string, candidates []string) []string {
		if configured != "" {
			return attrs[configured]
		}
		for _, c := range candidates {
			if v := attrs[c]; len(v) > 0 {
				return v
			}
		}
		return nil
	}
	first := func(values []string) string {
		if len(values) == 0 {
			return ""
		}
		return values[0]
	}

	email := first(lookup(p.config.EmailAttribute, samlEmailAttributes))
	if email == "" && (nameID.Format == string(saml.EmailAddressNameIDFormat) || strings.Contains(nameID.Value, "@")) {
		email = nameID.Value
	}
	return &Identity{
		Subject:       strings.TrimSpace(nameID.Value),
		Email:         db.NormalizeEmail(email),
		EmailVerified: p.config.TrustEmail,
		Name:          first(lookup(p.config.NameAttribute, samlNameAttributes)),
		Groups:        lookup(p.config.GroupsAttribute, samlGroupsAttributes),
	}, nil
}

// SAMLMetadata returns the SP metadata XML of the provider.
func SAMLMetadata(p *db.AuthProvider, opts Options) ([]byte, error) {
	c, s, err := LoadConfig(p)
	if err != nil {
		return nil, err
	}
	sp, err := newSAML(p, c, s, opts)
	if err != nil {
		return nil, err
	}
	return sp.Metadata()
}

// ProbeSAML loads the IdP metadata and checks that it has an HTTP-Redirect SSO endpoint.
func ProbeSAML(ctx context.Context, c Config) error {
	p := &samlProvider{config: c, opts: Options{}}
	entity, err := p.idpMetadata(ctx)
	if err != nil {
		return err
	}
	sp := &saml.ServiceProvider{IDPMetadata: entity}
	if sp.GetSSOBindingLocation(saml.HTTPRedirectBinding) == "" {
		return errors.New("IdP has no HTTP-Redirect SSO endpoint")
	}
	return nil
}

// GenerateSPCertificate generates a self-signed SP certificate valid for 10 years and its RSA private key in PEM.
func GenerateSPCertificate(commonName string) (certPEM, keyPEM string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", errors.Wrap(err, "generate key")
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return "", "", errors.Wrap(err, "generate serial")
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return "", "", errors.Wrap(err, "create certificate")
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	return certPEM, keyPEM, nil
}

func parseKeyPair(certPEM, keyPEM string) (*x509.Certificate, crypto.Signer, error) {
	certBlock, _ := pem.Decode([]byte(certPEM))
	if certBlock == nil {
		return nil, nil, errors.New("invalid SP certificate PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, errors.Wrap(err, "parse SP certificate")
	}
	keyBlock, _ := pem.Decode([]byte(keyPEM))
	if keyBlock == nil {
		return nil, nil, errors.New("invalid SP private key PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, errors.Wrap(err, "parse SP private key")
	}
	return cert, key, nil
}
