package collab

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/i18n"
)

type notifyLocale string

func (l notifyLocale) Lang() string                                  { return string(l) }
func (l notifyLocale) Description() string                           { return "" }
func (l notifyLocale) Translate(key string, _ ...interface{}) string { return string(l) + ":" + key }

func TestNotifyTableUsesEachSubscribersLanguage(t *testing.T) {
	hub := NewHub(nil)
	message := &i18n.Message{Key: "failed", Messages: map[string]string{"en": "Request failed", "zh-CN": "请求失败"}}
	clients := map[string]*Client{}
	hub.projects["project"] = map[*Client]struct{}{}
	for _, lang := range []string{"en", "zh-CN"} {
		client := &Client{locale: notifyLocale(lang), tables: map[string]*db.SLTable{"table": {}}, out: make(chan []byte, 1)}
		clients[lang] = client
		hub.projects["project"][client] = struct{}{}
	}
	hub.NotifyTable("project", "table", MessageShortcutJobs, func(tr func(string, ...interface{}) string, lang string) interface{} {
		return tr("script_error") + ": " + message.Text(lang)
	})
	for lang, client := range clients {
		require.Len(t, client.out, 1)
		var notification Message
		require.NoError(t, json.Unmarshal(<-client.out, &notification))
		var text string
		require.NoError(t, json.Unmarshal(notification.Data, &text))
		require.Equal(t, lang+":script_error: "+message.Messages[lang], text)
	}
}

func TestLocalizedValidationArgument(t *testing.T) {
	f := flamego.New()
	f.Use(i18n.Middleware())
	f.Get("/", func(locale i18n.Locale) string {
		client := &Client{locale: locale}
		return client.tr("shortcut::input_required", i18n.LocalText{"en": "Source", "zh-CN": "来源"})
	})
	for lang, label := range map[string]string{"en": "Source", "zh-CN": "来源"} {
		response := httptest.NewRecorder()
		f.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?lang="+lang, nil))
		require.Equal(t, http.StatusOK, response.Code)
		require.Contains(t, response.Body.String(), label)
		require.NotContains(t, response.Body.String(), "map[")
	}
}
