package manifest

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/i18n"
)

func TestCheckText(t *testing.T) {
	require.NoError(t, CheckText(nil, "name", false))
	require.NoError(t, CheckText(i18n.LocalText{"": "Name"}, "name", true))
	require.NoError(t, CheckText(i18n.LocalText{"zh-CN": "名称", "en": "Name"}, "name", true))
	require.EqualError(t, CheckText(i18n.LocalText{"": "  "}, "name", true), "name is required")
	require.EqualError(t, CheckText(i18n.LocalText{"en_US": "Name"}, "name", false), `name has an invalid language "en_US"`)
	require.EqualError(t, CheckText(i18n.LocalText{"": strings.Repeat("a", MaxText+1)}, "name", false), "name is longer than 500 characters")
}

func TestCheckKey(t *testing.T) {
	keys := map[string]bool{}
	for _, key := range []string{"a", "_private", "Camel_9", strings.Repeat("k", 64)} {
		require.NoError(t, CheckKey(keys, key, "field"), key)
	}
	require.EqualError(t, CheckKey(keys, "a", "field"), `field "a" is duplicated`)
	for _, key := range []string{"", "9a", "a-b", strings.Repeat("k", 65)} {
		require.EqualError(t, CheckKey(keys, key, "credential"), `credential "`+key+`" has an invalid key`)
	}
}

func TestTimeout(t *testing.T) {
	got, err := Timeout(0, 30, 300)
	require.NoError(t, err)
	require.Equal(t, 30, got)
	got, err = Timeout(300, 30, 300)
	require.NoError(t, err)
	require.Equal(t, 300, got)
	_, err = Timeout(61, 10, 60)
	require.EqualError(t, err, "timeout must be 1–60 seconds")
	_, err = Timeout(-1, 10, 60)
	require.Error(t, err)
}

func TestNormalizeHosts(t *testing.T) {
	got, err := NormalizeHosts([]string{"B.example.com.", "*.a.io", "a.io", "*", "1.2.3.4"})
	require.NoError(t, err)
	require.Equal(t, []string{"*", "1.2.3.4", "a.io", "b.example.com"}, got)

	_, err = NormalizeHosts([]string{"https://a.io"})
	require.EqualError(t, err, `permissions.fetch has an invalid host "https://a.io"`)
	_, err = NormalizeHosts(make([]string, MaxHosts+1))
	require.EqualError(t, err, "permissions.fetch has more than 50 hosts")
}

func TestNormalizeHost(t *testing.T) {
	for in, want := range map[string]string{" Example.COM. ": "example.com", "a-b.c": "a-b.c", "1.2.3.4": "1.2.3.4", "::1": "::1"} {
		got, ok := NormalizeHost(in)
		require.True(t, ok, in)
		require.Equal(t, want, got)
	}
	for _, in := range []string{"", "https://example.com", "a_b.com", "-a.com", "example.com/x", "*.example.com", "*"} {
		_, ok := NormalizeHost(in)
		require.False(t, ok, in)
	}
}

func TestHostAllowed(t *testing.T) {
	hosts := []string{"example.com", "api.test.io"}
	require.True(t, HostAllowed("example.com", hosts))
	require.True(t, HostAllowed("A.Example.com.", hosts))
	require.True(t, HostAllowed("api.test.io", hosts))
	require.False(t, HostAllowed("test.io", hosts))
	require.False(t, HostAllowed("badexample.com", hosts))
	require.False(t, HostAllowed("example.com.evil.net", hosts))
	require.False(t, HostAllowed("", hosts))

	require.True(t, HostAllowed("anything.net", []string{"*"}))
	require.False(t, HostAllowed("", []string{"*"}))
}
