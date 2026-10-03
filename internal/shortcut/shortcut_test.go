package shortcut

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/ai/openai"
	"github.com/wuhan005/sayrud/internal/conf"
	"github.com/wuhan005/sayrud/internal/db"
)

func field(uid string, t db.SLFieldType, metadata map[string]interface{}) *db.SLField {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	return &db.SLField{UID: uid, Label: uid, Type: t, Metadata: datatypes.NewJSONType[db.SLFieldMetadata](metadata)}
}

func selectField(uid string, t db.SLFieldType, names ...string) *db.SLField {
	options := make([]interface{}, 0, len(names))
	for i, name := range names {
		options = append(options, map[string]interface{}{"uid": "opt" + string(rune('A'+i)), "name": name, "color": i})
	}
	return field(uid, t, map[string]interface{}{"options": options})
}

func TestInputValue(t *testing.T) {
	s := selectField("fldSSSSSSS", db.SingleSelectFieldType, "Bug", "Feature")
	m := selectField("fldMMMMMMM", db.MultiSelectFieldType, "UI", "API")
	require.Equal(t, "hello", InputValue(field("fldTTTTTTT", db.TextFieldType, nil), "hello"))
	require.Nil(t, InputValue(field("fldTTTTTTT", db.TextFieldType, nil), ""))
	require.Equal(t, 1.5, InputValue(field("fldNNNNNNN", db.NumberFieldType, nil), 1.5))
	require.Equal(t, false, InputValue(field("fldCCCCCCC", db.CheckboxFieldType, nil), nil))
	require.Equal(t, "Feature", InputValue(s, "optB"))
	require.Nil(t, InputValue(s, "optZ"))
	require.Equal(t, []string{"UI", "API"}, InputValue(m, []interface{}{"optA", "optB"}))
	require.Nil(t, InputValue(m, []interface{}{}))
}

func TestCellValue(t *testing.T) {
	s := selectField("fldSSSSSSS", db.SingleSelectFieldType, "Bug", "Feature")
	m := selectField("fldMMMMMMM", db.MultiSelectFieldType, "UI", "API")

	for _, tc := range []struct {
		field  *db.SLField
		result interface{}
		want   interface{}
	}{
		{field("fldTTTTTTT", db.TextFieldType, nil), "  hi  ", "hi"},
		{field("fldTTTTTTT", db.TextFieldType, nil), float64(3), "3"},
		{field("fldTTTTTTT", db.TextFieldType, nil), map[string]interface{}{"a": float64(1)}, `{"a":1}`},
		{field("fldTTTTTTT", db.TextFieldType, nil), "", nil},
		{field("fldNNNNNNN", db.NumberFieldType, nil), "1,234.5 元", 1234.5},
		{field("fldNNNNNNN", db.NumberFieldType, nil), float64(7), float64(7)},
		{field("fldCCCCCCC", db.CheckboxFieldType, nil), "TRUE", true},
		{field("fldDDDDDDD", db.DateTimeFieldType, nil), "2026-10-02", "2026-10-02T00:00:00Z"},
		{field("fldDDDDDDD", db.DateTimeFieldType, nil), float64(0), "1970-01-01T00:00:00Z"},
		{s, "bug", "optA"},
		{s, "optB", "optB"},
		{m, []interface{}{"API", "ui", "API"}, []interface{}{"optB", "optA"}},
		{m, "UI", []interface{}{"optA"}},
		{m, []interface{}{}, nil},
	} {
		got, err := CellValue(tc.field, tc.result)
		require.NoError(t, err, "%v", tc.result)
		require.Equal(t, tc.want, got, "%v", tc.result)
	}

	for _, tc := range []struct {
		field  *db.SLField
		result interface{}
	}{
		{field("fldNNNNNNN", db.NumberFieldType, nil), "1 or 2"},
		{field("fldNNNNNNN", db.NumberFieldType, nil), true},
		{s, "Question"},
		{s, float64(1)},
		{m, []interface{}{"UI", "Other"}},
		{field("fldDDDDDDD", db.DateTimeFieldType, nil), "tomorrow"},
	} {
		_, err := CellValue(tc.field, tc.result)
		var shortcutErr *Error
		require.ErrorAs(t, err, &shortcutErr, "%v", tc.result)
		require.Equal(t, "shortcut::invalid_output", shortcutErr.Key)
	}
}

func TestParseCompletion(t *testing.T) {
	s := selectField("fldSSSSSSS", db.SingleSelectFieldType, "Bug", "UI Bug", "Feature")
	m := selectField("fldMMMMMMM", db.MultiSelectFieldType, "UI", "API", "Docs")

	for _, tc := range []struct {
		field      *db.SLField
		completion string
		want       interface{}
	}{
		{s, "Feature.", "Feature"},
		{s, `"UI Bug"`, "UI Bug"},
		{s, "The category is: UI Bug", "UI Bug"},
		{s, "NONE", nil},
		{m, "```json\n[\"API\", \"Docs\", \"Other\"]\n```", []interface{}{"API", "Docs"}},
		{m, "UI、API", []interface{}{"UI", "API"}},
		{m, "[]", []interface{}{}},
		{field("fldTTTTTTT", db.TextFieldType, nil), "```\nHello\n```", "Hello"},
		{field("fldNNNNNNN", db.NumberFieldType, nil), "None", nil},
	} {
		got, err := parseCompletion(tc.field, tc.completion)
		require.NoError(t, err, tc.completion)
		require.Equal(t, tc.want, got, tc.completion)
	}

	_, err := parseCompletion(m, `["Other"]`)
	require.Error(t, err)
}

