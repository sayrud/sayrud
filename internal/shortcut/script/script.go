// Package script runs the custom field shortcuts written in JavaScript or TypeScript.
//
// Every run uses a new moejs runtime which has no module loader, file system or process access, and is interrupted on timeout.
// context.fetch is limited to allowed domains and public or explicitly allowed addresses; context.ai uses the injected server-side model client.
// moejs is an interpreter in the server process rather than an isolation boundary, so the scripts must be published by trusted admins.
package script

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/Calcium-Ion/moejs"
	"github.com/cockroachdb/errors"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/jsruntime"
)

// Credential is a secret the script refers to by key when fetching, the script can not read its value.
type Credential struct {
	Key   string
	Type  db.ShortcutCredentialType
	Name  string
	Value string
}

// Options are the inputs of a run.
type Options struct {
	// Code defines `async function execute(params, context)`.
	Code string
	// Language is javascript by default; typescript uses the engine's type-erasing module compiler.
	Language string
	// Params are passed as the first argument, keyed by the form item key.
	Params map[string]interface{}
	// Context is merged into the second argument, e.g. the UIDs of the table and the record.
	Context map[string]interface{}
	// Domains are the hosts allowed to fetch, including their subdomains.
	Domains []string
	// Networks permit fetches to otherwise blocked addresses, configured by the admin.
	Networks []netip.Prefix
	// Credentials are the secrets the script can use by key when fetching.
	Credentials []Credential
	// Timeout is the time limit of the run, it defaults to 30 seconds.
	Timeout time.Duration
	// AIComplete uses the global model without exposing its configuration or credentials to the script.
	AIComplete func(ctx context.Context, prompt, system string) (string, error)
}

// Result is the outcome of a successful run.
type Result struct {
	// Value is the JSON-compatible value returned by execute, nil for undefined or null.
	Value interface{}
	// Logs are the lines printed by console and context.log, with the credential values redacted.
	Logs []string
}

const scriptName = "shortcut.js"

var (
	ErrNoExecute      = errors.New("the script does not define the execute function")
	ErrTimeout        = jsruntime.ErrTimeout
	ErrMemoryLimit    = jsruntime.ErrMemoryLimit
	ErrPendingPromise = jsruntime.ErrPendingPromise
	ErrResultTooLarge = jsruntime.ErrResultTooLarge
	ErrAIDisabled     = errors.New("AI is not enabled for this shortcut")
)

// Error is an error thrown by the script or a syntax error, Message is shown to the users.
type Error = jsruntime.Error

// Compile checks classic scripts and single-file ES modules.
func Compile(code, language string) error {
	if language == "typescript" {
		_, err := compileModule(code, language)
		return err
	}
	if language != "" && language != "javascript" {
		return &Error{Message: "unsupported source language"}
	}
	if _, err := moejs.CompileScript(scriptName, code); err != nil {
		_, err = compileModule(code, language)
		return err
	}

	return nil
}

func compileModule(code, language string) (*moejs.Module, error) {
	var mod *moejs.Module
	var err error
	switch language {
	case "typescript":
		mod, err = moejs.CompileTS("shortcut.ts", code)
	case "", "javascript":
		mod, err = moejs.Compile(scriptName, code)
	default:
		return nil, &Error{Message: "unsupported source language"}
	}
	if err != nil {
		return nil, &Error{Message: err.Error()}
	}
	if len(mod.Requests()) > 0 {
		return nil, &Error{Message: "the module can not import other modules"}
	}
	if _, err := mod.Hook("execute"); err != nil {
		return nil, ErrNoExecute
	}

	return mod, nil
}

type sandbox struct {
	ctx     context.Context
	vm      *jsruntime.Runtime
	opts    Options
	fetcher *fetcher
	aiCalls int
}

// Run executes the script and returns the value returned or resolved by execute.
// The logs are returned along with the error if the run fails after the script starts.
func Run(ctx context.Context, opts Options) (*Result, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Keep classic scripts' mutable builtins, dynamic code and local Date time zone.
	vm, err := jsruntime.New(runCtx, moejs.Options{MutableIntrinsics: true})
	if err != nil {
		return nil, err
	}
	defer vm.Close()

	s := &sandbox{ctx: runCtx, vm: vm, opts: opts}
	s.fetcher = newFetcher(runCtx, opts.Domains, opts.Credentials, opts.Networks)
	defer s.fetcher.client.CloseIdleConnections()

	value, err := s.execute()
	result := &Result{}
	if len(vm.Logs) > 0 {
		result.Logs = s.redact(vm.Logs)
	}
	if err != nil {
		return result, err
	}

	result.Value = value

	return result, nil
}

