package workflow

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderErrorResponse(t *testing.T) {
	errs := []ParamError{
		{Path: "body.phone", Param: "phone", Code: CodeFormat, Message: "手机号\"格式\"不正确\n"},
		{Path: "body.age", Param: "age", Code: CodeTooSmall, Message: "年龄太小"},
	}
	scope := paramErrorScope(errs, "run_1")
	render := func(body string) string {
		t.Helper()
		resp := renderErrorResponse(&ErrorResponse{Status: 200, Body: body}, scope)
		require.NotNil(t, resp)
		assert.Equal(t, 200, resp.Status)
		raw, err := json.Marshal(resp.Body)
		require.NoError(t, err)
		return string(raw)
	}

	// Whole placeholders keep their types, mixed strings are interpolated as text, and missing variables become null.
	assert.JSONEq(t, `{
		"msg": "手机号\"格式\"不正确\n",
		"fields": [
			{"path": "body.phone", "param": "phone", "code": "format", "message": "手机号\"格式\"不正确\n"},
			{"path": "body.age", "param": "age", "code": "too_small", "message": "年龄太小"}
		],
		"count": 2,
		"hint": "phone: format",
		"trace": "run_1",
		"missing": null,
		"nested": [{"first": {"path": "body.phone", "param": "phone", "code": "format", "message": "手机号\"格式\"不正确\n"}}]
	}`, render(`{"msg":"{{error.message}}","fields":"{{ errors }}","count":"{{errors.length}}","hint":"{{error.param}}: {{error.code}}","trace":"{{run.id}}","missing":"{{error.unknown}}","nested":[{"first":"{{errors.0}}"}]}`))

	// Property order and number literals are kept, and keys are not rendered.
	assert.Equal(t, `{"z":12345678901234567890,"{{error.code}}":"format","a":true}`, render(`{"z":12345678901234567890,"{{error.code}}":"{{error.code}}","a":true}`))

	// A body without placeholders is returned as is.
	resp := renderErrorResponse(&ErrorResponse{Status: 422, Body: `{"ok":false}`}, scope)
	require.NotNil(t, resp)
	assert.Equal(t, json.RawMessage(`{"ok":false}`), resp.Body)

	for _, r := range []*ErrorResponse{nil, {Status: 204, Body: `{}`}, {Status: 400, Body: `{"code":`}, {Status: 400}} {
		assert.Nil(t, renderErrorResponse(r, scope))
	}
}

func TestParamErrorScopeWithoutRun(t *testing.T) {
	scope := paramErrorScope([]ParamError{{Path: SourceBody, Code: CodeInvalidBody, Message: "m"}}, "")
	resp := renderErrorResponse(&ErrorResponse{Status: 400, Body: `{"id":"{{run.id}}","code":"{{error.code}}","param":"{{error.param}}"}`}, scope)
	require.NotNil(t, resp)
	raw, err := json.Marshal(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":null,"code":"invalid_body","param":""}`, string(raw))
}

func TestTemplateRoots(t *testing.T) {
	assert.Equal(t, []string{"error", "input", "run"}, templateRoots(`{"{{key}}":"{{error.message}} {{input.x}}","a":["{{run.id}}","{{error.code}}"]}`))
	assert.Empty(t, templateRoots(`{"a":1}`))
	assert.Empty(t, templateRoots(`{`))
}

func TestCheckParamErrorResponse(t *testing.T) {
	check := func(api *APITrigger) []Issue {
		api.Method, api.Path, api.Auth = "POST", "/test", Auth{Type: AuthNone}
		c := &checker{spec: &Spec{Trigger: Trigger{Type: TriggerAPI, API: api}}, inputKeys: map[string]bool{}}
		c.checkRoute()
		c.checkParams()
		return c.issues
	}

	assert.Empty(t, check(&APITrigger{
		ParamErrorResponse: &ErrorResponse{Status: 200, Body: `{"msg":"{{error.message}}","list":"{{errors}}","id":"{{run.id}}"}`},
		Params:             []Param{{ID: "p1", Key: "phone", Source: SourceBody, Type: TypeString, Message: "m", Messages: map[string]string{CodeRequired: "r", CodeFormat: "f"}}},
	}))
	assert.Equal(t, []Issue{
		{Code: "security.invalid_response_status", Target: "params"},
		{Code: "param_error.unknown_var", Target: "params", Args: map[string]string{"var": "input"}},
		{Code: "param.invalid_message_code", Target: "param:p1", Args: map[string]string{"key": "phone", "code": "bogus"}},
	}, check(&APITrigger{
		ParamErrorResponse: &ErrorResponse{Status: 204, Body: `{"msg":"{{input.phone}}"}`},
		Params:             []Param{{ID: "p1", Key: "phone", Source: SourceBody, Type: TypeString, Messages: map[string]string{"bogus": "x"}}},
	}))
}