func TestValidate(t *testing.T) {
	text := field("fldTTTTTTT", db.TextFieldType, nil)
	formula := field("fldFFFFFFF", db.FormulaFieldType, nil)
	target := selectField("fldSSSSSSS", db.SingleSelectFieldType, "A")
	fields := []*db.SLField{text, formula, target}
	classify := builtins["ai_classify"].definition()

	got, err := Validate(classify, fields, target, &db.FieldShortcut{ID: "ai_classify", Inputs: map[string]interface{}{"source": " fldTTTTTTT ", "unknown": "x"}, AutoUpdate: true})
	require.NoError(t, err)
	require.Equal(t, &db.FieldShortcut{ID: "ai_classify", Inputs: map[string]interface{}{"source": "fldTTTTTTT"}, AutoUpdate: true}, got)

	for name, tc := range map[string]struct {
		def    *Definition
		field  *db.SLField
		inputs map[string]interface{}
		key    string
	}{
		"required":         {classify, target, map[string]interface{}{}, "shortcut::input_required"},
		"self":             {classify, target, map[string]interface{}{"source": "fldSSSSSSS"}, "shortcut::invalid_input_field"},
		"formula":          {classify, target, map[string]interface{}{"source": "fldFFFFFFF"}, "shortcut::invalid_input_field"},
		"missing":          {classify, target, map[string]interface{}{"source": "fldXXXXXXX"}, "shortcut::invalid_input_field"},
		"type":             {classify, text, map[string]interface{}{"source": "fldSSSSSSS"}, "shortcut::unsupported_type"},
		"option":           {builtins["ai_translate"].definition(), text, map[string]interface{}{"source": "fldSSSSSSS", "language": "Klingon"}, "shortcut::invalid_option"},
		"prompt reference": {builtins["ai_custom"].definition(), text, map[string]interface{}{"prompt": "Use {fldFFFFFFF}"}, "shortcut::invalid_input_field"},
	} {
		_, err := Validate(tc.def, fields, tc.field, &db.FieldShortcut{ID: tc.def.ID, Inputs: tc.inputs})
		var shortcutErr *Error
		require.ErrorAs(t, err, &shortcutErr, name)
		require.Equal(t, tc.key, shortcutErr.Key, name)
	}
}

func TestValidateCycle(t *testing.T) {
	a := field("fldAAAAAAA", db.TextFieldType, nil)
	b := field("fldBBBBBBB", db.TextFieldType, nil)
	c := field("fldCCCCCCC", db.TextFieldType, nil)
	b.Shortcut = &db.FieldShortcut{ID: "ai_summarize", Inputs: map[string]interface{}{"source": "fldAAAAAAA"}}
	c.Shortcut = &db.FieldShortcut{ID: "ai_custom", Inputs: map[string]interface{}{"prompt": "Rewrite {fldBBBBBBB}"}}
	fields := []*db.SLField{a, b, c}
	summarize := builtins["ai_summarize"].definition()

	// a <- b <- c, so a can not read c.
	_, err := Validate(summarize, fields, a, &db.FieldShortcut{ID: "ai_summarize", Inputs: map[string]interface{}{"source": "fldCCCCCCC"}})
	var shortcutErr *Error
	require.ErrorAs(t, err, &shortcutErr)
	require.Equal(t, "shortcut::cycle", shortcutErr.Key)

	_, err = Validate(summarize, fields, c, &db.FieldShortcut{ID: "ai_summarize", Inputs: map[string]interface{}{"source": "fldAAAAAAA"}})
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"fldBBBBBBB"}, Dependencies(c.Shortcut))
}

type fakeAI struct {
	reply    string
	messages []openai.Message
	calls    int
}

func (f *fakeAI) Complete(_ context.Context, messages []openai.Message) (string, error) {
	f.calls++
	f.messages = messages
	return f.reply, nil
}

