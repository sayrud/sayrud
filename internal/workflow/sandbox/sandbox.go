// Package sandbox runs the installed workflow node plugins with the moejs JavaScript engine.
//
// A plugin is a single ES module (JavaScript or TypeScript) that exports a `manifest` object and an `execute(config, ctx)` function,
// optionally a `preview(config, ctx)` function for simulated debugging. The module is compiled once and every call runs in a new runtime,
// so no state is shared between calls. The runtime has no module loader, timers, file system or process access, and dynamic code is disabled;
// a call is interrupted on timeout or when it allocates too much memory. ctx.fetch is provided by the host and ctx.log collects the logs.
// moejs is an interpreter in the server process rather than an isolation boundary, so plugins must be installed by trusted admins.
package sandbox

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Calcium-Ion/moejs"
	"github.com/cockroachdb/errors"

	"github.com/wuhan005/sayrud/internal/jsruntime"
)

// Language is the source language of a plugin.
type Language string

const (
	JavaScript Language = "javascript"
	TypeScript Language = "typescript"
)

// Valid reports whether the language is supported.
func (l Language) Valid() bool {
	return l == JavaScript || l == TypeScript
}

// Hooks exported by a plugin module.
const (
	HookExecute = "execute"
	HookPreview = "preview"
)

const (
	// MaxSourceSize is the maximum size of a plugin source in bytes.
	MaxSourceSize   = 512 << 10
	maxManifestSize = 256 << 10
	loadTimeout     = 2 * time.Second
	defaultTimeout  = 10 * time.Second
)

var (
	ErrNoManifest     = errors.New("the module does not export a manifest object")
	ErrNoExecute      = errors.New("the module does not export an execute function")
	ErrImports        = errors.New("the module can not import other modules")
	ErrTimeout        = jsruntime.ErrTimeout
	ErrMemoryLimit    = jsruntime.ErrMemoryLimit
	ErrPendingPromise = jsruntime.ErrPendingPromise
	ErrResultTooLarge = jsruntime.ErrResultTooLarge
	ErrFetchDisabled  = errors.New("fetch is not available for this node")
)

// Error is an error thrown by the plugin or a syntax error, with an optional retry hint.
type Error = jsruntime.Error

// Program is a compiled plugin module; it is immutable and can be run concurrently.
type Program struct {
	mod      *moejs.Module
	manifest json.RawMessage
	hooks    map[string]moejs.Hook
}

// Compile compiles the source, evaluates its top level once to read the manifest, and checks that execute is exported.
func Compile(ctx context.Context, name, source string, lang Language) (*Program, error) {
	if len(source) > MaxSourceSize {
		return nil, &Error{Message: "the source is too large"}
	}

	var mod *moejs.Module
	var err error
	if lang == TypeScript {
		mod, err = moejs.CompileTS(name, source)
	} else {
		mod, err = moejs.Compile(name, source)
	}
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}

	if len(mod.Requests()) > 0 {
		return nil, ErrImports
	}

	p := &Program{mod: mod, hooks: map[string]moejs.Hook{}}
	for _, name := range []string{HookExecute, HookPreview} {
		if h, err := mod.Hook(name); err == nil {
			p.hooks[name] = h
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, loadTimeout)
	defer cancel()

	s := &state{ctx: runCtx}
	rt, err := s.runtime(p)
	if err != nil {
		return nil, err
	}
	defer rt.Close()

	for name, h := range p.hooks {
		if ok, err := rt.Has(h); err != nil || !ok {
			delete(p.hooks, name)
		}
	}
	if _, ok := p.hooks[HookExecute]; !ok {
		return nil, ErrNoExecute
	}

	value, ok := rt.Export("manifest")
	if !ok || !value.IsObject() {
		return nil, ErrNoManifest
	}

	raw, err := rt.AppendJSON(nil, value)
	if err != nil {
		return nil, rt.Wrap(err)
	}
	if len(raw) > maxManifestSize {
		return nil, &Error{Message: "the manifest is too large"}
	}

	p.manifest = raw

	return p, nil
}

// Manifest returns the JSON of the exported manifest.
func (p *Program) Manifest() json.RawMessage {
	return p.manifest
}

// Has reports whether the module exports the hook as a function.
func (p *Program) Has(hook string) bool {
	_, ok := p.hooks[hook]
	return ok
}

// FetchRequest is a request made by ctx.fetch.
type FetchRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
	// JSONBody is set when the body was given as an object and encoded as JSON.
	JSONBody bool
}

// FetchResponse is the response returned to ctx.fetch.
type FetchResponse struct {
	Status     int
	StatusText string
	URL        string
	Headers    map[string]string
	Body       string
}

// Call is a call of a plugin hook.
type Call struct {
	Hook string
	// Args are the JSON-compatible arguments before the ctx argument.
	Args []interface{}
	// Context is merged into the ctx argument, along with log and fetch.
	Context map[string]interface{}
	// Fetch implements ctx.fetch; ctx.fetch throws when it is nil.
	Fetch func(ctx context.Context, req FetchRequest) (*FetchResponse, error)
	// Timeout is the time limit of the call, 10 seconds by default.
	Timeout time.Duration
	// Location is the time zone of Date, UTC by default.
	Location *time.Location
}

