package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/setup"
)

var Setup setupRoute

type setupRoute struct{}

func (setupRoute) LimitBody(ctx context.Context) {
	r := ctx.Request().Request
	r.Body = http.MaxBytesReader(ctx.ResponseWriter(), r.Body, 64<<10)
}

const (
	// SetupTokenHeader carries the setup token of the wizard requests.
	SetupTokenHeader = "X-Setup-Token"
	// SetupRequiredHeader marks the responses of the application APIs before the installation finishes.
	SetupRequiredHeader = "X-Sayrud-Setup"
)

// Authorizer responds 401 unless the request carries the setup token, and 429 after too many failures of the client.
func (setupRoute) Authorizer(ctx context.Context, wizard *setup.Wizard) error {
	err := wizard.Authorize(ctx.IP(), ctx.Request().Header.Get(SetupTokenHeader))
	switch {
	case err == nil:
		return nil
	case errors.Is(err, setup.ErrTooManyAttempts):
		return ctx.ApiError(http.StatusTooManyRequests, "common::too_many_attempts")
	default:
		return ctx.ApiError(http.StatusUnauthorized, "setup::invalid_token")
	}
}

// Unavailable responds 503 to the application APIs before the installation finishes.
func (setupRoute) Unavailable(ctx context.Context) error {
	ctx.ResponseWriter().Header().Set(SetupRequiredHeader, "required")
	return ctx.ApiError(http.StatusServiceUnavailable, "setup::required")
}

// Status
// @Summary Get the installation status
// @Tags Setup
// @Description The setup token is not required. It always responds installed once the application serves the requests.
// @Produce json
// @Success 200 {object} dto.SetupStatus
// @Failure 500 {string} string "Internal server error"
// @ID getSetupStatus
// @Router /setup/status [get]
func (setupRoute) Status(ctx context.Context, wizard *setup.Wizard) error {
	stage := wizard.Stage()
	status := &dto.SetupStatus{Stage: string(stage)}
	switch stage {
	case setup.StageDone:
		return ctx.ApiSuccess(&dto.SetupStatus{Installed: true})
	case setup.StageDatabase:
		status.PasswordMinLength = db.DefaultSystemSettings().PasswordMinLength
	case setup.StageAdmin:
		settings, ok := loadSettings(ctx)
		if !ok {
			return nil
		}
		status.PasswordMinLength = settings.PasswordMinLength
	}
	return ctx.ApiSuccess(status)
}

// Installed serves GET /setup/status of the application.
func (setupRoute) Installed(ctx context.Context) error {
	return ctx.ApiSuccess(&dto.SetupStatus{Installed: true})
}

// Verify
// @Summary Verify the setup token
// @Tags Setup
// @Param X-Setup-Token header string true "Setup token printed in the server log"
// @Success 204 "No Content"
// @Failure 401 {string} string "Wrong setup token"
// @Failure 429 {string} string "Too many attempts"
// @ID verifySetupToken
// @Router /setup/verify [post]
func (setupRoute) Verify(ctx context.Context) error {
	return ctx.Status(http.StatusNoContent)
}

// TestDatabase
// @Summary Test the connections of PostgreSQL and Redis
// @Tags Setup
// @Description The connection errors are responded in the results instead of the status code.
// @Accept json
// @Produce json
// @Param X-Setup-Token header string true "Setup token printed in the server log"
// @Param data body form.SetupDatabase true "Connections"
// @Success 200 {object} dto.SetupDatabaseCheck
// @Failure 400 {string} string "Invalid request body"
// @Failure 401 {string} string "Wrong setup token"
// @Failure 409 {string} string "The database has been configured"
// @Failure 429 {string} string "Too many attempts"
// @ID testSetupDatabase
// @Router /setup/database/test [post]
func (setupRoute) TestDatabase(ctx context.Context, wizard *setup.Wizard, f form.SetupDatabase) error {
	if wizard.Stage() != setup.StageDatabase {
		return ctx.ApiError(http.StatusConflict, "setup::stage_conflict")
	}
	opts, key := databaseOptions(f)
	if key != "" {
		return ctx.ApiError(http.StatusBadRequest, key)
	}

	pg, rdb := setup.CheckServices(ctx.Request().Context(), opts)
	return ctx.ApiSuccess(&dto.SetupDatabaseCheck{
		Postgres: serviceCheck(ctx, "postgres", pg),
		Redis:    serviceCheck(ctx, "redis", rdb),
	})
}

