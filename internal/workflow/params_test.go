package workflow

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func intPtr(v int) *int { return &v }

func codes(errs []ParamError) map[string]string {
	out := map[string]string{}
	for _, e := range errs {
		out[e.Param] = e.Code
	}
	return out
}

func TestValidateParamsDistinguishesEmptyValues(t *testing.T) {
	params := []Param{
		{ID: "a", Key: "name", Source: SourceBody, Type: TypeString, Required: true},
		{ID: "b", Key: "age", Source: SourceBody, Type: TypeInteger, Required: true},
		{ID: "c", Key: "agree", Source: SourceBody, Type: TypeBoolean, Required: true},
		{ID: "d", Key: "note", Source: SourceBody, Type: TypeString},
		{ID: "e", Key: "level", Source: SourceBody, Type: TypeNumber, Default: json.RawMessage("3")},
	}
	body := func(raw string) map[string]json.RawMessage {
		var m map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(raw), &m))
		return m
	}

	// 0 and false are valid values; missing optional parameters are filled with their defaults.
	input, errs := ValidateParams(params, RawInput{Body: body(`{"name":"Ann","age":0,"agree":false}`)})
	require.Empty(t, errs)
	assert.Equal(t, map[string]interface{}{"name": "Ann", "age": float64(0), "agree": false, "level": float64(3)}, input)

	// Missing, null and empty string values each report their own error.
	_, errs = ValidateParams(params, RawInput{Body: body(`{"name":"","age":null}`)})
	assert.Equal(t, map[string]string{"name": CodeEmpty, "age": CodeNull, "agree": CodeRequired}, codes(errs))

	// An optional parameter with an explicit null keeps null instead of using the default.
	input, errs = ValidateParams(params, RawInput{Body: body(`{"name":"a","age":1,"agree":true,"level":null,"note":null}`)})
	require.Empty(t, errs)
	assert.Contains(t, input, "level")
	assert.Nil(t, input["level"])
	assert.Nil(t, input["note"])
}

func TestValidateParamsStrictTypes(t *testing.T) {
	params := []Param{
		{ID: "a", Key: "age", Source: SourceBody, Type: TypeInteger},
		{ID: "b", Key: "price", Source: SourceBody, Type: TypeNumber},
		{ID: "c", Key: "ok", Source: SourceBody, Type: TypeBoolean},
	}
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(`{"age":1.5,"price":"12","ok":"true"}`), &body))
	_, errs := ValidateParams(params, RawInput{Body: body})
	assert.Equal(t, map[string]string{"age": CodeInvalidType, "price": CodeInvalidType, "ok": CodeInvalidType}, codes(errs))
}

func TestValidateParamsQueryAndHeader(t *testing.T) {
	params := []Param{
		{ID: "a", Key: "page", Source: SourceQuery, Type: TypeInteger, Default: json.RawMessage("1")},
		{ID: "b", Key: "flag", Source: SourceQuery, Type: TypeBoolean},
		{ID: "c", Key: "tags", Source: SourceQuery, Type: TypeArray, ItemType: TypeString},
		{ID: "d", Key: "X-Tenant", Source: SourceHeader, Type: TypeString, Required: true},
	}
	q := url.Values{"page": {""}, "flag": {"false"}, "tags": {"a", "b"}}
	h := http.Header{"X-Tenant": {"acme"}}
	input, errs := ValidateParams(params, RawInput{Query: q, Header: h})
	require.Empty(t, errs)
	assert.Equal(t, float64(1), input["page"])
	assert.Equal(t, false, input["flag"])
	assert.Equal(t, []interface{}{"a", "b"}, input["tags"])
	assert.Equal(t, "acme", input["X-Tenant"])

	_, errs = ValidateParams(params, RawInput{Query: url.Values{"page": {"1e"}, "flag": {"1"}}, Header: http.Header{}})
	assert.Equal(t, map[string]string{"page": CodeInvalidType, "flag": CodeInvalidType, "X-Tenant": CodeRequired}, codes(errs))
}

func TestValidateParamsRules(t *testing.T) {
	min := 18.0
	params := []Param{
		{ID: "a", Key: "email", Source: SourceBody, Type: TypeString, Format: FormatEmail},
		{ID: "b", Key: "code", Source: SourceBody, Type: TypeString, MinLength: intPtr(4), MaxLength: intPtr(6)},
		{ID: "c", Key: "age", Source: SourceBody, Type: TypeNumber, Min: &min},
		{ID: "d", Key: "plan", Source: SourceBody, Type: TypeEnum, Options: []string{"free", "pro"}},
		{ID: "e", Key: "sku", Source: SourceBody, Type: TypeString, Pattern: `^[A-Z]{3}$`},
	}
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(`{"email":"x","code":"12","age":3,"plan":"vip","sku":"abc"}`), &body))
	_, errs := ValidateParams(params, RawInput{Body: body})
	assert.Equal(t, map[string]string{"email": CodeFormat, "code": CodeTooShort, "age": CodeTooSmall, "plan": CodeNotOption, "sku": CodePattern}, codes(errs))
	assert.Equal(t, "body.email", errs[0].Path)
}