func (s *sandbox) execute() (interface{}, error) {
	fn, err := s.loadExecute()
	if err != nil {
		return nil, err
	}

	params := s.opts.Params
	if params == nil {
		params = map[string]interface{}{}
	}
	arg, err := s.vm.Parse(params)
	if err != nil {
		return nil, errors.Wrap(err, "convert params")
	}
	ctxValue, err := s.vm.Context(s.opts.Context, map[string]any{
		"fetch": moejs.NativeFunc(s.fetch),
		"ai":    map[string]any{"complete": moejs.NativeFunc(s.aiComplete)},
	})
	if err != nil {
		return nil, errors.Wrap(err, "build context")
	}

	// Fetch and AI calls block, so promise jobs have settled when the outermost call returns.
	value, err := s.vm.CallFunction(fn, moejs.Undefined(), arg, ctxValue)
	if err != nil {
		return nil, s.vm.Wrap(err)
	}

	return s.vm.Resolve(value)
}

func (s *sandbox) loadExecute() (moejs.Value, error) {
	// Read execute in the same global scope without turning a classic script into a strict module.
	var program *moejs.Script
	if s.opts.Language == "" || s.opts.Language == "javascript" {
		program, _ = moejs.CompileScript(scriptName, s.opts.Code+"\n;typeof execute === 'function' ? execute : undefined;")
	}
	if program == nil {
		mod, err := compileModule(s.opts.Code, s.opts.Language)
		if err != nil {
			return moejs.Undefined(), err
		}
		if err := s.vm.Load(mod); err != nil {
			return moejs.Undefined(), s.vm.Wrap(err)
		}
		hook, _ := mod.Hook("execute")
		if ok, err := s.vm.Has(hook); err != nil || !ok {
			return moejs.Undefined(), ErrNoExecute
		}
		fn, _ := s.vm.Export("execute")
		return fn, nil
	}

	fn, err := s.vm.RunScript(program)
	if err != nil {
		return moejs.Undefined(), s.vm.Wrap(err)
	}
	if fn.IsUndefined() {
		return moejs.Undefined(), ErrNoExecute
	}

	return fn, nil
}

// redact hides the credential values printed by the script.
func (s *sandbox) redact(lines []string) []string {
	for i, line := range lines {
		for _, c := range s.opts.Credentials {
			if len(c.Value) >= 4 {
				line = strings.ReplaceAll(line, c.Value, "******")
			}
		}
		lines[i] = line
	}
	return lines
}

// fetch implements context.fetch(url, options, credentialKey), it blocks and returns the response, the script may await it.
func (s *sandbox) fetch(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	url, err := r.ToString(moejs.Arg(args, 0))
	if err != nil {
		return moejs.Undefined(), err
	}
	request := fetchRequest{URL: url.GoString()}
	opts, err := s.vm.ToGo(moejs.Arg(args, 1))
	if err != nil {
		return moejs.Undefined(), err
	}
	if options, ok := opts.(map[string]interface{}); ok {
		request.Method, _ = options["method"].(string)
		if headers, ok := options["headers"].(map[string]interface{}); ok {
			request.Headers = make(map[string]string, len(headers))
			for k, v := range headers {
				request.Headers[k] = fmt.Sprint(v)
			}
		}
		switch body := options["body"].(type) {
		case nil:
		case string:
			request.Body = body
		default:
			raw, err := json.Marshal(body)
			if err != nil {
				return moejs.Undefined(), r.TypeError("fetch: the body can not be encoded as JSON")
			}
			request.Body = string(raw)
			request.JSONBody = true
		}
	}
	if key := moejs.Arg(args, 2); !key.IsUndefined() && !key.IsNull() {
		text, err := r.ToString(key)
		if err != nil {
			return moejs.Undefined(), err
		}
		request.CredentialKey = text.GoString()
	}

	response, err := s.fetcher.do(request)
	if s.ctx.Err() != nil {
		return moejs.Undefined(), s.hostError(s.vm.ContextError())
	}
	if err != nil {
		return moejs.Undefined(), s.hostError(err)
	}

	return s.vm.FromGo(map[string]any{
		"status":     response.Status,
		"statusText": response.StatusText,
		"ok":         response.Status >= 200 && response.Status < 300,
		"url":        response.URL,
		"headers":    response.Headers,
		"text": moejs.NativeFunc(func(*moejs.Realm, moejs.Value, []moejs.Value) (moejs.Value, error) {
			return moejs.String(response.Body), nil
		}),
		"json": moejs.NativeFunc(func(*moejs.Realm, moejs.Value, []moejs.Value) (moejs.Value, error) {
			return s.vm.ParseJSONString(response.Body)
		}),
	})
}
