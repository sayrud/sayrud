package context

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/flamego/flamego"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
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

func (c *Context) Status(statusCode int) error {
	c.ResponseWriter().WriteHeader(statusCode)
	return nil
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
func Contexter(gormDB *gorm.DB, redisClient *redis.Client) flamego.Handler {
	return func(ctx flamego.Context) {
		c := Context{
			Context: ctx,
			IsLogin: false,
		}

		c.ResponseWriter().Header().Set("Access-Control-Allow-Origin", "*")
		c.ResponseWriter().Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.ResponseWriter().Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if ctx.Request().Method == http.MethodOptions {
			ctx.ResponseWriter().WriteHeader(http.StatusOK)
			return
		}

		authorizationHeader := ctx.Request().Header.Get("Authorization")
		if authorizationHeader != "" && strings.HasPrefix(authorizationHeader, "Bearer ") {
			tokenString := strings.TrimSpace(strings.TrimPrefix(authorizationHeader, "Bearer "))

			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, errors.Errorf("unexpected signing method: %q", token.Header["alg"])
				}

				return []byte(os.Getenv("JWT_SECRET")), nil
			})
			if err == nil {
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					userID, _ := claims["userUID"].(string)

					user, err := db.Users.GetByUID(ctx.Request().Context(), userID)
					if err == nil {
						c.IsLogin = true
						c.Map(user)
					}
				}
			}
		}

		c.MapTo(gormDB, (*dbutil.Transactor)(nil))
		c.Map(redisClient)
		c.Map(c)
	}
}
