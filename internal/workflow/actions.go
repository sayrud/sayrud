package workflow

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/netip"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/shortcut/script"
	"github.com/wuhan005/sayrud/internal/sso"
)

// ActionError is a failed external action; Message can be shown to the workflow author.
type ActionError struct {
	Message string
	// Transient reports whether the action can be retried, e.g. a connection failure or a server 5xx.
	Transient bool
	// Uncertain reports whether the action may have taken effect, e.g. a timeout waiting for confirmation after the mail content was sent.
	Uncertain bool
}

func (e *ActionError) Error() string {
	return e.Message
}

// Mail is an email to be sent.
type Mail struct {
	To      []string
	CC      []string
	Subject string
	Content string
	// MessageID is derived from the run and step so that recipients can detect duplicate deliveries.
	MessageID string
}

// HTTPRequest is an HTTP request to be sent.
type HTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    interface{}
	Timeout time.Duration
	// IdempotencyKey is derived from the run and step and sent in the Idempotency-Key header.
	IdempotencyKey string
}

// HTTPResult is the result of an HTTP request; Body keeps its structure when it is JSON and is text otherwise.
type HTTPResult struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    interface{}       `json:"body"`
}

const (
	maxRecipients     = 20
	maxHTTPResponse   = 1 << 20
	defaultHTTPMethod = http.MethodPost
	mailTimeout       = 30 * time.Second
)

func (x *execution) email(step *Step) (interface{}, error) {
	if x.actions == nil {
		return nil, &ActionError{Message: "external actions are not available"}
	}
	to, err := x.recipients(step.To)
	if err != nil {
		return nil, err
	}
	if len(to) == 0 {
		return nil, &ActionError{Message: "no recipient"}
	}
	cc, err := x.recipients(step.CC)
	if err != nil {
		return nil, err
	}
	subject, _ := x.scope.Resolve(step.Subject)
	content, _ := x.scope.Resolve(step.Content)
	runID, _ := x.scope.Lookup("run.id")
	m := Mail{To: to, CC: cc, Subject: Text(subject), Content: Text(content), MessageID: fmt.Sprintf("<%s.%s@sayrud>", Text(runID), step.ID)}
	if err := x.actions.SendEmail(x.ctx, m); err != nil {
		return nil, err
	}
	return map[string]interface{}{"to": to, "cc": cc, "subject": m.Subject, "messageID": m.MessageID}, nil
}

// previewAction returns the arguments the external action would use without performing it.
func (x *execution) previewAction(step *Step) (interface{}, error) {
	if step.Type == StepEmail {
		to, err := x.recipients(step.To)
		if err != nil {
			return nil, err
		}
		cc, err := x.recipients(step.CC)
		if err != nil {
			return nil, err
		}
		subject, _ := x.scope.Resolve(step.Subject)
		content, _ := x.scope.Resolve(step.Content)
		return map[string]interface{}{"to": to, "cc": cc, "subject": Text(subject), "content": Text(content)}, nil
	}
	rawURL, _ := x.scope.Resolve(step.URL)
	method := strings.ToUpper(step.HTTPMethod)
	if method == "" {
		method = defaultHTTPMethod
	}
	body, err := x.scope.BuildBody(step.Body)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"method": method, "url": Text(rawURL), "body": body}, nil
}

