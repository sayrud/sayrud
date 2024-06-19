package context

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/flamego/flamego"
	"github.com/flamego/session"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

const (
	UserIDSessionID = "_user_id"
)

// Context represents context of a request.
type Context struct {
	flamego.Context
	IsLogin bool
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

func (c *Context) Status(statusCode int) {
	c.ResponseWriter().WriteHeader(statusCode)
}

func (c *Context) ServerError() {
	// TODO
	c.ResponseWriter().WriteHeader(http.StatusBadGateway)
}

func (c *Context) IP() string {
	// TODO: from request header
	return c.Request().RemoteAddr
}

// Contexter initializes a classic context for a request.
func Contexter(gormDB *gorm.DB) flamego.Handler {
	return func(ctx flamego.Context, session session.Session) {
		c := Context{
			Context: ctx,
			IsLogin: false,
		}

		c.MapTo(gormDB, (*dbutil.Transactor)(nil))
		c.Map(c)
	}
}
