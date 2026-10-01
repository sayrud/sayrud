package db

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
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
	require.Equal(t, UserSettings{Theme: ThemeLight}, defaults)

	require.Equal(t, defaults, ParseUserSettings(nil, defaults))
	require.Equal(t, UserSettings{Theme: ThemeDark}, ParseUserSettings([]byte(`{"theme":"dark"}`), defaults))
	require.Equal(t, UserSettings{Theme: ThemeSystem}, ParseUserSettings([]byte(`{"theme":"system"}`), defaults))
	require.Equal(t, defaults, ParseUserSettings([]byte(`{"theme":"purple"}`), defaults))
}

func TestUserSettingsValidate(t *testing.T) {
	for _, theme := range []string{ThemeLight, ThemeDark, ThemeSystem} {
		require.NoError(t, UserSettings{Theme: theme}.Validate())
	}
	require.Error(t, UserSettings{Theme: ""}.Validate())
	require.Error(t, UserSettings{Theme: "purple"}.Validate())
}

func TestSettingValues(t *testing.T) {
	values, err := settingValues(UserSettings{Theme: ThemeDark})
	require.NoError(t, err)
	require.Equal(t, map[string]json.RawMessage{"theme": json.RawMessage(`"dark"`)}, values)

	require.JSONEq(t, `{"theme":"dark","other":1}`, string(settingsObject(map[string]json.RawMessage{
		"theme": json.RawMessage(`"dark"`),
		"other": json.RawMessage(`1`),
	})))
}
