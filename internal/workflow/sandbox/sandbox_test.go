package sandbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const echoPlugin = `
export const manifest = { id: "echo", name: { en: "Echo", "zh-CN": "回显" }, version: "1.0.0" };

export async function execute(config, ctx) {
  console.log("hello", { n: 1 });
  ctx.log("step", ctx.step.id);
  return { text: config.text.toUpperCase(), when: new Date(0).toISOString() };
}
`

func compile(t *testing.T, source string) *Program {
	t.Helper()
	p, err := Compile(context.Background(), "plugin.js", source, JavaScript)
	require.NoError(t, err)
	return p
}

func TestCompile(t *testing.T) {
	p := compile(t, echoPlugin)
	assert.JSONEq(t, `{"id":"echo","name":{"en":"Echo","zh-CN":"回显"},"version":"1.0.0"}`, string(p.Manifest()))
	assert.True(t, p.Has(HookExecute))
	assert.False(t, p.Has(HookPreview))

	for name, tc := range map[string]struct {
		source string
		want   error
	}{
		"no manifest":  {source: `export function execute() {}`, want: ErrNoManifest},
		"no execute":   {source: `export const manifest = {}`, want: ErrNoExecute},
		"not function": {source: `export const manifest = {}; export const execute = 1`, want: ErrNoExecute},
		"imports":      {source: `import x from "./x.js"; export const manifest = {}; export function execute() {}`, want: ErrImports},
		"top level":    {source: `for (;;) {}`, want: ErrTimeout},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Compile(context.Background(), "plugin.js", tc.source, JavaScript)
			assert.ErrorIs(t, err, tc.want)
		})
	}

	_, err := Compile(context.Background(), "plugin.js", `export const manifest = {`, JavaScript)
	var syntaxErr *Error
	require.ErrorAs(t, err, &syntaxErr)
	assert.Contains(t, syntaxErr.Message, "SyntaxError")
}

func TestCompileTypeScript(t *testing.T) {
	source := `
interface Config { n: number }
export const manifest = { id: "ts" };
export function execute(config: Config): number { return config.n * 2 }
`
	p, err := Compile(context.Background(), "plugin.ts", source, TypeScript)
	require.NoError(t, err)
	result, err := p.Run(context.Background(), Call{Hook: HookExecute, Args: []interface{}{map[string]interface{}{"n": 21}}})
	require.NoError(t, err)
	assert.Equal(t, float64(42), result.Value)
}

func TestRun(t *testing.T) {
	p := compile(t, echoPlugin)
	result, err := p.Run(context.Background(), Call{
		Hook:    HookExecute,
		Args:    []interface{}{map[string]interface{}{"text": "hi"}},
		Context: map[string]interface{}{"step": map[string]interface{}{"id": "echo1"}},
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]interface{}{"text": "HI", "when": "1970-01-01T00:00:00.000Z"}, result.Value)
	assert.Equal(t, []string{`hello {"n":1}`, "step echo1"}, result.Logs)

	_, err = p.Run(context.Background(), Call{Hook: HookPreview})
	assert.Error(t, err)
}

