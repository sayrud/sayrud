// Package script runs the custom field shortcuts written in JavaScript.
//
// Every run uses a new goja runtime which has no module loader, file system or process access, and is interrupted on timeout.
// context.fetch is limited to allowed domains and public or explicitly allowed addresses; context.ai uses the injected server-side model client.
// goja is an interpreter in the server process rather than an isolation boundary, so the scripts must be published by trusted admins.
package script

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dop251/goja"
	"github.com/pkg/errors"

	"github.com/wuhan005/sayrud/internal/db"
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

const (
	maxCallStackSize = 512
	maxResultSize    = 1 << 20
	maxLogs          = 200
	maxLogLength     = 2000
	scriptName       = "shortcut.js"
)

var (
	ErrNoExecute      = errors.New("the script does not define the execute function")
	ErrTimeout        = errors.New("the script timed out")
	ErrPendingPromise = errors.New("the promise returned by execute never settles")
	ErrResultTooLarge = errors.New("the result is too large")
	ErrAIDisabled     = errors.New("AI is not enabled for this shortcut")
)

// Error is an error thrown by the script or a syntax error, Message is shown to the users.
type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

// Compile checks the syntax of the code.
func Compile(code string) error {
	if _, err := goja.Compile(scriptName, code, false); err != nil {
		return &Error{Message: err.Error()}
	}
	return nil
}

type sandbox struct {
	ctx        context.Context
	vm         *goja.Runtime
	opts       Options
	logs       []string
	fetcher    *fetcher
	aiCalls    int
	hostErrors map[*goja.Object]error
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

	vm := goja.New()
	vm.SetMaxCallStackSize(maxCallStackSize)
	s := &sandbox{ctx: runCtx, vm: vm, opts: opts}
	s.fetcher = newFetcher(runCtx, opts.Domains, opts.Credentials, opts.Networks)
	defer s.fetcher.client.CloseIdleConnections()
	stop := context.AfterFunc(runCtx, func() { vm.Interrupt(runCtx.Err()) })
	defer stop()

	value, err := s.run()
	result := &Result{Logs: s.redact(s.logs)}
	if err != nil {
		return result, err
	}
	result.Value = value
	return result, nil
}

func (s *sandbox) run() (value interface{}, err error) {
	// The goja helpers called from Go panic with JavaScript exceptions.
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = s.wrap(e)
				return
			}
			err = &Error{Message: fmt.Sprint(r)}
		}
	}()
	return s.execute()
}

func (s *sandbox) execute() (interface{}, error) {
	if err := s.setGlobals(); err != nil {
		return nil, err
	}
	if _, err := s.vm.RunScript(scriptName, s.opts.Code); err != nil {
		return nil, s.wrap(err)
	}
	execute, ok := goja.AssertFunction(s.vm.Get("execute"))
	if !ok {
		return nil, ErrNoExecute
	}

	params, err := s.fromGo(s.opts.Params)
	if err != nil {
		return nil, errors.Wrap(err, "convert params")
	}
	contextObject, err := s.contextObject()
	if err != nil {
		return nil, errors.Wrap(err, "build context")
	}

	// Promise jobs run before the outermost call returns; fetch and AI calls block, so an async execute has settled by then.
	value, err := execute(goja.Undefined(), params, contextObject)
	if err != nil {
		return nil, s.wrap(err)
	}
	if promise, ok := value.Export().(*goja.Promise); ok {
		switch promise.State() {
		case goja.PromiseStateFulfilled:
			value = promise.Result()
		case goja.PromiseStateRejected:
			return nil, s.valueError(promise.Result())
		default:
			if s.ctx.Err() != nil {
				return nil, s.ctxError()
			}
			return nil, ErrPendingPromise
		}
	}
	return s.toGo(value)
}

func (s *sandbox) setGlobals() error {
	console := s.vm.NewObject()
	for _, name := range []string{"log", "info", "warn", "error", "debug"} {
		if err := console.Set(name, s.log); err != nil {
			return errors.Wrap(err, "set console")
		}
	}
	return errors.Wrap(s.vm.Set("console", console), "set console")
}

func (s *sandbox) contextObject() (goja.Value, error) {
	value, err := s.fromGo(s.opts.Context)
	if err != nil {
		return nil, err
	}
	object := value.ToObject(s.vm)
	if err := object.Set("fetch", s.fetch); err != nil {
		return nil, err
	}
	if err := object.Set("log", s.log); err != nil {
		return nil, err
	}

	ai := s.vm.NewObject()
	if err := ai.Set("complete", s.aiComplete); err != nil {
		return nil, err
	}
	if err := object.Set("ai", ai); err != nil {
		return nil, err
	}

	return object, nil
}

