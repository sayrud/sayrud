// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/redis"
)

var Auth authRoute

type authRoute struct{}

const sessionCookieName = "sayrud_session"

// Authenticator maps the user of the session cookie as *db.User and the session as *db.UserSession, and responds 401 if not signed in or the user is disabled.
func (authRoute) Authenticator(ctx context.Context) error {
	user, session, key, err := sessionUser(ctx)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to authenticate session")
		return ctx.ApiServerError()
	}
	if key != "" {
		return ctx.ApiError(http.StatusUnauthorized, key)
	}
	ctx.Map(user)
	ctx.Map(session)
	return nil
}

// sessionUser returns the enabled user of the session cookie, or the message key of 401 if not signed in, err is only for server errors.
func sessionUser(ctx context.Context) (*db.User, *db.UserSession, string, error) {
	cookie, err := ctx.Request().Cookie(sessionCookieName)
	if err != nil {
		return nil, nil, "auth::sign_in_required", nil
	}
	session, err := db.UserSessions.GetByToken(ctx.Request().Context(), cookie.Value)
	if err != nil {
		if errors.Is(err, db.ErrUserSessionNotFound) {
			return nil, nil, "auth::session_expired", nil
		}
		return nil, nil, "", errors.Wrap(err, "get session")
	}
	user, err := db.Users.GetByID(ctx.Request().Context(), session.UserID)
	if err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			return nil, nil, "auth::account_not_found", nil
		}
		return nil, nil, "", errors.Wrap(err, "get user of session")
	}
	if user.Disabled() {
		if err := db.UserSessions.DeleteByToken(ctx.Request().Context(), cookie.Value); err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete session of disabled user")
		}
		return nil, nil, "auth::account_disabled", nil
	}
	return user, session, "", nil
}

// loadSettings returns the system settings, the 500 response has been written if it fails.
func loadSettings(ctx context.Context) (*db.SystemSettings, bool) {
	settings, err := db.Settings.GetSystem(ctx.Request().Context())
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get system settings")
		_ = ctx.ApiServerError()
		return nil, false
	}
	return settings, true
}

// passwordTooShort returns the error of the message key if the password is shorter than the minimum length, the message takes the minimum length.
func passwordTooShort(settings *db.SystemSettings, password, key string) error {
	if utf8.RuneCountInString(password) < settings.PasswordMinLength {
		return i18n.Errorf(key, settings.PasswordMinLength)
	}
	return nil
}

// SignUp
// @Summary Sign up with email and password
// @Description Create an account and sign in. The first user takes over the projects created before accounts were introduced.
// @Accept json
// @Produce json
// @Param data body form.SignUp true "Account to create"
// @Success 200 {object} dto.Profile
// @Failure 400 {string} string "Invalid request body"
// @Failure 403 {string} string "Signing up is disabled"
// @Failure 409 {string} string "Email has been used"
// @Failure 429 {string} string "Too many attempts"
// @Failure 500 {string} string "Internal server error"
// @ID signUp
// @Router /auth/sign-up [post]
func (authRoute) SignUp(ctx context.Context, f form.SignUp) error {
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	if !settings.AllowSignUp || !settings.AllowPasswordSignIn {
		return ctx.ApiError(http.StatusForbidden, "auth::sign_up_disabled")
	}
	if err := passwordTooShort(settings, f.Password, "auth::password_too_short"); err != nil {
		return ctx.ApiErrorFrom(http.StatusBadRequest, err)
	}
	if err := redis.AuthAttempts.CheckSignUp(ctx.Request().Context(), ctx.IP()); err != nil {
		if errors.Is(err, redis.ErrTooManyAttempts) {
			return ctx.ApiError(http.StatusTooManyRequests, "auth::sign_up_too_frequent")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Warn("Failed to check sign-up attempts")
	}

	userName := strings.TrimSpace(f.UserName)
	if userName == "" {
		return ctx.ApiError(http.StatusBadRequest, "auth::user_name_required")
	}
	options := db.CreateUserOptions{
		Email:    f.Email,
		UserName: userName,
		Password: f.Password,
	}
	user, err := db.Users.ClaimLegacyDefault(ctx.Request().Context(), options)
	if errors.Is(err, db.ErrUserNotFound) {
		user, err = db.Users.Create(ctx.Request().Context(), options)
	}
	if err != nil {
		if errors.Is(err, db.ErrUserAlreadyExisted) {
			return ctx.ApiError(http.StatusConflict, "auth::email_taken")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create user")
		return ctx.ApiServerError()
	}
	if err := db.Users.EnsureAdmin(ctx.Request().Context()); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to ensure admin")
		return ctx.ApiServerError()
	}
	if user, err = db.Users.GetByID(ctx.Request().Context(), user.ID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to reload user")
		return ctx.ApiServerError()
	}

	if err := startSession(ctx, user, settings.SessionTTL()); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to start session")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToProfile(user))
}

// SignIn
// @Summary Sign in with email and password
// @Accept json
// @Produce json
// @Param data body form.SignIn true "Credential"
// @Success 200 {object} dto.Profile
// @Failure 400 {string} string "Invalid request body"
// @Failure 401 {string} string "Wrong email or password"
// @Failure 403 {string} string "The user is disabled, or password sign-in is disabled for non-admins"
// @Failure 429 {string} string "Too many attempts"
// @Failure 500 {string} string "Internal server error"
// @ID signIn
// @Router /auth/sign-in [post]
func (authRoute) SignIn(ctx context.Context, f form.SignIn) error {
	email := db.NormalizeEmail(f.Email)
	if err := redis.AuthAttempts.CheckSignIn(ctx.Request().Context(), ctx.IP(), email); err != nil {
		if errors.Is(err, redis.ErrTooManyAttempts) {
			return ctx.ApiError(http.StatusTooManyRequests, "common::too_many_attempts")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Warn("Failed to check sign-in attempts")
	}

	user, err := db.Users.Authenticate(ctx.Request().Context(), email, f.Password)
	if err != nil {
		if errors.Is(err, db.ErrBadCredential) {
			return ctx.ApiError(http.StatusUnauthorized, "auth::wrong_email_or_password")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to authenticate")
		return ctx.ApiServerError()
	}
	if err := redis.AuthAttempts.ResetSignIn(ctx.Request().Context(), email); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Warn("Failed to reset sign-in attempts")
	}
	if user.Disabled() {
		return ctx.ApiError(http.StatusForbidden, "auth::account_disabled")
	}

	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	if !settings.AllowPasswordSignIn && !user.IsAdmin {
		return ctx.ApiError(http.StatusForbidden, "auth::password_sign_in_disabled")
	}
	if err := startSession(ctx, user, settings.SessionTTL()); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to start session")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToProfile(user))
}

// SignOut
// @Summary Sign out
// @Description Revoke the current session and clear the session cookie.
// @Success 204 "No Content"
// @Failure 500 {string} string "Internal server error"
// @ID signOut
// @Router /auth/sign-out [post]
func (authRoute) SignOut(ctx context.Context) error {
	if cookie, err := ctx.Request().Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		if err := db.UserSessions.DeleteByToken(ctx.Request().Context(), cookie.Value); err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete session")
			return ctx.ApiServerError()
		}
	}
	setSessionCookie(ctx, "", -1)
	return ctx.Status(http.StatusNoContent)
}