func TestRunArguments(t *testing.T) {
	p := compile(t, `
export const manifest = {};
export function execute(...args) {
  const ctx = args.pop();
  return { args: JSON.stringify(args), marker: ctx.marker };
}
`)
	for _, tc := range []struct {
		name string
		args []interface{}
		want string
	}{
		{name: "nil", want: `[]`},
		{name: "empty", args: []interface{}{}, want: `[]`},
		{name: "single", args: []interface{}{map[string]interface{}{"value": "config"}}, want: `[{"value":"config"}]`},
		{name: "multiple", args: []interface{}{"first", float64(42), nil, map[string]interface{}{"marker": "argument"}}, want: `["first",42,null,{"marker":"argument"}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := p.Run(context.Background(), Call{
				Hook:    HookExecute,
				Args:    tc.args,
				Context: map[string]interface{}{"marker": "context"},
			})

			require.NoError(t, err)
			assert.Equal(t, map[string]interface{}{"args": tc.want, "marker": "context"}, result.Value)
		})
	}
}

func TestRunErrors(t *testing.T) {
	run := func(body string, timeout time.Duration) (*Result, error) {
		p := compile(t, `export const manifest = {}; export async function execute(config, ctx) {`+body+`}`)
		return p.Run(context.Background(), Call{Hook: HookExecute, Args: []interface{}{map[string]interface{}{}}, Timeout: timeout})
	}

	_, err := run(`throw new TypeError("bad input")`, 0)
	var e *Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "TypeError: bad input", e.Message)
	assert.False(t, e.Retryable)

	_, err = run(`const err = new Error("busy"); err.retryable = true; throw err`, 0)
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "Error: busy", e.Message)
	assert.True(t, e.Retryable)

	_, err = run(`for (;;) {}`, 100*time.Millisecond)
	assert.ErrorIs(t, err, ErrTimeout)

	_, err = run(`const a = []; for (;;) a.push({ n: a.length, items: [1, 2, 3] })`, 10*time.Second)
	assert.ErrorIs(t, err, ErrMemoryLimit)

	_, err = run(`return eval("1 + 1")`, 0)
	require.ErrorAs(t, err, &e)
	assert.Contains(t, e.Message, "EvalError")

	_, err = run(`await new Promise(() => {})`, 0)
	assert.ErrorIs(t, err, ErrPendingPromise)

	_, err = run(`return ctx.fetch("https://example.com")`, 0)
	assert.ErrorIs(t, err, ErrFetchDisabled)

	_, err = run(`return "x".repeat(2 << 20)`, 0)
	assert.ErrorIs(t, err, ErrResultTooLarge)
}

func TestFetch(t *testing.T) {
	p := compile(t, `
export const manifest = {};
export async function execute(config, ctx) {
  const resp = await ctx.fetch("https://api.example.com/items", { method: "POST", headers: { "X-Token": "t" }, body: { name: config.name } });
  const data = await resp.json();
  return { status: resp.status, ok: resp.ok, type: resp.headers["content-type"], id: data.id, text: resp.text() };
}
`)
	var got FetchRequest
	result, err := p.Run(context.Background(), Call{
		Hook: HookExecute,
		Args: []interface{}{map[string]interface{}{"name": "alice"}},
		Fetch: func(_ context.Context, req FetchRequest) (*FetchResponse, error) {
			got = req
			return &FetchResponse{Status: 201, Headers: map[string]string{"content-type": "application/json"}, Body: `{"id":7}`}, nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, FetchRequest{Method: "POST", URL: "https://api.example.com/items", Headers: map[string]string{"X-Token": "t"}, Body: `{"name":"alice"}`, JSONBody: true}, got)
	assert.Equal(t, map[string]interface{}{"status": float64(201), "ok": true, "type": "application/json", "id": float64(7), "text": `{"id":7}`}, result.Value)
	assert.Equal(t, 1, result.Fetches)
}

type hostError struct{ transient bool }

func (e *hostError) Error() string { return "host failed" }

func TestFetchHostError(t *testing.T) {
	p := compile(t, `export const manifest = {}; export async function execute(config, ctx) { await ctx.fetch("https://x.test") }`)
	_, err := p.Run(context.Background(), Call{
		Hook:  HookExecute,
		Args:  []interface{}{nil},
		Fetch: func(context.Context, FetchRequest) (*FetchResponse, error) { return nil, &hostError{transient: true} },
	})
	var he *hostError
	require.True(t, errors.As(err, &he))
	assert.True(t, he.transient)

	// A plugin that catches the host error can rethrow its own.
	p = compile(t, `export const manifest = {}; export async function execute(config, ctx) { try { await ctx.fetch("https://x.test") } catch (e) { throw new Error("wrapped: " + e.message) } }`)
	_, err = p.Run(context.Background(), Call{
		Hook:  HookExecute,
		Args:  []interface{}{nil},
		Fetch: func(context.Context, FetchRequest) (*FetchResponse, error) { return nil, &hostError{} },
	})
	var e *Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "Error: wrapped: host failed", e.Message)
}

func TestTimezone(t *testing.T) {
	p := compile(t, `export const manifest = {}; export function execute() { return new Date(0).getHours() }`)
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	result, err := p.Run(context.Background(), Call{Hook: HookExecute, Location: loc})
	require.NoError(t, err)
	assert.Equal(t, float64(8), result.Value)
}
