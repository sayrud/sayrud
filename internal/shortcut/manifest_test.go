package shortcut

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/manifest"
)

const validManifest = `{
  "id": "translate", "homepage": "https://example.com/translate",
  "name": {"en": "Translate", "zh-CN": "翻译"}, "version": "1.0.0", "resultType": "text",
  "permissions": {"fetch": ["*.API.example.com", "*"], "ai": true},
  "credentials": [{"key": "api", "type": "header", "name": "X-Api-Key"}],
  "sections": [
    {"title": "Source", "fields": [
      {"key": "source", "widget": "field_select", "fieldTypes": ["text", "text"], "default": "dropped", "multiline": true},
      {"key": "notes", "widget": "text", "multiline": true, "help": "Shown below", "options": [{"value": "dropped"}]}
    ]},
    {"fields": [{"key": "target", "label": "Target", "widget": "select", "default": "en", "options": [{"value": "en"}, {"value": "zh", "label": "中文"}]}]}
  ]
}`

func TestParseManifest(t *testing.T) {
	m, err := ParseManifest([]byte(validManifest))
	require.NoError(t, err)
	require.Equal(t, "翻译", m.Name.In("zh-CN"))
	require.Equal(t, "translate", m.ID)
	require.Equal(t, "https://example.com/translate", m.Homepage)
	require.Equal(t, DefaultTimeout, m.Timeout)
	require.Equal(t, db.ShortcutPermissions{Fetch: []string{"*", "api.example.com"}, AI: true}, m.Permissions)
	require.Equal(t, []db.ShortcutSection{
		{Title: i18n.LocalText{"": "Source"}, Fields: []db.ShortcutField{
			{Key: "source", Widget: db.ShortcutWidgetFieldSelect, FieldTypes: []db.SLFieldType{db.TextFieldType}},
			{Key: "notes", Widget: db.ShortcutWidgetText, Multiline: true, Help: i18n.LocalText{"": "Shown below"}},
		}},
		{Fields: []db.ShortcutField{{Key: "target", Label: i18n.LocalText{"": "Target"}, Widget: db.ShortcutWidgetSelect, Default: "en",
			Options: []db.ShortcutOption{{Value: "en"}, {Value: "zh", Label: i18n.LocalText{"": "中文"}}}}}},
	}, m.Sections)
	require.Equal(t, "source", db.ShortcutField{Key: "source"}.DisplayLabel().In("en"), "an empty label shows the key")

	m, err = ParseManifest([]byte(`{"name": "Minimal", "version": "1.0.0", "resultType": "number"}`))
	require.NoError(t, err)
	require.Empty(t, m.ID, "local scripts can omit the source package identifier")
	require.Empty(t, m.Homepage)
	require.Equal(t, []string{}, m.Permissions.Fetch)
	require.Equal(t, []db.ShortcutCredential{}, m.Credentials)
	require.Equal(t, []db.ShortcutSection{}, m.Sections)
}

func TestParseManifestErrors(t *testing.T) {
	replace := func(old, new string) string { return strings.Replace(validManifest, old, new, 1) }
	for source, want := range map[string]string{
		replace(`"id": "translate"`, `"id": "invalid-id"`):                  "id must be 2–40 lowercase letters, digits or underscores starting with a letter",
		replace(`"https://example.com/translate"`, `"javascript:alert(1)"`): "homepage must be an http or https URL",
		`[]`: "the manifest can not be decoded: json: cannot unmarshal array into Go value of type shortcut.Manifest",
		replace(`{"en": "Translate", "zh-CN": "翻译"}`, `" "`):                    "name is required",
		replace(`"version": "1.0.0", `, ``):                                     "version is required, e.g. 1.0.0",
		replace(`"resultType": "text"`, `"resultType": "formula"`):              "resultType must be one of text, number, single_select, multi_select, checkbox, datetime",
		replace(`"resultType": "text"`, `"resultType": "text", "timeout": 301`): "timeout must be 1–300 seconds",
		replace(`"*.API.example.com"`, `"https://api.example.com"`):             `permissions.fetch has an invalid host "https://api.example.com"`,
		replace(`"key": "api"`, `"key": "9api"`):                                `credential "9api" has an invalid key`,
		replace(`"type": "header"`, `"type": "cookie"`):                         `credential "api" must have the type bearer, header or query`,
		replace(`"name": "X-Api-Key"`, `"name": ""`):                            `credential "api" must have the name of the header or query parameter`,
		replace(`"name": "X-Api-Key"`, `"name": "X Api"`):                       `credential "api" has an invalid header name "X Api"`,
		replace(`"key": "notes"`, `"key": "source"`):                            `field "source" is duplicated`,
		replace(`"key": "target"`, `"key": "a-b"`):                              `field "a-b" has an invalid key`,
		replace(`"widget": "text"`, `"widget": "textarea"`):                     `field "notes" has an unsupported widget "textarea", use one of field_select, text, select, field_options, prompt`,
		replace(`"fieldTypes": ["text", "text"]`, `"fieldTypes": ["formula"]`):  `field "source" has an invalid field type "formula" in fieldTypes`,
		replace(`"default": "en"`, `"default": "fr"`):                           `field "target" has an invalid default`,
		replace(`{"value": "zh", "label": "中文"}`, `{"value": "en"}`):            `field "target" has an invalid or duplicated option value`,
		replace(`"widget": "select", "default": "en", "options": [{"value": "en"}, {"value": "zh", "label": "中文"}]`, `"widget": "select"`): `field "target" must have 1–100 options`,
		replace(`"help": "Shown below"`, `"help": {"en_US": "Shown below"}`):                                                               `the help of field "notes" has an invalid language "en_US"`,
		replace(`"title": "Source"`, `"title": "`+strings.Repeat("a", 501)+`"`):                                                            "a section title is longer than 500 characters",
		replace(`"widget": "field_select"`, `"widget": "field_options"`):                                                                   `field "source" edits the field options, which needs the resultType single_select or multi_select`,
		replace(`"sections"`, `"formItems"`):                                                                                               "formItems is renamed to sections",
		replace(`"widget": "text"`, `"component": "text"`):                                                                                 `field "notes" uses component, which is renamed to widget`,
	} {
		_, err := ParseManifest([]byte(source))
		var manifestErr *manifest.Error
		require.ErrorAs(t, err, &manifestErr, want)
		require.Equal(t, want, manifestErr.Message)
	}
}
