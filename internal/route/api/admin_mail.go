package api

import (
	"net/http"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/sso"
	"github.com/wuhan005/sayrud/internal/workflow"
)

func toAdminMailSettings(s *db.MailSettings) *dto.AdminMailSettings {
	return &dto.AdminMailSettings{
		Enabled: s.Enabled, Host: s.Host, Port: s.Port, Security: s.Security, Username: s.Username,
		HasPassword: s.SealedPassword != "", FromAddress: s.FromAddress, FromName: s.FromName, SecretsReady: sso.SecretsReady(),
	}
}

func buildMailSettings(f form.UpdateMailSettings, requireServer bool) (*db.MailSettings, error) {
	s := &db.MailSettings{
		Enabled: f.Enabled, Host: strings.TrimSpace(f.Host), Port: f.Port, Security: f.Security,
		Username: strings.TrimSpace(f.Username), FromAddress: strings.TrimSpace(f.FromAddress), FromName: strings.TrimSpace(f.FromName),
	}
	check := *s
	check.Enabled = s.Enabled || requireServer
	if err := check.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// GetMailSettings
// @Summary Get the SMTP settings
// @Description Requires the admin. The password is not returned.
// @Produce json
// @Success 200 {object} dto.AdminMailSettings
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID getAdminMailSettings
// @Router /admin/mail [get]
func (adminRoute) GetMailSettings(ctx context.Context) error {
	s, err := db.Settings.GetMail(ctx.Request().Context())
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get mail settings")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(toAdminMailSettings(s))
}

// UpdateMailSettings
// @Summary Update the SMTP settings
// @Description Requires the admin. The password is encrypted by auth.secret_key, empty keeps the saved one.
// @Accept json
// @Produce json
// @Param data body form.UpdateMailSettings true "SMTP settings"
// @Success 200 {object} dto.AdminMailSettings
// @Failure 400 {string} string "Invalid settings"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID updateAdminMailSettings
// @Router /admin/mail [put]
func (adminRoute) UpdateMailSettings(ctx context.Context, f form.UpdateMailSettings) error {
	c := ctx.Request().Context()
	s, err := buildMailSettings(f, false)
	if err != nil {
		return ctx.ApiErrorFrom(http.StatusBadRequest, err)
	}
	saved, err := db.Settings.GetMail(c)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to get mail settings")
		return ctx.ApiServerError()
	}
	switch {
	case f.Password != "":
		if !sso.SecretsReady() {
			return ctx.ApiError(http.StatusBadRequest, "mail::secret_key_required")
		}
		if s.SealedPassword, err = sso.Seal([]byte(f.Password)); err != nil {
			logrus.WithContext(c).WithError(err).Error("Failed to seal the SMTP password")
			return ctx.ApiServerError()
		}
	case !f.ClearPassword:
		s.SealedPassword = saved.SealedPassword
	}
	if err := db.Settings.SaveMail(c, *s); err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to save mail settings")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(toAdminMailSettings(s))
}

// TestMailSettings
// @Summary Send a test email
// @Description Requires the admin. Send an email with the unsaved settings, the saved password is used if it is not given.
// @Accept json
// @Produce json
// @Param data body form.TestMailSettings true "SMTP settings and the recipient"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid settings or sending failed"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID testAdminMailSettings
// @Router /admin/mail/test [post]
func (adminRoute) TestMailSettings(ctx context.Context, f form.TestMailSettings) error {
	c := ctx.Request().Context()
	s, err := buildMailSettings(f.UpdateMailSettings, true)
	if err != nil {
		return ctx.ApiErrorFrom(http.StatusBadRequest, err)
	}
	to, err := mail.ParseAddress(strings.TrimSpace(f.To))
	if err != nil {
		return ctx.ApiError(http.StatusBadRequest, "mail::invalid_recipient")
	}
	password := f.Password
	if password == "" && !f.ClearPassword {
		saved, err := db.Settings.GetMail(c)
		if err != nil {
			logrus.WithContext(c).WithError(err).Error("Failed to get mail settings")
			return ctx.ApiServerError()
		}
		if saved.SealedPassword != "" {
			plain, err := sso.Open(saved.SealedPassword)
			if err != nil {
				return ctx.ApiError(http.StatusBadRequest, "mail::secret_key_required")
			}
			password = string(plain)
		}
	}

	err = workflow.SendSMTP(c, *s, password, workflow.Mail{
		To: []string{to.Address}, Subject: ctx.Tr("mail::test_subject"), Content: ctx.Tr("mail::test_content"),
	})
	if err != nil {
		msg := err.Error()
		var actionErr *workflow.ActionError
		if errors.As(err, &actionErr) {
			msg = actionErr.Message
		}
		if utf8.RuneCountInString(msg) > 300 {
			msg = string([]rune(msg)[:300])
		}
		return ctx.ApiError(http.StatusBadRequest, "mail::test_failed", msg)
	}
	return ctx.Status(http.StatusNoContent)
}
