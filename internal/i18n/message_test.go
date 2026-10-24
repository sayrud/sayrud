package i18n

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPluginMessage(t *testing.T) {
	m := &Message{
		Key: "request_failed",
		Messages: map[string]string{
			"en": "Request failed (HTTP {status}): {detail}", "zh-CN": "请求失败（HTTP {status}）：{detail}", "ja": "リクエスト失敗（HTTP {status}）：{detail}",
		},
		Params: map[string]interface{}{"status": float64(429), "detail": "{status}"},
	}
	require.True(t, m.Valid())
	for lang, want := range map[string]string{
		"en": "Request failed (HTTP 429): {status}", "en-US": "Request failed (HTTP 429): {status}",
		"zh-CN": "请求失败（HTTP 429）：{status}", "zh-TW": "请求失败（HTTP 429）：{status}",
		"ja-JP": "リクエスト失敗（HTTP 429）：{status}", "de": "Request failed (HTTP 429): {status}",
	} {
		require.Equal(t, want, m.Text(lang), lang)
	}
	require.Equal(t, "默认 {missing}", (&Message{Messages: map[string]string{"": "默认 {missing}", "ja": "日本語"}}).Text("de"))
	require.Equal(t, "Deutsch", (&Message{Messages: map[string]string{"de": "Deutsch", "ja": "日本語"}}).Text("fr"))
}

func TestInvalidPluginMessage(t *testing.T) {
	for _, message := range []*Message{
		nil,
		{Key: "invalid key", Messages: map[string]string{"en": "Error"}},
		{Key: "missing"},
		{Key: "blank", Messages: map[string]string{"en": " \n "}},
		{Key: "oversized", Messages: map[string]string{"en": strings.Repeat("界", 1001)}},
		{Key: "locale", Messages: map[string]string{"not a language": "Error"}},
		{Key: "parameter", Messages: map[string]string{"en": "Error"}, Params: map[string]interface{}{"value": map[string]string{"secret": "nested"}}},
		{Key: "parameter", Messages: map[string]string{"en": "Error"}, Params: map[string]interface{}{"invalid key": "value"}},
		{Key: "parameter", Messages: map[string]string{"en": "Error"}, Params: map[string]interface{}{"value": strings.Repeat("界", 501)}},
	} {
		require.False(t, message.Valid())
	}
}
