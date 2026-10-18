package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/shortcut/script"
	"github.com/wuhan005/sayrud/internal/workflow/sandbox"
)

const (
	maxPluginFetches       = 20
	maxPluginFetchBody     = 5 << 20
	maxPluginFetchResponse = 5 << 20
)

// PluginFetcher sends the requests of ctx.fetch in plugin nodes; Actions that do not implement it can not fetch.
// Implementations must also enforce the allowed domains on redirects.
type PluginFetcher interface {
	PluginFetch(ctx context.Context, req sandbox.FetchRequest, domains []string) (*sandbox.FetchResponse, error)
}

// pluginImpl returns the implementation of an installed plugin; action plugins only compute their config in simulated debugging unless they export preview.
func pluginImpl(stepType string) (moduleImpl, bool) {
	p := plugins.get(stepType)
	if p == nil {
		return moduleImpl{}, false
	}
	impl := moduleImpl{exec: func(x *execution, step *Step) (interface{}, error) { return x.runPlugin(step, p, false) }}
	if p.manifest.Action {
		impl.preview = func(x *execution, step *Step) (interface{}, error) { return x.runPlugin(step, p, true) }
	}
	return impl, true
}

func (x *execution) runPlugin(step *Step, p *installedPlugin, preview bool) (interface{}, error) {
	if !p.enabled {
		return nil, &ActionError{Message: fmt.Sprintf("the node %s is disabled", p.base.Label)}
	}
	program, err := p.code.compile(x.ctx)
	if err != nil {
		return nil, &ActionError{Message: "the node can not be loaded: " + err.Error()}
	}
	config, err := x.pluginConfig(step, p.base, preview)
	if err != nil {
		return nil, err
	}
	hook := sandbox.HookExecute
	if preview {
		if !program.Has(sandbox.HookPreview) {
			return config, nil
		}
		hook = sandbox.HookPreview
	}

	call := sandbox.Call{
		Hook:     hook,
		Args:     []interface{}{config},
		Context:  x.pluginContext(step, p, preview),
		Timeout:  time.Duration(p.manifest.Timeout) * time.Second,
		Location: x.location(),
	}
	if fetcher, ok := x.actions.(PluginFetcher); ok && p.manifest.Action && !preview {
		call.Fetch = pluginFetch(fetcher, p.manifest.Permissions.Fetch)
	}
	result, err := program.Run(x.ctx, call)
	x.logs = result.Logs
	if err != nil {
		return nil, pluginError(err, result.Fetches > 0, p.manifest.Timeout)
	}
	return result.Value, nil
}

func (x *execution) location() *time.Location {
	tz, _ := x.scope.Lookup("run.timezone")
	if name, ok := tz.(string); ok && name != "" {
		if loc, err := time.LoadLocation(name); err == nil {
			return loc
		}
	}
	return time.UTC
}

func (x *execution) pluginContext(step *Step, p *installedPlugin, preview bool) map[string]interface{} {
	run, _ := x.scope.Lookup("run")
	return map[string]interface{}{
		"run":     run,
		"step":    map[string]interface{}{"id": step.ID, "name": step.Name},
		"project": map[string]interface{}{"uid": x.project.UID, "name": x.project.Name},
		"plugin":  map[string]interface{}{"id": p.manifest.ID, "version": p.manifest.Version},
		"preview": preview,
	}
}

// pluginConfig resolves the config by the widgets of the module: values are evaluated, headers become an object, bodies are built and credentials are replaced by their values.
// In preview the credentials only carry their UID and name, so the output never contains secrets.
func (x *execution) pluginConfig(step *Step, def *ModuleDef, preview bool) (map[string]interface{}, error) {
	raw := rawConfig(step)
	defaults := map[string]json.RawMessage{}
	_ = json.Unmarshal(def.Defaults, &defaults)
	for k, v := range defaults {
		if _, ok := raw[k]; !ok {
			raw[k] = v
		}
	}

	widgets := map[string]FieldDef{}
	for _, s := range def.Sections {
		for _, f := range s.Fields {
			widgets[f.Key] = f
		}
	}
	out := make(map[string]interface{}, len(raw))
	for key, v := range raw {
		f, ok := widgets[key]
		if !ok {
			var plain interface{}
			_ = json.Unmarshal(v, &plain)
			out[key] = plain
			continue
		}
		resolved, err := x.pluginValue(f, v, preview)
		if err != nil {
			return nil, err
		}
		out[key] = resolved
	}
	normalized, _ := normalize(out).(map[string]interface{})
	return normalized, nil
}

func (x *execution) pluginValue(f FieldDef, v json.RawMessage, preview bool) (interface{}, error) {
	if string(v) == "null" {
		return nil, nil
	}
	switch f.Widget {
	case "value":
		var val Value
		if err := json.Unmarshal(v, &val); err != nil {
			return nil, nil
		}
		return x.scope.Resolve(&val)
	case "headers":
		var headers []Header
		_ = json.Unmarshal(v, &headers)
		out := map[string]interface{}{}
		for i := range headers {
			h := &headers[i]
			if h.Name == "" {
				continue
			}
			value, err := x.scope.Resolve(&h.Value)
			if err != nil {
				return nil, err
			}
			out[h.Name] = strings.NewReplacer("\r", "", "\n", "").Replace(Text(value))
		}
		return out, nil
	case "body":
		var body Body
		if err := json.Unmarshal(v, &body); err != nil {
			return nil, nil
		}
		return x.scope.BuildBody(&body)
	case "credential":
		var uid string
		_ = json.Unmarshal(v, &uid)
		return x.pluginCredential(uid, f.CredentialType, preview)
	}
	var plain interface{}
	_ = json.Unmarshal(v, &plain)
	return plain, nil
}