// Result is the outcome of a call.
type Result struct {
	// Value is the JSON-compatible value returned or resolved by the hook, numbers are float64.
	Value interface{}
	// Logs are the lines printed by console and ctx.log.
	Logs []string
	// Fetches is the number of requests made by ctx.fetch.
	Fetches int
}

// Run calls the hook in a new runtime. The result carries the logs and the number of requests even when the call fails.
func (p *Program) Run(ctx context.Context, c Call) (*Result, error) {
	hook, ok := p.hooks[c.Hook]
	if !ok {
		return &Result{Logs: []string{}}, errors.Newf("the plugin does not export %s", c.Hook)
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	s := &state{ctx: runCtx, opts: c}
	value, err := s.call(p, hook)
	result := &Result{Logs: []string{}, Fetches: s.fetches}
	if s.rt != nil {
		result.Logs = s.rt.Logs
	}
	if err != nil {
		return result, err
	}

	result.Value = value

	return result, nil
}

// state is the host side of a single call.
type state struct {
	ctx     context.Context
	opts    Call
	rt      *jsruntime.Runtime
	fetches int
}

// runtime returns a fresh runtime with the module loaded.
func (s *state) runtime(p *Program) (*jsruntime.Runtime, error) {
	loc := s.opts.Location
	if loc == nil {
		loc = time.UTC
	}

	rt, err := jsruntime.New(s.ctx, moejs.Options{DisableDynamicCode: true, TimeZone: loc})
	if err != nil {
		return nil, err
	}
	s.rt = rt

	if err := rt.Load(p.mod); err != nil {
		rt.Close()
		return nil, rt.Wrap(err)
	}

	return rt, nil
}

func (s *state) call(p *Program, hook moejs.Hook) (interface{}, error) {
	rt, err := s.runtime(p)
	if err != nil {
		return nil, err
	}
	defer rt.Close()

	var args []moejs.Value
	for _, a := range s.opts.Args {
		v, err := rt.Parse(a)
		if err != nil {
			return nil, errors.Wrap(err, "convert argument")
		}
		args = append(args, v)
	}

	ctxValue, err := rt.Context(s.opts.Context, map[string]any{"fetch": moejs.NativeFunc(s.fetch), "crypto": cryptoFunctions()})
	if err != nil {
		return nil, errors.Wrap(err, "build context")
	}
	args = append(args, ctxValue)

	res, err := rt.Call(hook, args...)
	if err != nil {
		return nil, rt.Wrap(err)
	}

	return rt.Resolve(res)
}

func (s *state) fetch(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if s.opts.Fetch == nil {
		return moejs.Undefined(), ErrFetchDisabled
	}
	u, err := s.rt.ToGo(moejs.Arg(args, 0))
	if err != nil {
		return moejs.Undefined(), err
	}
	rawURL, ok := u.(string)
	if !ok || rawURL == "" {
		return moejs.Undefined(), errors.New("fetch: the URL must be a string")
	}
	req := FetchRequest{URL: rawURL}
	if opts := moejs.Arg(args, 1); opts.IsObject() {
		o, err := s.rt.ToGo(opts)
		if err != nil {
			return moejs.Undefined(), err
		}
		options, _ := o.(map[string]any)
		req.Method, _ = options["method"].(string)
		if headers, ok := options["headers"].(map[string]any); ok {
			req.Headers = make(map[string]string, len(headers))
			for k, v := range headers {
				if text, ok := v.(string); ok {
					req.Headers[k] = text
				} else if raw, err := json.Marshal(v); err == nil {
					req.Headers[k] = string(raw)
				}
			}
		}
		switch body := options["body"].(type) {
		case nil:
		case string:
			req.Body = body
		default:
			raw, err := s.rt.AppendJSON(nil, s.mustGet(opts, "body"))
			if err != nil {
				return moejs.Undefined(), errors.New("fetch: the body can not be encoded as JSON")
			}
			req.Body, req.JSONBody = string(raw), true
		}
	}

	s.fetches++
	resp, err := s.opts.Fetch(s.ctx, req)
	if err != nil {
		return moejs.Undefined(), err
	}
	body := resp.Body
	headers := resp.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	return s.rt.FromGo(map[string]any{
		"status":     int64(resp.Status),
		"statusText": resp.StatusText,
		"ok":         resp.Status >= 200 && resp.Status < 300,
		"url":        resp.URL,
		"headers":    headers,
		"text": moejs.NativeFunc(func(*moejs.Realm, moejs.Value, []moejs.Value) (moejs.Value, error) {
			return moejs.String(body), nil
		}),
		"json": moejs.NativeFunc(func(*moejs.Realm, moejs.Value, []moejs.Value) (moejs.Value, error) {
			return s.rt.ParseJSONString(body)
		}),
	})
}

func (s *state) mustGet(v moejs.Value, key string) moejs.Value {
	out, err := s.rt.Get(v, key)
	if err != nil {
		return moejs.Undefined()
	}
	return out
}
