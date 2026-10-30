package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormulaEval(t *testing.T) {
	scope := Scope{
		"input": map[string]interface{}{"name": "ann", "age": float64(30), "tags": []interface{}{"a", "b"}, "empty": ""},
		"trigger": map[string]interface{}{"record": map[string]interface{}{
			"fields": map[string]interface{}{"due": "2026-01-31T00:00:00Z"},
		}},
	}
	loc := time.UTC
	eval := func(src string) interface{} {
		t.Helper()
		f, err := ParseFormula(src)
		require.NoError(t, err, src)
		v, err := f.Eval(func(path string) interface{} {
			v, _ := scope.Lookup(path)
			return v
		}, loc)
		require.NoError(t, err, src)
		return v
	}

	tests := []struct {
		src  string
		want interface{}
	}{
		{`1 + 2 * 3`, 7.0},
		{`2 ^ 3 ^ 2`, 512.0},
		{`-{input.age} + 1`, -29.0},
		{`"Hi " & UPPER({input.name})`, "Hi ANN"},
		{`CONCATENATE({input.name}, "-", {input.age})`, "ann-30"},
		{`{input.tags}`, "a, b"},
		{`IF({input.age} >= 18, "adult", "minor")`, "adult"},
		{`IF(FALSE, 1)`, nil},
		{`AND(TRUE, {input.age} > 1)`, true},
		{`ISBLANK({input.empty})`, true},
		{`ISBLANK({input.missing})`, true},
		{`LEN("你好")`, 2.0},
		{`LEFT("abcdef", 2) & RIGHT("abcdef", 2) & MID("abcdef", 2, 3)`, "abefbcd"},
		{`SUBSTITUTE("a-b-c", "-", "+")`, "a+b+c"},
		{`ROUND(2.5)`, 3.0},
		{`ROUND(1.234, 2)`, 1.23},
		{`ROUND(-2.5)`, -2.0},
		{`MOD(7, 3) + INT(2.7) + ABS(-1)`, 4.0},
		{`SUM(1, "2", "") + AVERAGE(2, 4) + MAX(1, 5) + MIN(3, 2)`, 13.0},
		{`"10" > 9`, true},
		{`"b" > "a"`, true},
		{`YEAR({trigger.record.fields.due}) & "/" & MONTH({trigger.record.fields.due})`, "2026/1"},
		{`DATEADD({trigger.record.fields.due}, 1, "M")`, "2026-02-28T00:00:00Z"},
		{`DATEADD("2026-01-01", 2)`, "2026-01-03T00:00:00Z"},
		{`DATEDIF("2026-01-31", "2026-03-01", "M")`, 1.0},
		{`DATEDIF("2026-01-01", "2026-03-01")`, 59.0},
		{`WEEKDAY("2026-10-12")`, 1.0},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, eval(tt.src), tt.src)
	}
}

func TestFormulaErrors(t *testing.T) {
	for _, src := range []string{``, `1 +`, `(1`, `FOO(1)`, `"abc`, `{input.a`, `abc`, `1 # 2`, `TRUE()`} {
		_, err := ParseFormula(src)
		var fe *FormulaError
		assert.ErrorAs(t, err, &fe, src)
	}

	f, err := ParseFormula(`CONCATENATE({input.a}, {steps.q.total}, {input.a})`)
	require.NoError(t, err)
	assert.Equal(t, []string{"input.a", "steps.q.total"}, f.Refs)

	for _, src := range []string{`1 / 0`, `MOD(1, 0)`, `"abc" * 2`, `SQRT(-1)`, `ROUND()`} {
		f, err := ParseFormula(src)
		require.NoError(t, err, src)
		_, err = f.Eval(func(string) interface{} { return nil }, time.UTC)
		var fe *FormulaError
		assert.ErrorAs(t, err, &fe, src)
	}
}

func TestFormulaRandstr(t *testing.T) {
	for _, length := range []int{0, 1, 16, 4096} {
		src := fmt.Sprintf("RANDSTR(%d)", length)
		f, err := ParseFormula(src)
		require.NoError(t, err, src)
		v, err := f.Eval(func(string) interface{} { return nil }, time.UTC)
		require.NoError(t, err, src)
		require.IsType(t, "", v)
		assert.Len(t, v.(string), length)
		assert.Regexp(t, `^[A-Za-z0-9]*$`, v)
	}

	scope := Scope{"input": map[string]interface{}{"length": 20.0}}
	v, err := scope.Resolve(&Value{Kind: ValueFormula, Formula: `"id-" & RANDSTR({input.length})`})
	require.NoError(t, err)
	assert.Regexp(t, `^id-[A-Za-z0-9]{20}$`, v)
	for _, src := range []string{`LEN(randstr(3 * 4))`, `LEN(RANDSTR("12"))`} {
		v, err := scope.evalFormula(src)
		require.NoError(t, err, src)
		assert.Equal(t, 12.0, v)
	}

	first, err := scope.evalFormula(`RANDSTR(32)`)
	require.NoError(t, err)
	second, err := scope.evalFormula(`RANDSTR(32)`)
	require.NoError(t, err)
	assert.NotEqual(t, first, second, "parsed formula caching must not cache random results")
	v, err = scope.evalFormula(`IF(FALSE, RANDSTR(-1), "skip")`)
	require.NoError(t, err)
	assert.Equal(t, "skip", v)
}