func (x *execution) recipients(v *Value) ([]string, error) {
	raw, err := x.scope.Resolve(v)
	if err != nil {
		return nil, err
	}
	var parts []string
	switch t := raw.(type) {
	case string:
		parts = strings.FieldsFunc(t, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	case []interface{}:
		for _, item := range t {
			parts = append(parts, Text(item))
		}
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		addr, err := mail.ParseAddress(p)
		if err != nil {
			return nil, &ActionError{Message: fmt.Sprintf("invalid email address %q", p)}
		}
		out = append(out, addr.Address)
	}
	if len(out) > maxRecipients {
		return nil, &ActionError{Message: fmt.Sprintf("at most %d recipients are allowed", maxRecipients)}
	}
	return out, nil
}

func (x *execution) fetch(step *Step) (interface{}, error) {
	if x.actions == nil {
		return nil, &ActionError{Message: "external actions are not available"}
	}
	rawURL, _ := x.scope.Resolve(step.URL)
	headers := map[string]string{}
	for i := range step.Headers {
		h := &step.Headers[i]
		if h.Name == "" {
			continue
		}
		v, _ := x.scope.Resolve(&h.Value)
		headers[h.Name] = strings.NewReplacer("\r", "", "\n", "").Replace(Text(v))
	}
	var body interface{}
	if step.Body != nil {
		b, err := x.scope.BuildBody(step.Body)
		if err != nil {
			return nil, err
		}
		body = b
	}
	timeout := time.Duration(min(max(step.Timeout, 1), 30)) * time.Second
	if step.Timeout == 0 {
		timeout = 10 * time.Second
	}
	runID, _ := x.scope.Lookup("run.id")
	method := strings.ToUpper(step.HTTPMethod)
	if method == "" {
		method = defaultHTTPMethod
	}
	result, err := x.actions.Fetch(x.ctx, HTTPRequest{
		Method: method, URL: Text(rawURL), Headers: headers, Body: body, Timeout: timeout,
		IdempotencyKey: Text(runID) + "-" + step.ID,
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// DefaultActions sends emails through the SMTP server configured in the admin console and sends HTTP requests with a client that cannot reach private networks.
type DefaultActions struct {
	Settings func() db.SettingsStore
}

func (a *DefaultActions) settings() db.SettingsStore {
	if a.Settings != nil {
		return a.Settings()
	}
	return db.Settings
}

func (a *DefaultActions) SendEmail(ctx context.Context, m Mail) error {
	s, err := a.settings().GetMail(ctx)
	if err != nil {
		return errors.Wrap(err, "get mail settings")
	}
	if !s.Configured() {
		return &ActionError{Message: "email sending is not configured by the administrator"}
	}
	password := ""
	if s.SealedPassword != "" {
		plain, err := sso.Open(s.SealedPassword)
		if err != nil {
			return &ActionError{Message: "the SMTP password can not be decrypted"}
		}
		password = string(plain)
	}
	return SendSMTP(ctx, *s, password, m)
}

// SendSMTP sends an email; failures before the content is submitted are retryable, while failures while awaiting confirmation after submission have an uncertain outcome.
func SendSMTP(ctx context.Context, s db.MailSettings, password string, m Mail) error {
	transient := func(format string, args ...interface{}) error {
		return &ActionError{Message: fmt.Sprintf(format, args...), Transient: true}
	}
	address := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	var err error
	if s.Security == db.MailSecurityTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return transient("connect to the SMTP server: %v", err)
	}
	deadline := time.Now().Add(mailTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	client, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		_ = conn.Close()
		return transient("greet the SMTP server: %v", err)
	}
	defer func() { _ = client.Close() }()

	if s.Security == db.MailSecurityStartTLS {
		if err := client.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return transient("start TLS: %v", err)
		}
	}
	if s.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.Username, password, s.Host)); err != nil {
			return &ActionError{Message: fmt.Sprintf("SMTP authentication failed: %v", err)}
		}
	}
	if err := client.Mail(s.FromAddress); err != nil {
		return &ActionError{Message: fmt.Sprintf("the sender is rejected: %v", err)}
	}
	for _, rcpt := range append(append([]string{}, m.To...), m.CC...) {
		if err := client.Rcpt(rcpt); err != nil {
			return &ActionError{Message: fmt.Sprintf("the recipient %s is rejected: %v", rcpt, err)}
		}
	}
	w, err := client.Data()
	if err != nil {
		return transient("start the message: %v", err)
	}
	if _, err := w.Write(buildMessage(s, m)); err != nil {
		_ = w.Close()
		return &ActionError{Message: fmt.Sprintf("send the message: %v", err), Uncertain: true}
	}
	if err := w.Close(); err != nil {
		return &ActionError{Message: fmt.Sprintf("the server did not confirm the message: %v", err), Uncertain: true}
	}
	_ = client.Quit()
	return nil
}

