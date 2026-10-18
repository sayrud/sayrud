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
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Calcium-Ion/moejs"
	"github.com/cockroachdb/errors"
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
	maxResultSize   = 1 << 20
	maxLogs         = 200
	maxLogLength    = 2000
	memoryLimit     = 64 << 20
	loadTimeout     = 2 * time.Second
	defaultTimeout  = 10 * time.Second
)

var (
	ErrNoManifest     = errors.New("the module does not export a manifest object")
	ErrNoExecute      = errors.New("the module does not export an execute function")
	ErrImports        = errors.New("the module can not import other modules")
	ErrTimeout        = errors.New("the plugin timed out")
	ErrMemoryLimit    = errors.New("the plugin exceeded the memory limit")
	ErrPendingPromise = errors.New("the promise returned by the plugin never settles")
	ErrResultTooLarge = errors.New("the result of the plugin is too large")
	ErrFetchDisabled  = errors.New("fetch is not available for this node")
)

// Error is an error thrown by the plugin or a syntax error; Message can be shown to the workflow author.
type Error struct {
	Message string
	// Retryable is set when the plugin throws an object whose retryable property is true.
	Retryable bool
}

func (e *Error) Error() string {
	return e.Message
}

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
	s := newState(runCtx, Call{})
	rt, release, err := s.runtime(p)
	if err != nil {
		return nil, err
	}
	defer release()

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
		return nil, s.wrap(err)
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

	s := newState(runCtx, c)
	value, err := s.call(p, hook)
	result := &Result{Logs: s.logs, Fetches: s.fetches}
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
	rt      *moejs.Runtime
	logs    []string
	fetches int
}

func newState(ctx context.Context, c Call) *state {
	return &state{ctx: ctx, opts: c, logs: []string{}}
}

// runtime returns a new runtime with the module loaded; release stops the interrupt watcher.
func (s *state) runtime(p *Program) (*moejs.Runtime, func(), error) {
	loc := s.opts.Location
	if loc == nil {
		loc = time.UTC
	}
	rt := moejs.NewRuntime(moejs.Options{DisableDynamicCode: true, MemoryLimit: memoryLimit, TimeZone: loc})
	s.rt = rt

	interrupted := make(chan struct{})
	stop := context.AfterFunc(s.ctx, func() {
		rt.Interrupt(context.Cause(s.ctx))
		close(interrupted)
	})
	release := func() {
		if !stop() {
			<-interrupted
		}
	}

	log := moejs.NativeFunc(s.log)
	console := map[string]any{"log": log, "info": log, "warn": log, "error": log, "debug": log}
	if err := rt.SetGlobal("console", console); err != nil {
		release()
		return nil, nil, errors.Wrap(err, "set console")
	}
	if err := rt.Load(p.mod); err != nil {
		release()
		return nil, nil, s.wrap(err)
	}
	return rt, release, nil
}

func (s *state) call(p *Program, hook moejs.Hook) (interface{}, error) {
	rt, release, err := s.runtime(p)
	if err != nil {
		return nil, err
	}
	defer release()

	args := make([]moejs.Value, 0, len(s.opts.Args)+1)
	for _, a := range s.opts.Args {
		v, err := s.parse(a)
		if err != nil {
			return nil, errors.Wrap(err, "convert argument")
		}
		args = append(args, v)
	}
	ctxValue, err := s.contextValue()
	if err != nil {
		return nil, errors.Wrap(err, "build context")
	}
	args = append(args, ctxValue)

	res, err := rt.Call(hook, args...)
	if err != nil {
		return nil, s.wrap(err)
	}
	if state, result, ok := moejs.PromiseResult(res); ok {
		switch state {
		case moejs.PromiseFulfilled:
			res = result
		case moejs.PromiseRejected:
			return nil, s.thrown(result)
		default:
			if s.ctx.Err() != nil {
				return nil, s.ctxError()
			}
			return nil, ErrPendingPromise
		}
	}
	return s.toGo(res)
}

// parse converts a Go value through JSON, so the plugin only sees plain objects.
func (s *state) parse(v interface{}) (moejs.Value, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return moejs.Undefined(), err
	}
	return s.rt.ParseJSON(raw)
}

func (s *state) contextValue() (moejs.Value, error) {
	data := map[string]any{}
	if s.opts.Context != nil {
		raw, err := json.Marshal(s.opts.Context)
		if err != nil {
			return moejs.Undefined(), err
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			return moejs.Undefined(), err
		}
	}
	data["log"] = moejs.NativeFunc(s.log)
	data["fetch"] = moejs.NativeFunc(s.fetch)
	return s.rt.FromGo(data)
}

func (s *state) toGo(v moejs.Value) (interface{}, error) {
	if v.IsUndefined() || v.IsNull() {
		return nil, nil
	}
	raw, err := s.rt.AppendJSON(nil, v)
	if err != nil {
		return nil, s.wrap(err)
	}
	if len(raw) > maxResultSize {
		return nil, ErrResultTooLarge
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, errors.Wrap(err, "decode result")
	}
	return out, nil
}

func (s *state) log(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if len(s.logs) >= maxLogs {
		return moejs.Undefined(), nil
	}
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, s.describe(a))
	}
	line := strings.Join(parts, " ")
	if utf8.RuneCountInString(line) > maxLogLength {
		line = string([]rune(line)[:maxLogLength]) + "…"
	}
	s.logs = append(s.logs, line)
	return moejs.Undefined(), nil
}

// describe returns the text of a value for the logs and the errors without running plugin code for errors.
func (s *state) describe(v moejs.Value) string {
	switch {
	case v.IsUndefined():
		return "undefined"
	case v.IsString():
		return v.String()
	case v.IsObject():
		exc := &moejs.Exception{Value: v}
		if msg := exc.Message(); msg != "" {
			if name := exc.Name(); name != "" {
				return name + ": " + msg
			}
			return msg
		}
		if raw, err := s.rt.AppendJSON(nil, v); err == nil {
			return string(raw)
		}
	}
	return v.String()
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

func (s *state) ctxError() error {
	if errors.Is(s.ctx.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	return s.ctx.Err()
}

// wrap converts the errors of the engine: interrupts become timeouts or the memory limit, throws become *Error unless they carry a host error.
func (s *state) wrap(err error) error {
	var interrupted *moejs.InterruptedError
	if errors.As(err, &interrupted) {
		if errors.Is(err, moejs.ErrMemoryLimit) {
			return ErrMemoryLimit
		}
		if s.ctx.Err() != nil {
			return s.ctxError()
		}
		return ErrTimeout
	}
	var exc *moejs.Exception
	if errors.As(err, &exc) {
		return s.thrown(exc.Value)
	}
	var syntaxErr *moejs.SyntaxError
	if errors.As(err, &syntaxErr) {
		return &Error{Message: syntaxErr.Error()}
	}
	return &Error{Message: err.Error()}
}

// thrown converts a thrown or rejected value; the Go error of a host function is returned as is.
func (s *state) thrown(v moejs.Value) error {
	exc := &moejs.Exception{Value: v}
	if hostErr := exc.Unwrap(); hostErr != nil {
		return hostErr
	}
	e := &Error{Message: s.describe(v)}
	if v.IsObject() {
		if retry := s.mustGet(v, "retryable"); retry.IsBool() && retry.AsBool() {
			e.Retryable = true
		}
	}
	return e
}
