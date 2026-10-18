// Package jsruntime provides the moejs runtime shared by field shortcuts and workflow plugins.
// Callers install their own host functions and enforce the permissions of those functions.
package jsruntime

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/Calcium-Ion/moejs"
	"github.com/cockroachdb/errors"
)

const (
	maxResultSize = 1 << 20
	maxLogs       = 200
	maxLogLength  = 2000
	memoryLimit   = 64 << 20
)

var (
	ErrTimeout        = errors.New("the script timed out")
	ErrMemoryLimit    = errors.New("the script exceeded the memory limit")
	ErrPendingPromise = errors.New("the promise returned by the script never settles")
	ErrResultTooLarge = errors.New("the result is too large")
)

// Error is a syntax error or a value thrown or rejected by JavaScript.
type Error struct {
	Message string
	// Retryable is true when the thrown object has a retryable property set to true.
	Retryable bool
}

func (e *Error) Error() string {
	return e.Message
}

// Runtime owns one call's globals, logs and interrupt watcher; it must not be shared between calls.
type Runtime struct {
	*moejs.Runtime
	ctx     context.Context
	release func()
	Logs    []string
}

// New creates a runtime with a memory budget and console; Close stops its interrupt watcher.
func New(ctx context.Context, opts moejs.Options) (*Runtime, error) {
	opts.MemoryLimit = memoryLimit
	rt := &Runtime{Runtime: moejs.NewRuntime(opts), ctx: ctx, Logs: []string{}}

	interrupted := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		rt.Interrupt(context.Cause(ctx))
		close(interrupted)
	})
	rt.release = func() {
		if !stop() {
			<-interrupted
		}
	}

	log := moejs.NativeFunc(rt.Log)
	console := map[string]any{"log": log, "info": log, "warn": log, "error": log, "debug": log}
	if err := rt.SetGlobal("console", console); err != nil {
		rt.Close()
		return nil, errors.Wrap(err, "set console")
	}

	return rt, nil
}

// Close waits for a pending interrupt to finish before releasing the runtime.
func (rt *Runtime) Close() {
	rt.release()
}

// Parse converts JSON-compatible input into plain JavaScript objects without exposing Go values.
func (rt *Runtime) Parse(v interface{}) (moejs.Value, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return moejs.Undefined(), err
	}

	return rt.ParseJSON(raw)
}

// Context copies the data and adds only the host functions supplied by the caller, plus log.
func (rt *Runtime) Context(data map[string]interface{}, functions map[string]any) (moejs.Value, error) {
	values := map[string]any{}
	if data != nil {
		raw, err := json.Marshal(data)
		if err != nil {
			return moejs.Undefined(), err
		}
		if err := json.Unmarshal(raw, &values); err != nil {
			return moejs.Undefined(), err
		}
	}

	for key, value := range functions {
		values[key] = value
	}
	values["log"] = moejs.NativeFunc(rt.Log)

	return rt.FromGo(values)
}

// Resolve unwraps an already settled promise and returns JSON-compatible output with float64 numbers.
func (rt *Runtime) Resolve(value moejs.Value) (interface{}, error) {
	if state, result, ok := moejs.PromiseResult(value); ok {
		switch state {
		case moejs.PromiseFulfilled:
			value = result
		case moejs.PromiseRejected:
			return nil, rt.Thrown(result)
		default:
			if rt.ctx.Err() != nil {
				return nil, rt.ContextError()
			}
			return nil, ErrPendingPromise
		}
	}

	if value.IsUndefined() || value.IsNull() {
		return nil, nil
	}
	raw, err := rt.AppendJSON(nil, value)
	if err != nil {
		return nil, rt.Wrap(err)
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

// Log implements console methods and context.log with bounded output.
func (rt *Runtime) Log(_ *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
	if len(rt.Logs) >= maxLogs {
		return moejs.Undefined(), nil
	}

	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, rt.Describe(arg))
	}
	line := strings.Join(parts, " ")
	if utf8.RuneCountInString(line) > maxLogLength {
		line = string([]rune(line)[:maxLogLength]) + "…"
	}
	rt.Logs = append(rt.Logs, line)

	return moejs.Undefined(), nil
}

// Describe returns text for logs and errors without running getters on an error's name or message.
func (rt *Runtime) Describe(v moejs.Value) string {
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
		if raw, err := rt.AppendJSON(nil, v); err == nil {
			return string(raw)
		}
	}

	return v.String()
}

// ContextError preserves cancellation and gives deadlines a script-specific error.
func (rt *Runtime) ContextError() error {
	if errors.Is(rt.ctx.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}

	return rt.ctx.Err()
}

// Wrap converts engine failures while preserving errors returned by host functions.
func (rt *Runtime) Wrap(err error) error {
	var interrupted *moejs.InterruptedError
	if errors.As(err, &interrupted) {
		if errors.Is(err, moejs.ErrMemoryLimit) {
			return ErrMemoryLimit
		}
		if rt.ctx.Err() != nil {
			return rt.ContextError()
		}
		return ErrTimeout
	}

	var exc *moejs.Exception
	if errors.As(err, &exc) {
		return rt.Thrown(exc.Value)
	}

	return &Error{Message: err.Error()}
}

// Thrown preserves the identity of host errors and reads a script's retry hint.
func (rt *Runtime) Thrown(v moejs.Value) error {
	exc := &moejs.Exception{Value: v}
	if hostErr := exc.Unwrap(); hostErr != nil {
		return hostErr
	}

	err := &Error{Message: rt.Describe(v)}
	if v.IsObject() {
		if retry, getErr := rt.Get(v, "retryable"); getErr == nil && retry.IsBool() && retry.AsBool() {
			err.Retryable = true
		}
	}

	return err
}
