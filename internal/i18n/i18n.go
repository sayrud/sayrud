// Package i18n provides the middleware and the error type to translate the messages by the language of the request.
package i18n

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"unicode"

	"github.com/cockroachdb/errors"
	"github.com/flamego/flamego"
	flamei18n "github.com/flamego/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.ini
var localeFS embed.FS

// Locale translates the messages in the language of the request, it is injected by Middleware.
type Locale = flamei18n.Locale

const (
	LangEn   = "en"
	LangZhCN = "zh-CN"
	LangZhTW = "zh-TW"
	LangJa   = "ja"
	LangKo   = "ko"
	LangEs   = "es"
	LangPtBR = "pt-BR"
	LangFr   = "fr"
	LangDe   = "de"
	LangRu   = "ru"

	// CookieName is the cookie of the language, written by the frontend and read by the backend.
	CookieName = "lang"
)

// Languages are the supported languages, the names are written in their own languages.
var Languages = []flamei18n.Language{
	{Name: LangEn, Description: "English"},
	{Name: LangZhCN, Description: "简体中文"},
	{Name: LangZhTW, Description: "繁體中文"},
	{Name: LangJa, Description: "日本語"},
	{Name: LangKo, Description: "한국어"},
	{Name: LangEs, Description: "Español"},
	{Name: LangPtBR, Description: "Português"},
	{Name: LangFr, Description: "Français"},
	{Name: LangDe, Description: "Deutsch"},
	{Name: LangRu, Description: "Русский"},
}

// IsSupported reports whether the language is one of Languages.
func IsSupported(lang string) bool {
	for _, l := range Languages {
		if l.Name == lang {
			return true
		}
	}
	return false
}

var matcher = func() language.Matcher {
	tags := make([]language.Tag, len(Languages))
	for i, l := range Languages {
		tags[i] = language.MustParse(l.Name)
	}
	return language.NewMatcher(tags)
}()

// normalizeAcceptLanguage rewrites Accept-Language to the best supported language name.
// flamego/i18n looks up the locale by the matched tag, which keeps the region of the request
// as an extension (e.g. "en-u-rg-uszzzz" for "en-US") and fails to find the locale.
func normalizeAcceptLanguage(r *http.Request) {
	header := r.Header.Get("Accept-Language")
	if header == "" {
		return
	}
	tags, _, _ := language.ParseAcceptLanguage(header)
	if _, index, confidence := matcher.Match(tags...); confidence != language.No {
		r.Header.Set("Accept-Language", Languages[index].Name)
	} else {
		r.Header.Del("Accept-Language")
	}
}

// Middleware injects the Locale of the request, which is decided by ?lang, the cookie and Accept-Language in order, and defaults to Simplified Chinese.
func Middleware() flamego.Handler {
	sub, err := fs.Sub(localeFS, "locales")
	if err != nil {
		panic("i18n: " + err.Error())
	}
	handler := flamei18n.I18n(flamei18n.Options{
		FileSystem: http.FS(sub),
		Languages:  Languages,
		Default:    LangZhCN,
		NameFormat: "locale_%s.ini",
		// The frontend writes the cookie when switching the language, so it can not be HttpOnly.
		Cookie: flamei18n.CookieOptions{Name: CookieName, HTTPOnly: false},
	})
	return flamego.ContextInvoker(func(c flamego.Context) {
		normalizeAcceptLanguage(c.Request().Request)
		if _, err := c.Invoke(handler); err != nil {
			panic("i18n: " + err.Error())
		}
	})
}

// Error is an error to be translated in the language of the request when writing the response.
type Error struct {
	Key  string
	Args []interface{}
}

func (e *Error) Error() string {
	return e.Key
}

// Errorf returns the *Error of the message key.
func Errorf(key string, args ...interface{}) error {
	return &Error{Key: key, Args: args}
}

// Localize returns the translated message if there is an *Error in the chain of err.
func Localize(l Locale, err error) (string, bool) {
	var e *Error
	if !errors.As(err, &e) {
		return "", false
	}
	return l.Translate(e.Key, e.Args...), true
}

// ValidLanguage returns the language of the govalid error templates, which only has Chinese and English.
func ValidLanguage(l Locale) language.Tag {
	if l == nil || l.Lang() == LangZhCN || l.Lang() == LangZhTW {
		return language.Chinese
	}
	return language.English
}

// FieldLabelKey returns the message key of the label of the form field, e.g. "form_field::new_password" for NewPassword.
func FieldLabelKey(fieldName string) string {
	runes := []rune(fieldName)
	var b strings.Builder
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return "form_field::" + b.String()
}
