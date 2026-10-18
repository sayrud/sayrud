package jsruntime

import (
	"context"
	"strings"
	"testing"

	"github.com/Calcium-Ion/moejs"
	"github.com/stretchr/testify/require"
)

func TestResolveJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		code string
		want interface{}
	}{
		{name: "number", code: `42`, want: float64(42)},
		{name: "undefined", code: `undefined`},
		{name: "function", code: `(() => 1)`},
		{name: "symbol", code: `Symbol("x")`},
		{name: "date", code: `new Date(0)`, want: "1970-01-01T00:00:00.000Z"},
		{name: "object", code: `({a: undefined, b: NaN, c: Infinity, d: [undefined, 1]})`, want: map[string]interface{}{"b": nil, "c": nil, "d": []interface{}{nil, float64(1)}}},
		{name: "toJSON", code: `({toJSON() { return {n: 7} }})`, want: map[string]interface{}{"n": float64(7)}},
		{name: "promise", code: `Promise.resolve({n: 7})`, want: map[string]interface{}{"n": float64(7)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt, err := New(context.Background(), moejs.Options{})
			require.NoError(t, err)
			defer rt.Close()

			program, err := moejs.CompileScript("result.js", tc.code)
			require.NoError(t, err)
			value, err := rt.RunScript(program)
			require.NoError(t, err)
			got, err := rt.Resolve(value)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestContextAndLogs(t *testing.T) {
	rt, err := New(context.Background(), moejs.Options{})
	require.NoError(t, err)
	defer rt.Close()

	data := map[string]interface{}{"record": map[string]interface{}{"name": "original"}}
	value, err := rt.Context(data, nil)
	require.NoError(t, err)
	require.NoError(t, rt.SetGlobal("ctx", value))

	program, err := moejs.CompileScript("context.js", `
ctx.record.name = "changed";
ctx.log("界".repeat(2001));
for (let i = 0; i < 201; i++) console.log(i);
[typeof ctx.fetch, typeof ctx.ai, ctx.record.name];
`)
	require.NoError(t, err)
	result, err := rt.RunScript(program)
	require.NoError(t, err)
	got, err := rt.Resolve(result)
	require.NoError(t, err)
	require.Equal(t, []interface{}{"undefined", "undefined", "changed"}, got)
	require.Equal(t, "original", data["record"].(map[string]interface{})["name"])
	require.Len(t, rt.Logs, maxLogs)
	require.Equal(t, strings.Repeat("界", maxLogLength)+"…", rt.Logs[0])
}

func TestCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rt, err := New(ctx, moejs.Options{})
	require.NoError(t, err)
	defer rt.Close()

	program, err := moejs.CompileScript("cancel.js", `for (;;) {}`)
	require.NoError(t, err)
	_, err = rt.RunScript(program)
	require.ErrorIs(t, rt.Wrap(err), context.Canceled)
}
