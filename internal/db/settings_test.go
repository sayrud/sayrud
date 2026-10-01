package db

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/i18n"
)

func TestParseSystemSettings(t *testing.T) {
	defaults := SystemSettings{SiteName: "Sayrud", AllowSignUp: true, PasswordMinLength: 8, SessionTTLDays: 30}

	require.Equal(t, defaults, ParseSystemSettings(nil, defaults))
	require.Equal(t, defaults, ParseSystemSettings([]byte("not json"), defaults))

	got := ParseSystemSettings([]byte(`{"siteName":"Acme","allowSignUp":false}`), defaults)
	require.Equal(t, SystemSettings{SiteName: "Acme", AllowSignUp: false, PasswordMinLength: 8, SessionTTLDays: 30}, got)

	// Out-of-range numbers and blank site names fall back to the defaults.
	got = ParseSystemSettings([]byte(`{"passwordMinLength":3,"sessionTTLDays":9999,"siteName":"  "}`), defaults)
	require.Equal(t, defaults, got)
}

func TestSystemSettingsValidate(t *testing.T) {
	ok := SystemSettings{SiteName: "Sayrud", AllowSignUp: true, PasswordMinLength: 8, SessionTTLDays: 30}
	require.NoError(t, ok.Validate())

	for _, s := range []SystemSettings{
		{SiteName: "", PasswordMinLength: 8, SessionTTLDays: 30},
		{SiteName: "   ", PasswordMinLength: 8, SessionTTLDays: 30},
		{SiteName: "这是一个非常非常非常非常非常非常非常非常非常非常长的站点名称超过三十二", PasswordMinLength: 8, SessionTTLDays: 30},
		{SiteName: "a", PasswordMinLength: 7, SessionTTLDays: 30},
		{SiteName: "a", PasswordMinLength: 65, SessionTTLDays: 30},
		{SiteName: "a", PasswordMinLength: 8, SessionTTLDays: 0},
		{SiteName: "a", PasswordMinLength: 8, SessionTTLDays: 366},
	} {
		require.Error(t, s.Validate(), "%+v", s)
	}
}

func TestParseUserSettings(t *testing.T) {
	defaults := DefaultUserSettings()
	require.Equal(t, UserSettings{Theme: ThemeLight, Language: i18n.LangZhCN}, defaults)

	require.Equal(t, defaults, ParseUserSettings(nil, defaults))
	require.Equal(t, UserSettings{Theme: ThemeDark, Language: i18n.LangZhCN}, ParseUserSettings([]byte(`{"theme":"dark"}`), defaults))
	require.Equal(t, UserSettings{Theme: ThemeSystem, Language: i18n.LangZhCN}, ParseUserSettings([]byte(`{"theme":"system"}`), defaults))
	require.Equal(t, defaults, ParseUserSettings([]byte(`{"theme":"purple"}`), defaults))
}

func TestUserSettingsValidate(t *testing.T) {
	for _, theme := range []string{ThemeLight, ThemeDark, ThemeSystem} {
		require.NoError(t, UserSettings{Theme: theme, Language: i18n.LangEnUS}.Validate())
	}
	require.Error(t, UserSettings{Theme: "", Language: i18n.LangZhCN}.Validate())
	require.Error(t, UserSettings{Theme: "purple", Language: i18n.LangZhCN}.Validate())
}

func TestUserSettings_Language(t *testing.T) {
	assert.Equal(t, i18n.LangEnUS, ParseUserSettings([]byte(`{"language":"en-US"}`), DefaultUserSettings()).Language)
	assert.Equal(t, i18n.LangZhCN, ParseUserSettings([]byte(`{"language":"fr"}`), DefaultUserSettings()).Language)

	var e *i18n.Error
	require.ErrorAs(t, UserSettings{Theme: ThemeLight, Language: "fr"}.Validate(), &e)
	assert.Equal(t, "settings::invalid_language", e.Key)
	require.ErrorAs(t, UserSettings{Theme: "pink", Language: i18n.LangZhCN}.Validate(), &e)
	assert.Equal(t, "settings::invalid_theme", e.Key)
}

func TestUserSettings_Apply(t *testing.T) {
	current := UserSettings{Theme: ThemeDark, Language: i18n.LangEnUS}
	language := i18n.LangZhCN
	assert.Equal(t, UserSettings{Theme: ThemeDark, Language: i18n.LangZhCN}, current.Apply(nil, &language))
	theme := ThemeSystem
	assert.Equal(t, UserSettings{Theme: ThemeSystem, Language: i18n.LangEnUS}, current.Apply(&theme, nil))
}

func TestSystemSettingsValidate_Key(t *testing.T) {
	var e *i18n.Error
	require.ErrorAs(t, SystemSettings{SiteName: "Sayrud", PasswordMinLength: 3, SessionTTLDays: 30}.Validate(), &e)
	assert.Equal(t, "settings::password_min_length_range", e.Key)
	assert.Equal(t, []interface{}{PasswordMinLengthMin, PasswordMinLengthMax}, e.Args)
}

func TestSettingValues(t *testing.T) {
	values, err := settingValues(UserSettings{Theme: ThemeDark, Language: i18n.LangEnUS})
	require.NoError(t, err)
	require.Equal(t, map[string]json.RawMessage{"theme": json.RawMessage(`"dark"`), "language": json.RawMessage(`"en-US"`)}, values)

	require.JSONEq(t, `{"theme":"dark","other":1}`, string(settingsObject(map[string]json.RawMessage{
		"theme": json.RawMessage(`"dark"`),
		"other": json.RawMessage(`1`),
	})))
}