// pluginCredential returns the values of the credential, which must belong to the project and be of the type.
func (x *execution) pluginCredential(uid, credType string, preview bool) (interface{}, error) {
	if uid == "" {
		return nil, nil
	}
	c, err := db.NewCredentialsStore(x.db).GetByUID(x.ctx, uid)
	if err != nil {
		if errors.Is(err, db.ErrCredentialNotFound) {
			return nil, &ActionError{Message: "the credential does not exist"}
		}
		return nil, err
	}
	if c.ProjectID != x.project.ID || c.Type != credType {
		return nil, &ActionError{Message: "the credential does not exist"}
	}
	if preview {
		return map[string]interface{}{"uid": c.UID, "name": c.Name}, nil
	}
	secrets, err := OpenSecrets(c.Sealed)
	if err != nil {
		return nil, &ActionError{Message: "the credential can not be decrypted"}
	}
	return CredentialValues(c.Data, secrets), nil
}

// pluginError maps the failures of a plugin to action errors; a timeout after a request was sent has an uncertain outcome.
func pluginError(err error, fetched bool, timeout int) error {
	var scriptErr *sandbox.Error
	var actionErr *ActionError
	switch {
	case errors.As(err, &actionErr):
		return actionErr
	case errors.As(err, &scriptErr):
		return &ActionError{Message: scriptErr.Message, Transient: scriptErr.Retryable}
	case errors.Is(err, sandbox.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return &ActionError{Message: fmt.Sprintf("the node timed out after %d seconds", timeout), Transient: !fetched, Uncertain: fetched}
	case errors.Is(err, context.Canceled):
		return &ActionError{Message: "the node was canceled", Uncertain: fetched}
	case errors.Is(err, sandbox.ErrMemoryLimit), errors.Is(err, sandbox.ErrPendingPromise), errors.Is(err, sandbox.ErrResultTooLarge), errors.Is(err, sandbox.ErrFetchDisabled):
		return &ActionError{Message: err.Error()}
	}
	return err
}

// pluginFetch checks the requests of a plugin against its allowed hosts before sending them.
func pluginFetch(fetcher PluginFetcher, domains []string) func(ctx context.Context, req sandbox.FetchRequest) (*sandbox.FetchResponse, error) {
	count := 0
	return func(ctx context.Context, req sandbox.FetchRequest) (*sandbox.FetchResponse, error) {
		count++
		if count > maxPluginFetches {
			return nil, &ActionError{Message: fmt.Sprintf("fetch: at most %d requests are allowed in a call", maxPluginFetches)}
		}
		u, err := url.Parse(req.URL)
		if err != nil {
			return nil, &ActionError{Message: fmt.Sprintf("fetch: invalid URL %q", req.URL)}
		}
		if err := checkPluginFetchURL(u, domains); err != nil {
			return nil, err
		}
		if len(req.Body) > maxPluginFetchBody {
			return nil, &ActionError{Message: "fetch: the request body is too large"}
		}
		req.Method = strings.ToUpper(strings.TrimSpace(req.Method))
		if req.Method == "" {
			req.Method = http.MethodGet
		}
		return fetcher.PluginFetch(ctx, req, domains)
	}
}

func checkPluginFetchURL(u *url.URL, domains []string) error {
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return &ActionError{Message: fmt.Sprintf("fetch: invalid URL %q", u.String())}
	}
	if u.User != nil {
		return &ActionError{Message: "fetch: credentials in the URL are not allowed"}
	}
	if !PluginHostAllowed(u.Hostname(), domains) {
		return &ActionError{Message: fmt.Sprintf("fetch: the host %q is not allowed by the plugin permissions", u.Hostname())}
	}
	return nil
}

// PluginHostAllowed reports whether a plugin with the allowed hosts can fetch the host; * allows any host.
func PluginHostAllowed(host string, domains []string) bool {
	for _, d := range domains {
		if d == "*" {
			return true
		}
	}
	return script.HostAllowed(host, domains)
}

// PluginFetch enforces the allowed domains and system network policy; responses of any status are returned.
func (a *DefaultActions) PluginFetch(ctx context.Context, req sandbox.FetchRequest, domains []string) (*sandbox.FetchResponse, error) {
	dialer := guardedDialer(networkAllowlist(ctx, a.settings()))
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
		},
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return checkPluginFetchURL(r.URL, domains)
		},
	}
	defer client.CloseIdleConnections()

	var body io.Reader
	if req.Body != "" {
		body = strings.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return nil, &ActionError{Message: fmt.Sprintf("fetch: invalid request: %v", err)}
	}
	if err := checkPluginFetchURL(httpReq.URL, domains); err != nil {
		return nil, err
	}
	httpReq.Header.Set("User-Agent", "Sayrud-Workflow/1.0")
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}
	if req.JSONBody && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		var actionErr *ActionError
		if errors.As(err, &actionErr) {
			return nil, actionErr
		}
		var blocked *blockedAddressError
		if errors.As(err, &blocked) {
			return nil, &ActionError{Message: fmt.Sprintf("fetch: %v", blocked)}
		}
		return nil, &ActionError{Message: fmt.Sprintf("fetch: %v", err), Transient: true}
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxPluginFetchResponse+1))
	if err != nil {
		return nil, &ActionError{Message: fmt.Sprintf("fetch: read the response: %v", err), Transient: true}
	}
	if len(raw) > maxPluginFetchResponse {
		return nil, &ActionError{Message: "fetch: the response body is too large"}
	}
	headers := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		headers[strings.ToLower(k)] = strings.Join(v, ", ")
	}
	return &sandbox.FetchResponse{
		Status: resp.StatusCode, StatusText: http.StatusText(resp.StatusCode), URL: resp.Request.URL.String(), Headers: headers, Body: string(raw),
	}, nil
}
