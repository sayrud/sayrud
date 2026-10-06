package script

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/wuhan005/sayrud/internal/db"
)

const (
	maxRequests     = 20
	maxRedirects    = 5
	maxRequestBody  = 5 << 20
	maxResponseBody = 5 << 20
)

type fetchRequest struct {
	URL           string
	Method        string
	Headers       map[string]string
	Body          string
	JSONBody      bool
	CredentialKey string
}

type fetchResponse struct {
	Status     int
	StatusText string
	URL        string
	Headers    map[string]string
	Body       string
}

type fetcher struct {
	ctx         context.Context
	domains     []string
	networks    []netip.Prefix
	credentials map[string]Credential
	client      *http.Client
	requests    int
}

func newFetcher(ctx context.Context, domains []string, credentials []Credential, networks []netip.Prefix) *fetcher {
	f := &fetcher{
		ctx:         ctx,
		networks:    networks,
		credentials: make(map[string]Credential, len(credentials)),
	}
	for _, d := range domains {
		if normalized, ok := NormalizeDomain(d); ok {
			f.domains = append(f.domains, normalized)
		}
	}
	for _, c := range credentials {
		f.credentials[c.Key] = c
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: f.dialControl}
	f.client = &http.Client{
		Transport: &http.Transport{
			// A proxy would make the address check meaningless.
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			MaxIdleConns:          4,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			return f.checkURL(req.URL)
		},
	}
	return f
}

var domainPattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)*[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// NormalizeDomain returns the lowercase host name without the trailing dot, ok is false if it is not a host name or an IP address.
func NormalizeDomain(domain string) (string, bool) {
	d := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if d == "" || len(d) > 253 {
		return "", false
	}
	if _, err := netip.ParseAddr(d); err == nil {
		return d, true
	}
	return d, domainPattern.MatchString(d)
}

// HostAllowed reports whether the host equals one of the domains or is a subdomain of it.
func HostAllowed(host string, domains []string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" {
		return false
	}
	for _, d := range domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

func (f *fetcher) checkURL(u *url.URL) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.Errorf("fetch: unsupported protocol %q", u.Scheme)
	}
	if u.User != nil {
		return errors.New("fetch: credentials in the URL are not allowed")
	}
	if !HostAllowed(u.Hostname(), f.domains) {
		return errors.Errorf("fetch: the host %q is not in the allowed domains", u.Hostname())
	}
	return nil
}

func (f *fetcher) do(r fetchRequest) (*fetchResponse, error) {
	f.requests++
	if f.requests > maxRequests {
		return nil, errors.Errorf("fetch: at most %d requests are allowed in a run", maxRequests)
	}

	u, err := url.Parse(r.URL)
	if err != nil {
		return nil, errors.Errorf("fetch: invalid URL %q", r.URL)
	}
	if err := f.checkURL(u); err != nil {
		return nil, err
	}
	if len(r.Body) > maxRequestBody {
		return nil, errors.New("fetch: the request body is too large")
	}

	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method == "" {
		method = http.MethodGet
	}

	var credential *Credential
	if r.CredentialKey != "" {
		c, ok := f.credentials[r.CredentialKey]
		if !ok {
			return nil, errors.Errorf("fetch: unknown credential %q", r.CredentialKey)
		}
		credential = &c
		if c.Type == db.ShortcutCredentialQuery {
			query := u.Query()
			query.Set(c.Name, c.Value)
			u.RawQuery = query.Encode()
		}
	}

	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	req, err := http.NewRequestWithContext(f.ctx, method, u.String(), body)
	if err != nil {
		return nil, errors.Errorf("fetch: invalid request: %v", err)
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	if r.JSONBody && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if credential != nil {
		switch credential.Type {
		case db.ShortcutCredentialBearer:
			req.Header.Set("Authorization", "Bearer "+credential.Value)
		case db.ShortcutCredentialHeader:
			req.Header.Set(credential.Name, credential.Value)
		}
	}

	resp, err := f.client.Do(req)
	if err != nil {
		if f.ctx.Err() != nil {
			return nil, f.ctx.Err()
		}
		return nil, errors.Errorf("fetch: %v", redactURLError(err))
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return nil, errors.Errorf("fetch: read the response: %v", err)
	}
	if len(raw) > maxResponseBody {
		return nil, errors.New("fetch: the response body is too large")
	}

	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		headers[strings.ToLower(k)] = strings.Join(v, ", ")
	}
	finalURL := resp.Request.URL
	if credential != nil && credential.Type == db.ShortcutCredentialQuery {
		finalURL = redactQuery(finalURL, credential.Name)
	}
	return &fetchResponse{
		Status:     resp.StatusCode,
		StatusText: http.StatusText(resp.StatusCode),
		URL:        finalURL.String(),
		Headers:    headers,
		Body:       string(raw),
	}, nil
}

// redactURLError drops the URL from the error of the client, which may contain the query credential.
func redactURLError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

func redactQuery(u *url.URL, name string) *url.URL {
	copied := *u
	query := copied.Query()
	if query.Has(name) {
		query.Set(name, "******")
		copied.RawQuery = query.Encode()
	}
	return &copied
}

// dialControl checks the resolved address on every connection, including redirects, to block DNS rebinding.
func (f *fetcher) dialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return errors.Wrap(err, "split address")
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return errors.Wrap(err, "parse address")
	}
	addr = addr.Unmap()
	if IsPublicAddr(addr) {
		return nil
	}
	for _, prefix := range f.networks {
		if prefix.Contains(addr) {
			return nil
		}
	}
	return errors.Errorf("the address %s is not public and is not in the network allowlist", addr)
}

var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
}

// IsPublicAddr reports whether the address is globally routable, the loopback, private, link-local and reserved ranges are not.
func IsPublicAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() || addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	for _, prefix := range nonPublicPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}
