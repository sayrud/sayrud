package form

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/flamego/flamego"
	"github.com/sirupsen/logrus"
	"github.com/wuhan005/govalid"

	"github.com/wuhan005/sayrud/internal/i18n"
)

type ErrorCategory string

const (
	ErrorCategoryDeserialization ErrorCategory = "deserialization"
	ErrorCategoryValidation      ErrorCategory = "validation"
)

type Error struct {
	Category ErrorCategory
	Error    error
	// Message is responded to the user in the language of the request.
	Message string
}

var localeType = reflect.TypeOf((*i18n.Locale)(nil)).Elem()

// requestLocale returns the Locale injected by the middleware, or nil.
func requestLocale(c flamego.Context) i18n.Locale {
	if v := c.Value(localeType); v.IsValid() {
		l, _ := v.Interface().(i18n.Locale)
		return l
	}
	return nil
}

func Bind(model interface{}) flamego.Handler {
	// Ensure not pointer.
	if reflect.TypeOf(model).Kind() == reflect.Pointer {
		panic("form: pointer can not be accepted as binding model")
	}

	return flamego.ContextInvoker(func(c flamego.Context) {
		obj := reflect.New(reflect.TypeOf(model))
		r := c.Request().Request
		l := requestLocale(c)
		if r.Body != nil {
			defer func() { _ = r.Body.Close() }()
			err := json.NewDecoder(r.Body).Decode(obj.Interface())
			if err != nil {
				msg := "invalid request body"
				if l != nil {
					msg = l.Translate("common::invalid_body")
				}
				c.Map(Error{Category: ErrorCategoryDeserialization, Error: err, Message: msg})
				if _, err := c.Invoke(errorHandler); err != nil {
					panic("form: " + err.Error())
				}
				return
			}
		}

		if msg, ok := Validate(l, obj.Interface()); !ok {
			c.Map(Error{Category: ErrorCategoryValidation, Error: errors.New(msg), Message: msg})
			if _, err := c.Invoke(errorHandler); err != nil {
				panic("form: " + err.Error())
			}
			return
		}

		// Validation passed.
		c.Map(obj.Elem().Interface())
	})
}

// Validate checks v by the `valid` tags, and returns the first error message in the language of l.
// The field labels are looked up in the locale files by i18n.FieldLabelKey of the field names.
func Validate(l i18n.Locale, v interface{}) (string, bool) {
	errs, ok := govalid.Check(v, i18n.ValidLanguage(l))
	if ok {
		return "", true
	}
	err := errs[0]
	msg := err.Error()
	// govalid prefixes the messages with the labels, which default to the field names.
	if l != nil && err.FieldLabel != "" && strings.HasPrefix(msg, err.FieldLabel) {
		msg = l.Translate(i18n.FieldLabelKey(err.FieldLabel)) + strings.TrimPrefix(msg, err.FieldLabel)
	}
	return msg, false
}

func errorHandler(c flamego.Context, error Error) {
	c.ResponseWriter().WriteHeader(http.StatusBadRequest)
	c.ResponseWriter().Header().Set("Content-Type", "application/json; charset=utf-8")

	body := map[string]interface{}{
		"msg": error.Message,
	}
	err := json.NewEncoder(c.ResponseWriter()).Encode(body)
	if err != nil {
		logrus.WithError(err).Error("Failed to encode response")
	}
}
