// Package i18n provides the middleware and the error type to translate the messages by the language of the request.
package i18n

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"unicode"

	"github.com/flamego/flamego"
	flamei18n "github.com/flamego/i18n"
	"github.com/pkg/errors"
	"golang.org/x/text/language"
)

//go:embed locales/*.ini
var localeFS embed.FS

// Locale translates the messages in the language of the request, it is injected by Middleware.
type Locale = flamei18n.Locale

const (
	LangZhCN = "zh-CN"
	LangEnUS = "en-US"
	// CookieName is the cookie of the language, written by the frontend and read by the backend.
	CookieName = "lang"
)

// Middleware injects the Locale of the request, which is decided by ?lang, the cookie and Accept-Language in order, and defaults to Simplified Chinese.
func Middleware() flamego.Handler {
	sub, err := fs.Sub(localeFS, "locales")
	if err != nil {
		panic("i18n: " + err.Error())
	}
	return flamei18n.I18n(flamei18n.Options{
		FileSystem: http.FS(sub),
		Languages: []flamei18n.Language{
			{Name: LangZhCN, Description: "简体中文"},
			{Name: LangEnUS, Description: "English"},
		},
		Default: LangZhCN,
		// The frontend writes the cookie when switching the language, so it can not be HttpOnly.
		Cookie: flamei18n.CookieOptions{Name: CookieName, HTTPOnly: false},
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

// ValidLanguage returns the language of the govalid error templates.
func ValidLanguage(l Locale) language.Tag {
	if l != nil && l.Lang() == LangEnUS {
		return language.English
	}
	return language.Chinese
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