// Profile
// @Summary Get the profile of the signed-in user
// @Produce json
// @Success 200 {object} dto.Profile
// @Failure 401 {string} string "Not signed in"
// @Failure 500 {string} string "Internal server error"
// @ID getProfile
// @Router /auth/profile [get]
func (authRoute) Profile(ctx context.Context, user *db.User) error {
	return ctx.ApiSuccess(dto.ToProfile(user))
}

// UpdateProfile
// @Summary Update the profile of the signed-in user
// @Accept json
// @Produce json
// @Param data body form.UpdateProfile true "Profile"
// @Success 200 {object} dto.Profile
// @Failure 400 {string} string "Invalid request body"
// @Failure 401 {string} string "Not signed in"
// @Failure 500 {string} string "Internal server error"
// @ID updateProfile
// @Router /auth/profile [put]
func (authRoute) UpdateProfile(ctx context.Context, user *db.User, f form.UpdateProfile) error {
	userName := strings.TrimSpace(f.UserName)
	if userName == "" {
		return ctx.ApiError(http.StatusBadRequest, "auth::user_name_required")
	}
	if err := db.Users.Update(ctx.Request().Context(), user.ID, db.UpdateUserOptions{UserName: userName}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update user")
		return ctx.ApiServerError()
	}
	user.UserName = userName
	return ctx.ApiSuccess(dto.ToProfile(user))
}

// UpdatePassword
// @Summary Change the password of the signed-in user
// @Description The other sessions of the user are signed out.
// @Accept json
// @Param data body form.UpdatePassword true "Passwords"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid request body, wrong current password or the user has no password"
// @Failure 401 {string} string "Not signed in"
// @Failure 429 {string} string "Too many attempts"
// @Failure 500 {string} string "Internal server error"
// @ID updatePassword
// @Router /auth/password [put]
func (authRoute) UpdatePassword(ctx context.Context, user *db.User, f form.UpdatePassword) error {
	if !user.HasPassword() {
		return ctx.ApiError(http.StatusBadRequest, "auth::no_password")
	}
	if err := redis.AuthAttempts.CheckPassword(ctx.Request().Context(), user.Email); err != nil {
		if errors.Is(err, redis.ErrTooManyAttempts) {
			return ctx.ApiError(http.StatusTooManyRequests, "common::too_many_attempts")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Warn("Failed to check password attempts")
	}
	if _, err := db.Users.Authenticate(ctx.Request().Context(), user.Email, f.OldPassword); err != nil {
		if errors.Is(err, db.ErrBadCredential) {
			return ctx.ApiError(http.StatusBadRequest, "auth::wrong_current_password")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to authenticate")
		return ctx.ApiServerError()
	}
	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	if err := passwordTooShort(settings, f.NewPassword, "auth::new_password_too_short"); err != nil {
		return ctx.ApiErrorFrom(http.StatusBadRequest, err)
	}

	if err := db.Users.UpdatePassword(ctx.Request().Context(), user.ID, f.NewPassword); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update password")
		return ctx.ApiServerError()
	}
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

func startSession(ctx context.Context, user *db.User, ttl time.Duration) error {
	token, err := db.UserSessions.Create(ctx.Request().Context(), db.CreateUserSessionOptions{
		UserID:    user.ID,
		UserAgent: ctx.Request().UserAgent(),
		IP:        ctx.IP(),
		TTL:       ttl,
	})
	if err != nil {
		return errors.Wrap(err, "create session")
	}
	if err := db.Users.TouchSignIn(ctx.Request().Context(), user.ID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Warn("Failed to update last sign-in time")
	}
	setSessionCookie(ctx, token, int(ttl.Seconds()))
	return nil
}

// setSessionCookie sets the session cookie with SameSite=Lax to keep it from cross-site requests, a negative maxAge deletes it.
func setSessionCookie(ctx context.Context, token string, maxAge int) {
	r := ctx.Request()
	http.SetCookie(ctx.ResponseWriter(), &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteLaxMode,
	})
}
