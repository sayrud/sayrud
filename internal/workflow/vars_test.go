package workflow

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScopeResolve(t *testing.T) {
	scope := Scope{
		"input": map[string]interface{}{"name": "Ann", "age": float64(30)},
		"steps": map[string]interface{}{
			"find": map[string]interface{}{"records": []interface{}{map[string]interface{}{"uid": "rec1"}}, "count": float64(1)},
		},
	}
	v, ok := scope.Lookup("steps.find.records.0.uid")
	assert.True(t, ok)
	assert.Equal(t, "rec1", v)
	v, _ = scope.Lookup("steps.find.records.length")
	assert.Equal(t, float64(1), v)
	_, ok = scope.Lookup("steps.missing.uid")
	assert.False(t, ok)

	assert.Equal(t, "Hi Ann, 30", scope.Render("Hi {{ input.name }}, {{input.age}}{{input.none}}"))

	got, err := scope.Resolve(&Value{Kind: ValueConst, Const: json.RawMessage(`[1,true]`)})
	require.NoError(t, err)
	assert.Equal(t, []interface{}{float64(1), true}, got)
}

func TestBuildBodyKeepsOrderAndTypes(t *testing.T) {
	scope := Scope{
		"input": map[string]interface{}{"age": float64(30)},
		"steps": map[string]interface{}{"q": map[string]interface{}{"records": []interface{}{
			map[string]interface{}{"uid": "a", "fields": map[string]interface{}{"fldName": "Ann"}},
			map[string]interface{}{"uid": "b", "fields": map[string]interface{}{"fldName": "Bob"}},
		}}},
	}
	body := &Body{Type: BodyObject, Fields: []BodyField{
		{Key: "z", Value: Body{Type: BodyValue, Value: &Value{Kind: ValueVar, Var: "input.age"}}},
		{Key: "a", Value: Body{Type: BodyValue, Value: &Value{Kind: ValueConst, Const: json.RawMessage(`false`)}}},
		{Key: "list", Value: Body{Type: BodyMap, Source: &Value{Kind: ValueVar, Var: "steps.q.records"}, Item: &Body{Type: BodyObject, Fields: []BodyField{
			{Key: "id", Value: Body{Type: BodyValue, Value: &Value{Kind: ValueVar, Var: "item.uid"}}},
			{Key: "name", Value: Body{Type: BodyValue, Value: &Value{Kind: ValueVar, Var: "item.fields.fldName"}}},
		}}}},
	}}
	out, err := scope.BuildBody(body)
	require.NoError(t, err)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	assert.Equal(t, `{"z":30,"a":false,"list":[{"id":"a","name":"Ann"},{"id":"b","name":"Bob"}]}`, string(raw))
}

func TestCompare(t *testing.T) {
	assert.True(t, compare(float64(3), "eq", "3"))
	assert.True(t, compare("abc", "contains", "b"))
	assert.True(t, compare([]interface{}{"a", "b"}, "contains", "b"))
	assert.True(t, compare(nil, "empty", nil))
	assert.False(t, compare(false, "eq", nil))
	assert.True(t, compare(float64(10), "gt", float64(9)))
	assert.True(t, compare("2026-01-02", "gt", "2026-01-01"))
	assert.True(t, compare("10", "gt", float64(9)), "numeric strings compare numerically on either side")
	assert.True(t, compare("10", "gt", "9"))
	assert.False(t, compare("12abc", "eq", float64(12)), "strings with trailing text are not numbers")
	assert.False(t, compare(nil, "gt", float64(0)))
}

func TestNormalizePath(t *testing.T) {
	for in, want := range map[string]string{"/aaaa": "/aaaa", "leads/": "/leads", "/v1/leads": "/v1/leads"} {
		got, ok := NormalizePath(in)
		assert.True(t, ok, in)
		assert.Equal(t, want, got)
	}
	for _, in := range []string{"/", "", "/a//b", "/../x", "/a b", "/a?x=1"} {
		_, ok := NormalizePath(in)
		assert.False(t, ok, in)
	}
}

func TestNormalizeClearsResponseHeaders(t *testing.T) {
	s := &Spec{
		Trigger: Trigger{Type: TriggerAPI},
		Steps: []Step{
			{ID: "c", Type: StepCondition, Then: []Step{{ID: "r", Type: StepResponse, Headers: []Header{{Name: "X-Total"}}}}},
			{ID: "h", Type: StepHTTP, Headers: []Header{{Name: "X-Token"}}},
		},
	}
	s.Normalize()
	assert.Nil(t, s.Steps[0].Then[0].Headers, "response steps cannot set headers")
	assert.Len(t, s.Steps[1].Headers, 1, "http steps keep the request headers")
}
