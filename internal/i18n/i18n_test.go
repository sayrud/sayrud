package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flamego/flamego"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func serveLang(t *testing.T, setup func(r *http.Request)) string {
	t.Helper()
	f := flamego.New()
	f.Use(Middleware())
	f.Get("/", func(l Locale) string { return l.Lang() })

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	setup(req)
	f.ServeHTTP(resp, req)
	return resp.Body.String()
}

func TestMiddleware(t *testing.T) {
	assert.Equal(t, LangEnUS, serveLang(t, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: CookieName, Value: LangEnUS})
		r.Header.Set("Accept-Language", "zh-CN")
	}))
	assert.Equal(t, LangEnUS, serveLang(t, func(r *http.Request) {
		r.Header.Set("Accept-Language", "en-US,en;q=0.9")
	}))
	assert.Equal(t, LangZhCN, serveLang(t, func(*http.Request) {}))
}

type stubLocale struct{ lang string }

func (l stubLocale) Lang() string        { return l.lang }
func (l stubLocale) Description() string { return "" }
func (l stubLocale) Translate(key string, _ ...interface{}) string {
	return l.lang + ":" + key
}

func TestLocalize(t *testing.T) {
	msg, ok := Localize(stubLocale{LangEnUS}, errors.Wrap(Errorf("common::internal_error"), "save"))
	assert.True(t, ok)
	assert.Equal(t, "en-US:common::internal_error", msg)

	_, ok = Localize(stubLocale{LangEnUS}, errors.New("plain"))
	assert.False(t, ok)
}

func TestValidLanguage(t *testing.T) {
	assert.Equal(t, language.English, ValidLanguage(stubLocale{LangEnUS}))
	assert.Equal(t, language.Chinese, ValidLanguage(stubLocale{LangZhCN}))
	assert.Equal(t, language.Chinese, ValidLanguage(nil))
}

func TestFieldLabelKey(t *testing.T) {
	assert.Equal(t, "form_field::email", FieldLabelKey("Email"))
	assert.Equal(t, "form_field::new_password", FieldLabelKey("NewPassword"))
	assert.Equal(t, "form_field::user_id", FieldLabelKey("UserID"))
	assert.Equal(t, "form_field::field_uid", FieldLabelKey("FieldUID"))
}
