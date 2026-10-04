package shortcut

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

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

func TestRemovedBuiltins(t *testing.T) {
	for _, id := range []string{"ai_classify", "ai_tag", "ai_translate", "ai_summarize", "ai_extract", "ai_custom"} {
		def, err := Lookup(context.Background(), id)
		require.Nil(t, def, id)
		var shortcutErr *Error
		require.ErrorAs(t, err, &shortcutErr, id)
		require.Equal(t, "shortcut::not_found", shortcutErr.Key, id)
	}
}

func TestValidate(t *testing.T) {
	text := field("fldTTTTTTT", db.TextFieldType, nil)
	formula := field("fldFFFFFFF", db.FormulaFieldType, nil)
	target := selectField("fldSSSSSSS", db.SingleSelectFieldType, "A")
	fields := []*db.SLField{text, formula, target}
	classify := &db.CustomFieldShortcut{
		UID:        "fscAAAAAAA",
		ResultType: db.SingleSelectFieldType,
		Code:       `function execute() { return "A" }`,
		FormItems: datatypes.NewJSONType([]db.ShortcutFormItem{
			{Key: "source", Label: "Source", Component: db.ShortcutFormFieldSelect, Required: true},
			{Key: "language", Label: "Language", Component: db.ShortcutFormSelect, Options: []db.ShortcutFormOption{{Value: "English", Label: "English"}}},
			{Key: "prompt", Label: "Prompt", Component: db.ShortcutFormPrompt},
		}),
		Enabled: true,
	}

	got, err := Validate(classify, fields, target, &db.FieldShortcut{ID: classify.UID, Inputs: map[string]interface{}{"source": " fldTTTTTTT ", "unknown": "x"}, AutoUpdate: true})
	require.NoError(t, err)
	require.Equal(t, &db.FieldShortcut{ID: classify.UID, Inputs: map[string]interface{}{"source": "fldTTTTTTT"}, AutoUpdate: true}, got)

	for name, tc := range map[string]struct {
		def    *db.CustomFieldShortcut
		field  *db.SLField
		inputs map[string]interface{}
		key    string
	}{
		"required":         {classify, target, map[string]interface{}{}, "shortcut::input_required"},
		"self":             {classify, target, map[string]interface{}{"source": "fldSSSSSSS"}, "shortcut::invalid_input_field"},
		"formula":          {classify, target, map[string]interface{}{"source": "fldFFFFFFF"}, "shortcut::invalid_input_field"},
		"missing":          {classify, target, map[string]interface{}{"source": "fldXXXXXXX"}, "shortcut::invalid_input_field"},
		"type":             {classify, text, map[string]interface{}{"source": "fldSSSSSSS"}, "shortcut::unsupported_type"},
		"option":           {classify, target, map[string]interface{}{"source": "fldTTTTTTT", "language": "Klingon"}, "shortcut::invalid_option"},
		"prompt reference": {classify, target, map[string]interface{}{"source": "fldTTTTTTT", "prompt": "Use {fldFFFFFFF}"}, "shortcut::invalid_input_field"},
	} {
		_, err := Validate(tc.def, fields, tc.field, &db.FieldShortcut{ID: tc.def.UID, Inputs: tc.inputs})
		var shortcutErr *Error
		require.ErrorAs(t, err, &shortcutErr, name)
		require.Equal(t, tc.key, shortcutErr.Key, name)
	}
}

func TestValidateCycle(t *testing.T) {
	a := field("fldAAAAAAA", db.TextFieldType, nil)
	b := field("fldBBBBBBB", db.TextFieldType, nil)
	c := field("fldCCCCCCC", db.TextFieldType, nil)
	b.Shortcut = &db.FieldShortcut{ID: "fscAAAAAAA", Inputs: map[string]interface{}{"source": "fldAAAAAAA"}}
	c.Shortcut = &db.FieldShortcut{ID: "fscBBBBBBB", Inputs: map[string]interface{}{"prompt": "Rewrite {fldBBBBBBB}"}}
	fields := []*db.SLField{a, b, c}
	summarize := &db.CustomFieldShortcut{
		UID:        "fscAAAAAAA",
		ResultType: db.TextFieldType,
		Code:       `function execute(params) { return params.source }`,
		FormItems: datatypes.NewJSONType([]db.ShortcutFormItem{
			{Key: "source", Label: "Source", Component: db.ShortcutFormFieldSelect, Required: true},
		}),
		Enabled: true,
	}

	// a <- b <- c, so a can not read c.
	_, err := Validate(summarize, fields, a, &db.FieldShortcut{ID: summarize.UID, Inputs: map[string]interface{}{"source": "fldCCCCCCC"}})
	var shortcutErr *Error
	require.ErrorAs(t, err, &shortcutErr)
	require.Equal(t, "shortcut::cycle", shortcutErr.Key)

	_, err = Validate(summarize, fields, c, &db.FieldShortcut{ID: summarize.UID, Inputs: map[string]interface{}{"source": "fldAAAAAAA"}})
	require.NoError(t, err)

	require.ElementsMatch(t, []string{"fldBBBBBBB"}, Dependencies(c.Shortcut))
}

func TestExecutePrompt(t *testing.T) {
	name := field("fldNNNNNNN", db.TextFieldType, nil)
	count := field("fldCCCCCCC", db.NumberFieldType, nil)
	target := field("fldTTTTTTT", db.TextFieldType, nil)
	target.Shortcut = &db.FieldShortcut{ID: "fscAAAAAAA", Inputs: map[string]interface{}{"prompt": "Double {fldCCCCCCC} for {fldNNNNNNN}"}}
	def := &db.CustomFieldShortcut{
		UID:        "fscAAAAAAA",
		ResultType: db.TextFieldType,
		Code:       `function execute(params) { return params.prompt }`,
		FormItems: datatypes.NewJSONType([]db.ShortcutFormItem{
			{Key: "prompt", Label: "Prompt", Component: db.ShortcutFormPrompt, Required: true},
		}),
		Enabled: true,
	}

	value, err := Execute(context.Background(), def, []*db.SLField{name, count, target}, target,
		map[string]interface{}{"fldCCCCCCC": float64(21), "fldNNNNNNN": "Alice"}, Env{})
	require.NoError(t, err)
	require.Equal(t, "Double 21 for Alice", value)
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
	value, err := Execute(context.Background(), custom, []*db.SLField{source, target}, target, map[string]interface{}{"fldTTTTTTT": "hello"}, Env{})
	require.NoError(t, err)
	require.Equal(t, float64(5), value)

	custom.Code = `function execute() { throw new Error("quota exceeded") }`
	// Empty inputs clear the cell without executing the script.
	value, err = Execute(context.Background(), custom, []*db.SLField{source, target}, target, map[string]interface{}{}, Env{})
	require.NoError(t, err)
	require.Nil(t, value)

	_, err = Execute(context.Background(), custom, []*db.SLField{source, target}, target, map[string]interface{}{"fldTTTTTTT": "hello"}, Env{})
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
		case "custom::source":
			return "Source"
		}
		return key
	}
	require.Equal(t, "required: Source", ErrorText(tr, db.ShortcutJobError{Key: "shortcut::input_required", Args: []string{"custom::source"}}))
	require.Equal(t, "shortcut::script_error: boom", ErrorText(tr, db.ShortcutJobError{Key: "shortcut::script_error", Detail: "boom"}))
	require.Empty(t, ErrorText(tr, db.ShortcutJobError{}))
}
