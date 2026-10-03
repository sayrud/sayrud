package db

import (
	"encoding/json"
	"strings"
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

func TestNormalizeExternalURL(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "", true},
		{"  ", "", true},
		{"https://sayrud.example.com", "https://sayrud.example.com", true},
		{"https://sayrud.example.com/", "https://sayrud.example.com", true},
		{"http://127.0.0.1:2830", "http://127.0.0.1:2830", true},
		{"https://example.com/sayrud", "", false},
		{"https://example.com?a=1", "", false},
		{"https://user:pass@example.com", "", false},
		{"ftp://example.com", "", false},
		{"example.com", "", false},
	} {
		got, ok := NormalizeExternalURL(tc.in)
		assert.Equal(t, tc.ok, ok, tc.in)
		assert.Equal(t, tc.want, got, tc.in)
	}
}

func TestSystemSettings_SignIn(t *testing.T) {
	defaults := DefaultSystemSettings()
	assert.True(t, defaults.AllowPasswordSignIn)
	assert.Empty(t, defaults.ExternalURL)

	got := ParseSystemSettings([]byte(`{"externalURL":"https://a.example.com/","allowPasswordSignIn":false}`), defaults)
	assert.Equal(t, "https://a.example.com", got.ExternalURL)
	assert.False(t, got.AllowPasswordSignIn)
	assert.Empty(t, ParseSystemSettings([]byte(`{"externalURL":"https://a.example.com/x"}`), defaults).ExternalURL)

	s := defaults
	s.ExternalURL = "https://a.example.com/x"
	var e *i18n.Error
	require.ErrorAs(t, s.Validate(), &e)
	assert.Equal(t, "settings::invalid_external_url", e.Key)
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
		require.NoError(t, UserSettings{Theme: theme, Language: i18n.LangEn}.Validate())
	}
	require.Error(t, UserSettings{Theme: "", Language: i18n.LangZhCN}.Validate())
	require.Error(t, UserSettings{Theme: "purple", Language: i18n.LangZhCN}.Validate())
}

func TestUserSettings_Language(t *testing.T) {
	assert.Equal(t, i18n.LangEn, ParseUserSettings([]byte(`{"language":"en"}`), DefaultUserSettings()).Language)
	assert.Equal(t, i18n.LangZhCN, ParseUserSettings([]byte(`{"language":"it"}`), DefaultUserSettings()).Language)

	var e *i18n.Error
	require.ErrorAs(t, UserSettings{Theme: ThemeLight, Language: "it"}.Validate(), &e)
	assert.Equal(t, "settings::invalid_language", e.Key)
	require.ErrorAs(t, UserSettings{Theme: "pink", Language: i18n.LangZhCN}.Validate(), &e)
	assert.Equal(t, "settings::invalid_theme", e.Key)
}

func TestUserSettings_Apply(t *testing.T) {
	current := UserSettings{Theme: ThemeDark, Language: i18n.LangEn}
	language := i18n.LangZhCN
	assert.Equal(t, UserSettings{Theme: ThemeDark, Language: i18n.LangZhCN}, current.Apply(nil, &language))
	theme := ThemeSystem
	assert.Equal(t, UserSettings{Theme: ThemeSystem, Language: i18n.LangEn}, current.Apply(&theme, nil))
}

func TestSystemSettingsValidate_Key(t *testing.T) {
	var e *i18n.Error
	require.ErrorAs(t, SystemSettings{SiteName: "Sayrud", PasswordMinLength: 3, SessionTTLDays: 30}.Validate(), &e)
	assert.Equal(t, "settings::password_min_length_range", e.Key)
	assert.Equal(t, []interface{}{PasswordMinLengthMin, PasswordMinLengthMax}, e.Args)
}

