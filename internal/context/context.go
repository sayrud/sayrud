package context

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/flamego/flamego"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/conf"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/i18n"
)

// Context represents context of a request.
type Context struct {
	flamego.Context
	locale i18n.Locale
}

// Locale returns the language of the request.
func (c *Context) Locale() i18n.Locale {
	return c.locale
}

// Tr translates the message key in the language of the request, or returns the key without a locale.
func (c *Context) Tr(key string, args ...interface{}) string {
	if c.locale == nil {
		return key
	}
	return c.locale.Translate(key, args...)
}

func (c *Context) ApiSuccess(data interface{}) error {
	c.ResponseWriter().Header().Set("Content-Type", "application/json; charset=utf-8")
	c.ResponseWriter().WriteHeader(http.StatusOK)

	err := json.NewEncoder(c.ResponseWriter()).Encode(
		map[string]interface{}{
			"msg":  "success",
			"data": data,
		},
	)
	if err != nil {
		logrus.WithContext(c.Request().Context()).WithError(err).Error("Failed to encode error JSON")
	}
	return nil
}

func (c *Context) writeError(statusCode int, message string) error {
	c.ResponseWriter().Header().Set("Content-Type", "application/json; charset=utf-8")
	c.ResponseWriter().WriteHeader(statusCode)

	err := json.NewEncoder(c.ResponseWriter()).Encode(
		map[string]interface{}{
			"msg": message,
		},
	)
	if err != nil {
		logrus.WithContext(c.Request().Context()).WithError(err).Error("Failed to encode error JSON")
	}
	return nil
}

// ApiError responds the message of the key in the language of the request.
func (c *Context) ApiError(statusCode int, key string, args ...interface{}) error {
	return c.writeError(statusCode, c.Tr(key, args...))
}

// ApiErrorMessage responds the message already translated, e.g. the validation errors.
func (c *Context) ApiErrorMessage(statusCode int, message string) error {
	return c.writeError(statusCode, message)
}

// ApiErrorFrom responds the translated *i18n.Error, other errors are responded as internal server errors.
func (c *Context) ApiErrorFrom(statusCode int, err error) error {
	if c.locale != nil {
		if msg, ok := i18n.Localize(c.locale, err); ok {
			return c.writeError(statusCode, msg)
		}
	}
	logrus.WithContext(c.Request().Context()).WithError(err).Error("Unlocalized API error")
	return c.ApiServerError()
}

func (c *Context) ApiServerError() error {
	return c.ApiError(http.StatusInternalServerError, "common::internal_error")
}

func (c *Context) Status(statusCode int) error {
	c.ResponseWriter().WriteHeader(statusCode)
	return nil
}

func (c *Context) ServerError() {
	// TODO
	c.ResponseWriter().WriteHeader(http.StatusBadGateway)
}

// IP returns the client IP from app.ip_header if configured, or the remote address without the port.
func (c *Context) IP() string {
	ip := c.Request().RemoteAddr
	if ipHeader := conf.App.IPHeader; ipHeader != "" {
		ip = c.Request().Header.Get(ipHeader)
	}
	if host, _, err := net.SplitHostPort(ip); err == nil {
		return host
	}
	return ip
}

// Contexter initializes a classic context for a request.
func Contexter(gormDB *gorm.DB) flamego.Handler {
	return func(ctx flamego.Context, l i18n.Locale) {
		c := Context{
			Context: ctx,
			locale:  l,
		}

		spanCtx := trace.SpanContextFromContext(ctx.Request().Context())
		if spanCtx.HasTraceID() {
			c.ResponseWriter().Header().Set("Trace-ID", spanCtx.TraceID().String())
		}

		c.ResponseWriter().Header().Set("Server", "Sayrud")
		c.ResponseWriter().Header().Set("Access-Control-Allow-Origin", "*")
		c.ResponseWriter().Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.ResponseWriter().Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if ctx.Request().Method == http.MethodOptions {
			ctx.ResponseWriter().WriteHeader(http.StatusOK)
			return
		}

		c.MapTo(gormDB, (*dbutil.Transactor)(nil))
		c.Map(c)
	}
}