// SaveDatabase
// @Summary Save the database configuration
// @Tags Setup
// @Description Test the connections, save the configuration file and connect the services. The wizard then moves to the admin step.
// @Accept json
// @Param X-Setup-Token header string true "Setup token printed in the server log"
// @Param data body form.SetupDatabase true "Connections"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid request body or unusable services"
// @Failure 401 {string} string "Wrong setup token"
// @Failure 409 {string} string "Another step is in progress, the database has been configured, or the configuration file exists"
// @Failure 429 {string} string "Too many attempts"
// @Failure 500 {string} string "Internal server error"
// @ID saveSetupDatabase
// @Router /setup/database [post]
func (setupRoute) SaveDatabase(ctx context.Context, wizard *setup.Wizard, f form.SetupDatabase) error {
	opts, key := databaseOptions(f)
	if key != "" {
		return ctx.ApiError(http.StatusBadRequest, key)
	}

	err := wizard.ConfigureDatabase(ctx.Request().Context(), opts)
	var checkErr *setup.CheckError
	switch {
	case err == nil:
		return ctx.Status(http.StatusNoContent)
	case errors.Is(err, setup.ErrBusy):
		return ctx.ApiError(http.StatusConflict, "setup::busy")
	case errors.Is(err, setup.ErrStage):
		return ctx.ApiError(http.StatusConflict, "setup::stage_conflict")
	case errors.As(err, &checkErr):
		return ctx.ApiErrorMessage(http.StatusBadRequest, checkMessage(ctx, checkErr.Service, checkErr.Err))
	case errors.Is(err, setup.ErrConfigExists):
		return ctx.ApiError(http.StatusConflict, "setup::config_exists")
	case errors.Is(err, setup.ErrRestartRequired):
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to start after saving the configuration")
		return ctx.ApiError(http.StatusInternalServerError, "setup::restart_required")
	default:
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to configure database")
		return ctx.ApiServerError()
	}
}

// CreateAdmin
// @Summary Create the admin and finish the installation
// @Tags Setup
// @Description Create the first admin and sign in. The application serves the requests afterwards.
// @Accept json
// @Produce json
// @Param X-Setup-Token header string true "Setup token printed in the server log"
// @Param data body form.SetupAdmin true "Admin account"
// @Success 200 {object} dto.Profile
// @Failure 400 {string} string "Invalid request body"
// @Failure 401 {string} string "Wrong setup token"
// @Failure 409 {string} string "Another step is in progress, the database has not been configured, or it has been installed"
// @Failure 429 {string} string "Too many attempts"
// @Failure 500 {string} string "Internal server error"
// @ID createSetupAdmin
// @Router /setup/admin [post]
func (setupRoute) CreateAdmin(ctx context.Context, wizard *setup.Wizard, f form.SetupAdmin) error {
	// The settings live in the database, which is not connected before the database step.
	if wizard.Stage() == setup.StageDatabase {
		return ctx.ApiError(http.StatusConflict, "setup::stage_conflict")
	}

	if len(f.Password) > 72 {
		return ctx.ApiError(http.StatusBadRequest, "setup::password_too_long")
	}

	settings, ok := loadSettings(ctx)
	if !ok {
		return nil
	}
	if err := passwordTooShort(settings, f.Password, "auth::password_too_short"); err != nil {
		return ctx.ApiErrorFrom(http.StatusBadRequest, err)
	}

	userName := strings.TrimSpace(f.UserName)
	if userName == "" {
		return ctx.ApiError(http.StatusBadRequest, "auth::user_name_required")
	}

	user, err := wizard.CreateAdmin(ctx.Request().Context(), db.CreateUserOptions{
		Email:    f.Email,
		UserName: userName,
		Password: f.Password,
	})
	switch {
	case err == nil:
	case errors.Is(err, setup.ErrBusy):
		return ctx.ApiError(http.StatusConflict, "setup::busy")
	case errors.Is(err, setup.ErrInstalled):
		return ctx.ApiError(http.StatusConflict, "setup::installed")
	case errors.Is(err, setup.ErrStage):
		return ctx.ApiError(http.StatusConflict, "setup::stage_conflict")
	default:
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create admin")
		return ctx.ApiServerError()
	}

	if err := startSession(ctx, user, settings.SessionTTL()); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to start session")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToProfile(user))
}

// databaseOptions converts the form, or returns the message key of the invalid value.
func databaseOptions(f form.SetupDatabase) (setup.DatabaseOptions, string) {
	address := strings.TrimSpace(f.Redis.Address)
	if host, port, err := net.SplitHostPort(address); err != nil || host == "" || port == "" {
		return setup.DatabaseOptions{}, "setup::invalid_redis_address"
	}
	return setup.DatabaseOptions{
		Postgres: setup.PostgresOptions{
			Host:     strings.TrimSpace(f.Postgres.Host),
			Port:     f.Postgres.Port,
			Database: strings.TrimSpace(f.Postgres.Database),
			User:     strings.TrimSpace(f.Postgres.Username),
			Password: f.Postgres.Password,
			SSLMode:  f.Postgres.SSLMode,
		},
		Redis: setup.RedisOptions{
			Address:  address,
			Username: f.Redis.Username,
			Password: f.Redis.Password,
		},
	}, ""
}

func serviceCheck(ctx context.Context, service string, result setup.CheckResult) dto.SetupServiceCheck {
	if result.Err != nil {
		return dto.SetupServiceCheck{Version: result.Version, Message: checkMessage(ctx, service, result.Err)}
	}
	return dto.SetupServiceCheck{OK: true, Version: result.Version}
}

// checkMessage explains the failed connection test in the language of the request.
func checkMessage(ctx context.Context, service string, err error) string {
	switch {
	case errors.Is(err, setup.ErrDatabaseInUse):
		return ctx.Tr("setup::database_in_use")
	case service == "postgres":
		return ctx.Tr("setup::postgres_unreachable", err.Error())
	default:
		return ctx.Tr("setup::redis_unreachable", err.Error())
	}
}