// fromGo converts the JSON-compatible Go values to a plain JavaScript object.
func (s *sandbox) fromGo(v map[string]interface{}) (goja.Value, error) {
	if v == nil {
		v = map[string]interface{}{}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return s.parseJSON(string(raw))
}

func (s *sandbox) parseJSON(text string) (goja.Value, error) {
	parse, ok := goja.AssertFunction(s.vm.Get("JSON").ToObject(s.vm).Get("parse"))
	if !ok {
		return nil, errors.New("JSON.parse is unavailable")
	}
	return parse(goja.Undefined(), s.vm.ToValue(text))
}

// toGo converts the JavaScript value through JSON, so the result only contains JSON types and numbers are float64.
func (s *sandbox) toGo(v goja.Value) (interface{}, error) {
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return nil, nil
	}
	stringify, ok := goja.AssertFunction(s.vm.Get("JSON").ToObject(s.vm).Get("stringify"))
	if !ok {
		return nil, errors.New("JSON.stringify is unavailable")
	}
	text, err := stringify(goja.Undefined(), v)
	if err != nil {
		return nil, s.wrap(err)
	}
	if goja.IsUndefined(text) {
		return nil, nil
	}
	if len(text.String()) > maxResultSize {
		return nil, ErrResultTooLarge
	}
	var result interface{}
	if err := json.Unmarshal([]byte(text.String()), &result); err != nil {
		return nil, errors.Wrap(err, "decode result")
	}
	return result, nil
}

func (s *sandbox) log(call goja.FunctionCall) goja.Value {
	if len(s.logs) >= maxLogs {
		return goja.Undefined()
	}
	parts := make([]string, 0, len(call.Arguments))
	for _, arg := range call.Arguments {
		parts = append(parts, s.describe(arg))
	}
	line := strings.Join(parts, " ")
	if utf8.RuneCountInString(line) > maxLogLength {
		line = string([]rune(line)[:maxLogLength]) + "…"
	}
	s.logs = append(s.logs, line)
	return goja.Undefined()
}

// describe returns the text of the value for the logs and the errors.
func (s *sandbox) describe(v goja.Value) string {
	if v == nil || goja.IsUndefined(v) {
		return "undefined"
	}
	if object, ok := v.(*goja.Object); ok {
		if message := object.Get("message"); message != nil && !goja.IsUndefined(message) {
			if name := object.Get("name"); name != nil && !goja.IsUndefined(name) {
				return name.String() + ": " + message.String()
			}
			return message.String()
		}
		if exported, err := s.toGo(v); err == nil {
			if raw, err := json.Marshal(exported); err == nil {
				return string(raw)
			}
		}
	}
	return v.String()
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

func (s *sandbox) ctxError() error {
	if errors.Is(s.ctx.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	return s.ctx.Err()
}

func (s *sandbox) wrap(err error) error {
	var interrupted *goja.InterruptedError
	if errors.As(err, &interrupted) {
		return s.ctxError()
	}
	var exception *goja.Exception
	if errors.As(err, &exception) {
		return s.valueError(exception.Value())
	}
	var stackOverflow *goja.StackOverflowError
	if errors.As(err, &stackOverflow) {
		return &Error{Message: "maximum call stack size exceeded"}
	}
	return &Error{Message: err.Error()}
}

func (s *sandbox) valueError(value goja.Value) error {
	if object, ok := value.(*goja.Object); ok {
		if err := s.hostErrors[object]; err != nil {
			return err
		}
	}

	return &Error{Message: s.describe(value)}
}

// fetch implements context.fetch(url, options, credentialKey), it blocks and returns the response, the script may await it.
func (s *sandbox) fetch(call goja.FunctionCall) goja.Value {
	request := fetchRequest{URL: call.Argument(0).String()}
	if options, ok := call.Argument(1).Export().(map[string]interface{}); ok {
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
				panic(s.vm.NewTypeError("fetch: the body can not be encoded as JSON"))
			}
			request.Body = string(raw)
			request.JSONBody = true
		}
	}
	if key := call.Argument(2); !goja.IsUndefined(key) && !goja.IsNull(key) {
		request.CredentialKey = key.String()
	}

	response, err := s.fetcher.do(request)
	if err != nil {
		panic(s.vm.NewGoError(err))
	}

	object := s.vm.NewObject()
	headers := s.vm.NewObject()
	for k, v := range response.Headers {
		_ = headers.Set(k, v)
	}
	for k, v := range map[string]interface{}{
		"status":     response.Status,
		"statusText": response.StatusText,
		"ok":         response.Status >= 200 && response.Status < 300,
		"url":        response.URL,
		"headers":    headers,
		"text":       func() string { return response.Body },
		"json": func() goja.Value {
			value, err := s.parseJSON(response.Body)
			if err != nil {
				panic(err)
			}
			return value
		},
	} {
		_ = object.Set(k, v)
	}
	return object
}