func TestSystemSettings_LoginNotice(t *testing.T) {
	defaults := DefaultSystemSettings()
	assert.Empty(t, defaults.LoginNotice)
	assert.Empty(t, ParseSystemSettings([]byte(`{}`), defaults).LoginNotice)

	got := ParseSystemSettings([]byte("{\"loginNotice\":\"  Scheduled maintenance\\nPlease try again later  \"}"), defaults)
	assert.Equal(t, "Scheduled maintenance\nPlease try again later", got.LoginNotice)
	assert.Empty(t, ParseSystemSettings([]byte(`{"loginNotice":"`+strings.Repeat("a", 501)+`"}`), defaults).LoginNotice)

	ok := defaults
	ok.LoginNotice = strings.Repeat("a", 500)
	require.NoError(t, ok.Validate())
	require.NoError(t, defaults.Validate())
	blank := defaults
	blank.LoginNotice = "  \n  "
	require.NoError(t, blank.Validate())

	tooLong := defaults
	tooLong.LoginNotice = strings.Repeat("a", 501)
	var e *i18n.Error
	require.ErrorAs(t, tooLong.Validate(), &e)
	assert.Equal(t, "settings::login_notice_length", e.Key)
	assert.Equal(t, []interface{}{LoginNoticeMaxLength}, e.Args)
}

func TestSettingValues(t *testing.T) {
	values, err := settingValues(UserSettings{Theme: ThemeDark, Language: i18n.LangEn})
	require.NoError(t, err)
	require.Equal(t, map[string]json.RawMessage{"theme": json.RawMessage(`"dark"`), "language": json.RawMessage(`"en"`)}, values)

	require.JSONEq(t, `{"theme":"dark","other":1}`, string(settingsObject(map[string]json.RawMessage{
		"theme": json.RawMessage(`"dark"`),
		"other": json.RawMessage(`1`),
	})))
}

func TestParseAISettings(t *testing.T) {
	defaults := DefaultAISettings()
	require.False(t, defaults.Configured())
	require.Equal(t, defaults, ParseAISettings(nil, defaults))
	require.Equal(t, defaults, ParseAISettings([]byte("not json"), defaults))

	got := ParseAISettings([]byte(`{"enabled":true,"baseURL":"http://127.0.0.1:11434/v1/","model":" qwen3 ","sealedAPIKey":"x","timeoutSeconds":90}`), defaults)
	require.Equal(t, AISettings{Enabled: true, BaseURL: "http://127.0.0.1:11434/v1", Model: "qwen3", SealedAPIKey: "x", TimeoutSeconds: 90}, got)
	require.True(t, got.Configured())

	got = ParseAISettings([]byte(`{"baseURL":"ftp://example.com","timeoutSeconds":9999}`), defaults)
	require.Equal(t, defaults.BaseURL, got.BaseURL)
	require.Equal(t, defaults.TimeoutSeconds, got.TimeoutSeconds)

	values, err := settingValues(aiSettingsRow{AI: got})
	require.NoError(t, err)
	require.Len(t, values, 1)
	require.Contains(t, values, "ai")
}

func TestAISettingsValidate(t *testing.T) {
	require.NoError(t, AISettings{TimeoutSeconds: 60}.Validate())
	require.NoError(t, AISettings{Enabled: true, BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini", TimeoutSeconds: 60}.Validate())

	for _, tc := range []struct {
		s   AISettings
		key string
	}{
		{AISettings{BaseURL: "api.openai.com", TimeoutSeconds: 60}, "ai::invalid_base_url"},
		{AISettings{BaseURL: "https://u:p@api.openai.com/v1", TimeoutSeconds: 60}, "ai::invalid_base_url"},
		{AISettings{Enabled: true, Model: "m", TimeoutSeconds: 60}, "ai::base_url_required"},
		{AISettings{Enabled: true, BaseURL: "https://a.example.com/v1", Model: "  ", TimeoutSeconds: 60}, "ai::model_required"},
		{AISettings{Model: strings.Repeat("m", AIModelMaxLength+1), TimeoutSeconds: 60}, "ai::model_too_long"},
		{AISettings{TimeoutSeconds: 0}, "ai::invalid_timeout"},
		{AISettings{TimeoutSeconds: AITimeoutSecondsMax + 1}, "ai::invalid_timeout"},
	} {
		var e *i18n.Error
		require.ErrorAs(t, tc.s.Validate(), &e, "%+v", tc.s)
		assert.Equal(t, tc.key, e.Key, "%+v", tc.s)
	}
}