func TestFormulaRandstrErrors(t *testing.T) {
	for _, src := range []string{
		`RANDSTR()`, `RANDSTR(8, 2)`, `RANDSTR(-1)`, `RANDSTR(1.5)`, `RANDSTR(4097)`,
		`RANDSTR(1e100)`, `RANDSTR(POWER(10, 1000))`, `RANDSTR(SQRT(-1))`, `RANDSTR("abc")`,
	} {
		t.Run(src, func(t *testing.T) {
			f, err := ParseFormula(src)
			require.NoError(t, err)
			_, err = f.Eval(func(string) interface{} { return nil }, time.UTC)
			var fe *FormulaError
			require.ErrorAs(t, err, &fe)
		})
	}
}

func TestRunQueryMatchAnyAndFormula(t *testing.T) {
	f := newLeadFixture(t)
	for _, body := range []string{`{"name":"Ann","email":"ann@example.com"}`, `{"name":"Bob","email":"bob@example.com"}`, `{"name":"Cid","email":"cid@example.com"}`} {
		w, _ := f.call(t, body, nil)
		require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	}

	spec := &Spec{
		Trigger: Trigger{Type: TriggerAPI, API: &APITrigger{
			Method: "POST", Path: "/find", Auth: Auth{Type: AuthNone},
			Params: []Param{
				{ID: "p1", Key: "a", Source: SourceBody, Type: TypeString, Required: true},
				{ID: "p2", Key: "b", Source: SourceBody, Type: TypeString, Required: true},
			},
		}},
		Steps: []Step{
			{ID: "q", Type: StepQuery, Config: &QueryConfig{TableUID: f.table.UID, Match: "any", Filters: []Filter{
				{FieldUID: f.name.UID, Op: "eq", Value: vp("input.a")},
				{FieldUID: f.name.UID, Op: "eq", Value: &Value{Kind: ValueFormula, Formula: `UPPER(LEFT({input.b}, 1)) & MID({input.b}, 2, 10)`}},
			}}},
			{ID: "respond", Type: StepResponse, Config: &ResponseConfig{Status: 200, Body: &Body{Type: BodyObject, Fields: []BodyField{
				{Key: "total", Value: Body{Type: BodyValue, Value: vp("steps.q.total")}},
				{Key: "summary", Value: Body{Type: BodyValue, Value: &Value{Kind: ValueFormula, Formula: `"found " & {steps.q.total} & IF({steps.q.found}, "!", "?")`}}},
				{Key: "double", Value: Body{Type: BodyValue, Value: &Value{Kind: ValueFormula, Formula: `{steps.q.total} * 2`}}},
			}}}},
		},
		Async: []AsyncFlow{},
	}
	issues, err := Check(t.Context(), f.db, f.project, spec)
	require.NoError(t, err)
	require.Empty(t, issues)
	f.publish(t, "Find", spec)

	req := httptest.NewRequest(http.MethodPost, "/api/"+f.project.UID+"/find", bytes.NewBufferString(`{"a":"Ann","b":"bob"}`))
	w := httptest.NewRecorder()
	f.engine.Serve(w, req, f.project.UID, "/find", "127.0.0.1")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, map[string]interface{}{"total": 2.0, "summary": "found 2!", "double": 4.0}, out)

	// Formula syntax errors and references to nonexistent variables are reported by Check.
	body := spec.Steps[1].Config.(*ResponseConfig).Body
	body.Fields[1].Value.Value.Formula = `"found " &`
	body.Fields[2].Value.Value.Formula = `{steps.nope.total} * 2`
	issues, err = Check(t.Context(), f.db, f.project, spec)
	require.NoError(t, err)
	codes := make([]string, 0, len(issues))
	for _, issue := range issues {
		codes = append(codes, issue.Code)
	}
	assert.ElementsMatch(t, []string{"value.invalid_formula", "value.unknown_var"}, codes)
}

func TestFormulaTimezone(t *testing.T) {
	year := func(tz string) interface{} {
		t.Helper()
		v, err := Scope{"run": map[string]interface{}{"timezone": tz}}.evalFormula(`YEAR("2026-12-31T20:00:00Z")`)
		require.NoError(t, err)
		return v
	}
	assert.Equal(t, 2027.0, year("Asia/Shanghai"), "dates are handled in run.timezone")
	assert.Equal(t, 2026.0, year(""), "UTC is used without a time zone")
	assert.Equal(t, 2026.0, year("Not/AZone"), "UTC is used for an invalid time zone")

	spec := &Spec{Timezone: "Asia/Tokyo", Trigger: Trigger{Type: TriggerSchedule, Schedule: &ScheduleTrigger{Timezone: "Europe/Paris"}}}
	assert.Equal(t, "Europe/Paris", spec.FormulaTimezone(), "the schedule trigger's time zone takes precedence")
	spec.Trigger = Trigger{Type: TriggerAPI}
	assert.Equal(t, "Asia/Tokyo", spec.FormulaTimezone())
}

func TestFormulaCacheIsBounded(t *testing.T) {
	for i := 0; i < maxCachedFormulas+10; i++ {
		_, err := ParseFormula(fmt.Sprintf("%d + 1", i))
		require.NoError(t, err)
	}
	formulaCache.Lock()
	defer formulaCache.Unlock()
	assert.LessOrEqual(t, len(formulaCache.entries), maxCachedFormulas)
}