func TestExecuteAI(t *testing.T) {
	text := field("fldTTTTTTT", db.TextFieldType, nil)
	target := selectField("fldSSSSSSS", db.SingleSelectFieldType, "Product", "Service", "Other")
	target.Shortcut = &db.FieldShortcut{ID: "ai_classify", Inputs: map[string]interface{}{"source": "fldTTTTTTT", "requirement": "Product issues first"}}
	fields := []*db.SLField{text, target}
	ai := &fakeAI{reply: "Product"}
	executor := &Executor{AI: func(context.Context) (AIClient, error) { return ai, nil }}
	def := builtins["ai_classify"].definition()

	value, err := executor.Execute(context.Background(), def, fields, target, map[string]interface{}{"fldTTTTTTT": "The app crashes"}, Env{})
	require.NoError(t, err)
	require.Equal(t, "optA", value)
	require.Equal(t, 1, ai.calls)
	require.Contains(t, ai.messages[0].Content, "Product issues first")
	require.Contains(t, ai.messages[1].Content, "- Product\n- Service\n- Other")
	require.Contains(t, ai.messages[1].Content, "The app crashes")

	// Empty inputs clear the cell without calling the model.
	value, err = executor.Execute(context.Background(), def, fields, target, map[string]interface{}{}, Env{})
	require.NoError(t, err)
	require.Nil(t, value)
	require.Equal(t, 1, ai.calls)

	_, err = (&Executor{AI: func(context.Context) (AIClient, error) { return nil, nil }}).Execute(context.Background(), def, fields, target, map[string]interface{}{"fldTTTTTTT": "x"}, Env{})
	var shortcutErr *Error
	require.ErrorAs(t, err, &shortcutErr)
	require.Equal(t, "shortcut::ai_unavailable", shortcutErr.Key)
}

func TestExecutePrompt(t *testing.T) {
	name := field("fldNNNNNNN", db.TextFieldType, nil)
	count := field("fldCCCCCCC", db.NumberFieldType, nil)
	target := field("fldTTTTTTT", db.NumberFieldType, nil)
	target.Shortcut = &db.FieldShortcut{ID: "ai_custom", Inputs: map[string]interface{}{"prompt": "Double {fldCCCCCCC} for {fldNNNNNNN}"}}
	ai := &fakeAI{reply: "42"}
	executor := &Executor{AI: func(context.Context) (AIClient, error) { return ai, nil }}

	value, err := executor.Execute(context.Background(), builtins["ai_custom"].definition(), []*db.SLField{name, count, target}, target,
		map[string]interface{}{"fldCCCCCCC": float64(21), "fldNNNNNNN": "Alice"}, Env{})
	require.NoError(t, err)
	require.Equal(t, float64(42), value)
	require.Equal(t, "Double 21 for Alice", ai.messages[1].Content)
}

func TestExecuteScript(t *testing.T) {
	sso := conf.Auth.SecretKey
	t.Cleanup(func() { conf.Auth.SecretKey = sso })
	conf.Auth.SecretKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

	secrets, err := EncodeSecrets(map[string]string{"token": "abc"})
	require.NoError(t, err)
	decoded, err := DecodeSecrets(secrets)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"token": "abc"}, decoded)

	source := field("fldTTTTTTT", db.TextFieldType, nil)
	target := field("fldNNNNNNN", db.NumberFieldType, nil)
	target.Shortcut = &db.FieldShortcut{ID: "fscAAAAAAA", Inputs: map[string]interface{}{"text": "fldTTTTTTT", "unit": "chars"}}
	custom := &db.CustomFieldShortcut{
		UID:        "fscAAAAAAA",
		ResultType: db.NumberFieldType,
		Code:       `async function execute(params, context) { return params.unit === "chars" ? String(params.text.length) : -1 }`,
		FormItems: datatypes.NewJSONType([]db.ShortcutFormItem{
			{Key: "text", Label: "Text", Component: db.ShortcutFormFieldSelect, Required: true},
			{Key: "unit", Label: "Unit", Component: db.ShortcutFormSelect, Options: []db.ShortcutFormOption{{Value: "chars", Label: "Chars"}}},
		}),
		Domains:        datatypes.NewJSONType([]string{}),
		Credentials:    datatypes.NewJSONType([]db.ShortcutCredential{}),
		Secrets:        secrets,
		TimeoutSeconds: 5,
		Enabled:        true,
	}
	value, err := NewExecutor().Execute(context.Background(), CustomDefinition(custom), []*db.SLField{source, target}, target, map[string]interface{}{"fldTTTTTTT": "hello"}, Env{})
	require.NoError(t, err)
	require.Equal(t, float64(5), value)

	custom.Code = `function execute() { throw new Error("quota exceeded") }`
	_, err = NewExecutor().Execute(context.Background(), CustomDefinition(custom), []*db.SLField{source, target}, target, map[string]interface{}{"fldTTTTTTT": "hello"}, Env{})
	var shortcutErr *Error
	require.ErrorAs(t, err, &shortcutErr)
	require.Equal(t, "shortcut::script_error", shortcutErr.Key)
	require.Equal(t, "Error: quota exceeded", shortcutErr.Detail)
}

func TestErrorText(t *testing.T) {
	tr := func(key string, args ...interface{}) string {
		switch key {
		case "shortcut::input_required":
			return "required: " + args[0].(string)
		case "shortcut::form_source":
			return "Source"
		}
		return key
	}
	require.Equal(t, "required: Source", ErrorText(tr, db.ShortcutJobError{Key: "shortcut::input_required", Args: []string{"shortcut::form_source"}}))
	require.Equal(t, "shortcut::script_error: boom", ErrorText(tr, db.ShortcutJobError{Key: "shortcut::script_error", Detail: "boom"}))
	require.Empty(t, ErrorText(tr, db.ShortcutJobError{}))
}