func buildMessage(s db.MailSettings, m Mail) []byte {
	from := s.FromAddress
	if s.FromName != "" {
		from = (&mail.Address{Name: s.FromName, Address: s.FromAddress}).String()
	}
	messageID := m.MessageID
	if messageID == "" {
		messageID = "<" + randstr.String(16) + "@sayrud>"
	}
	var buf bytes.Buffer
	header := func(k, v string) { buf.WriteString(k + ": " + v + "\r\n") }
	header("From", from)
	header("To", strings.Join(m.To, ", "))
	if len(m.CC) > 0 {
		header("Cc", strings.Join(m.CC, ", "))
	}
	header("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	header("Date", time.Now().Format(time.RFC1123Z))
	header("Message-ID", messageID)
	header("MIME-Version", "1.0")
	header("Content-Type", "text/plain; charset=UTF-8")
	header("Content-Transfer-Encoding", "base64")
	buf.WriteString("\r\n")
	encoded := base64.StdEncoding.EncodeToString([]byte(m.Content))
	for len(encoded) > 76 {
		buf.WriteString(encoded[:76] + "\r\n")
		encoded = encoded[76:]
	}
	buf.WriteString(encoded + "\r\n")
	return buf.Bytes()
}

func (a *DefaultActions) Fetch(ctx context.Context, req HTTPRequest) (*HTTPResult, error) {
	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return nil, &ActionError{Message: fmt.Sprintf("invalid URL %q", req.URL)}
	}
	var networks []netip.Prefix
	if s, err := a.settings().GetSystem(ctx); err == nil {
		networks, _ = db.ParseNetworkAllowlist(s.NetworkAllowlist)
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		addr, err := netip.ParseAddr(host)
		if err != nil {
			return err
		}
		addr = addr.Unmap()
		if script.IsPublicAddr(addr) {
			return nil
		}
		for _, prefix := range networks {
			if prefix.Contains(addr) {
				return nil
			}
		}
		return errors.Errorf("the address %s is not public and is not in the network allowlist", addr)
	}}
	client := &http.Client{
		Timeout: req.Timeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: req.Timeout,
		},
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}

	var body io.Reader
	if req.Body != nil && req.Method != http.MethodGet {
		raw, err := json.Marshal(req.Body)
		if err != nil {
			return nil, errors.Wrap(err, "encode body")
		}
		body = bytes.NewReader(raw)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, u.String(), body)
	if err != nil {
		return nil, &ActionError{Message: fmt.Sprintf("invalid request: %v", err)}
	}
	httpReq.Header.Set("User-Agent", "Sayrud-API/1.0")
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if req.IdempotencyKey != "" {
		httpReq.Header.Set("Idempotency-Key", req.IdempotencyKey)
	}
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, &ActionError{Message: fmt.Sprintf("request failed: %v", err), Transient: true}
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponse+1))
	if err != nil {
		return nil, &ActionError{Message: fmt.Sprintf("read the response: %v", err), Transient: true}
	}
	if len(raw) > maxHTTPResponse {
		raw = raw[:maxHTTPResponse]
	}

	result := &HTTPResult{Status: resp.StatusCode, Headers: map[string]string{}}
	for k, v := range resp.Header {
		result.Headers[strings.ToLower(k)] = strings.Join(v, ", ")
	}
	var parsed interface{}
	if json.Unmarshal(raw, &parsed) == nil {
		result.Body = parsed
	} else {
		result.Body = string(raw)
	}
	if resp.StatusCode >= 300 {
		retry := resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests
		return nil, &ActionError{Message: fmt.Sprintf("the server responded %d", resp.StatusCode), Transient: retry}
	}
	return result, nil
}
