package context

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/flamego/flamego"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

// Context represents context of a request.
type Context struct {
	flamego.Context
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

func (c *Context) ApiError(statusCode int, msg string, v ...interface{}) error {
	c.ResponseWriter().Header().Set("Content-Type", "application/json; charset=utf-8")
	c.ResponseWriter().WriteHeader(statusCode)

	message := fmt.Sprintf(msg, v...)
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

func (c *Context) ApiServerError() error {
	return c.ApiError(http.StatusInternalServerError, "服务器内部错误")
}

func (c *Context) Status(statusCode int) error {
	c.ResponseWriter().WriteHeader(statusCode)
	return nil
}

func (c *Context) ServerError() {
	// TODO
	c.ResponseWriter().WriteHeader(http.StatusBadGateway)
}

// IP returns the client IP from the IP_HEADER header if configured, or the remote address without the port.
func (c *Context) IP() string {
	ip := c.Request().RemoteAddr
	if ipHeader := os.Getenv("IP_HEADER"); ipHeader != "" {
		ip = c.Request().Header.Get(ipHeader)
	}
	if host, _, err := net.SplitHostPort(ip); err == nil {
		return host
	}
	return ip
}

// Contexter initializes a classic context for a request.
func Contexter(gormDB *gorm.DB) flamego.Handler {
	return func(ctx flamego.Context) {
		c := Context{
			Context: ctx,
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
