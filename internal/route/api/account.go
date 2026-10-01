package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/redis"
)

var Account accountRoute

type accountRoute struct{}

// ListSessions
// @Summary List the sign-in sessions of the signed-in user
// @Description The unexpired sessions, the newest first.
// @Produce json
// @Success 200 {array} dto.UserSession
// @Failure 401 {string} string "Not signed in"
// @Failure 500 {string} string "Internal server error"
// @ID listSessions
// @Router /auth/sessions [get]
func (accountRoute) ListSessions(ctx context.Context, user *db.User, current *db.UserSession) error {
	sessions, err := db.UserSessions.ListByUserID(ctx.Request().Context(), user.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list sessions")
		return ctx.ApiServerError()
	}
	resp := make([]*dto.UserSession, 0, len(sessions))
	for _, s := range sessions {
		resp = append(resp, dto.ToUserSession(s, current.ID))
	}
	return ctx.ApiSuccess(resp)
}

// RevokeSession
// @Summary Sign out a session of the signed-in user
// @Description The current session can not be revoked here, sign out instead.
// @Param sessionID path int true "Session ID"
// @Success 204 "No Content"
// @Failure 400 {string} string "It is the current session"
// @Failure 401 {string} string "Not signed in"
// @Failure 404 {string} string "Session not found"
// @Failure 500 {string} string "Internal server error"
// @ID revokeSession
// @Router /auth/sessions/{sessionID} [delete]
func (accountRoute) RevokeSession(ctx context.Context, user *db.User, current *db.UserSession) error {
	sessionID := ctx.ParamInt64("sessionID")
	if sessionID == current.ID {
		return ctx.ApiError(http.StatusBadRequest, "当前设备请使用退出登录")
	}
	if err := db.UserSessions.DeleteByID(ctx.Request().Context(), user.ID, sessionID); err != nil {
		if errors.Is(err, db.ErrUserSessionNotFound) {
			return ctx.ApiError(http.StatusNotFound, "该设备已下线")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete session")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// RevokeOtherSessions
// @Summary Sign out all the other sessions of the signed-in user
// @Success 204 "No Content"
// @Failure 401 {string} string "Not signed in"
// @Failure 500 {string} string "Internal server error"
// @ID revokeOtherSessions
// @Router /auth/sessions [delete]
func (accountRoute) RevokeOtherSessions(ctx context.Context, user *db.User) error {
	var current string
	if cookie, err := ctx.Request().Cookie(sessionCookieName); err == nil {
		current = cookie.Value
	}
	if err := db.UserSessions.DeleteByUserID(ctx.Request().Context(), user.ID, current); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete other sessions")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// GetSettings
// @Summary Get the settings of the signed-in user
// @Description The personal settings synced across the devices, the unsaved ones take the defaults.
// @Produce json
// @Success 200 {object} db.UserSettings
// @Failure 401 {string} string "Not signed in"
// @Failure 500 {string} string "Internal server error"
// @ID getUserSettings
// @Router /auth/settings [get]
func (accountRoute) GetSettings(ctx context.Context, user *db.User) error {
	settings, err := db.Settings.GetUser(ctx.Request().Context(), user.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get user settings")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(settings)
}

// UpdateSettings
// @Summary Update the settings of the signed-in user
// @Accept json
// @Produce json
// @Param data body form.UpdateUserSettings true "Settings"
// @Success 200 {object} db.UserSettings
// @Failure 400 {string} string "Invalid settings"
// @Failure 401 {string} string "Not signed in"
// @Failure 500 {string} string "Internal server error"
// @ID updateUserSettings
// @Router /auth/settings [put]
func (accountRoute) UpdateSettings(ctx context.Context, user *db.User, f form.UpdateUserSettings) error {
	settings := db.UserSettings{Theme: f.Theme}
	if err := settings.Validate(); err != nil {
		return ctx.ApiError(http.StatusBadRequest, "%s", err.Error())
	}
	if err := db.Settings.SaveUser(ctx.Request().Context(), user.ID, settings); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to save user settings")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(settings)
}

// DeleteAccount
// @Summary Delete the account of the signed-in user
// @Description The projects owned by the user must be deleted or transferred first, and the last admin can not be deleted.
// @Accept json
// @Param data body form.DeleteAccount true "Password"
// @Success 204 "No Content"
// @Failure 400 {string} string "Wrong password"
// @Failure 401 {string} string "Not signed in"
// @Failure 409 {string} string "The user still owns projects or is the last admin"
// @Failure 429 {string} string "Too many attempts"
// @Failure 500 {string} string "Internal server error"
// @ID deleteAccount
// @Router /auth/account [delete]
func (accountRoute) DeleteAccount(ctx context.Context, hub *collab.Hub, user *db.User, f form.DeleteAccount) error {
	if err := redis.AuthAttempts.CheckPassword(ctx.Request().Context(), user.Email); err != nil {
		if errors.Is(err, redis.ErrTooManyAttempts) {
			return ctx.ApiError(http.StatusTooManyRequests, "尝试次数过多，请稍后再试")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Warn("Failed to check password attempts")
	}
	if _, err := db.Users.Authenticate(ctx.Request().Context(), user.Email, f.Password); err != nil {
		if errors.Is(err, db.ErrBadCredential) {
			return ctx.ApiError(http.StatusBadRequest, "密码错误")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to authenticate")
		return ctx.ApiServerError()
	}

	if err := db.Users.Delete(ctx.Request().Context(), user.ID, 0); err != nil {
		switch {
		case errors.Is(err, db.ErrUserOwnsProjects):
			counts, err := db.Projects.CountByOwnerIDs(ctx.Request().Context(), []int64{user.ID})
			if err != nil {
				logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to count owned projects")
				return ctx.ApiServerError()
			}
			return ctx.ApiError(http.StatusConflict, "请先删除或转移你拥有的 %d 个多维表格", counts[user.ID])
		case errors.Is(err, db.ErrLastAdmin):
			return ctx.ApiError(http.StatusConflict, "你是唯一的管理员，请先设置其他管理员")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete account")
		return ctx.ApiServerError()
	}

	hub.DisconnectUser(user.ID)
	setSessionCookie(ctx, "", -1)
	return ctx.Status(http.StatusNoContent)
}
