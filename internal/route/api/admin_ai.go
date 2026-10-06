package api

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/ai"
	"github.com/wuhan005/sayrud/internal/ai/openai"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/sso"
)

func toAdminAISettings(s *db.AISettings) *dto.AdminAISettings {
	apiKey, err := ai.OpenAPIKey(s.SealedAPIKey)
	return &dto.AdminAISettings{
		Enabled:        s.Enabled,
		BaseURL:        s.BaseURL,
		Model:          s.Model,
		TimeoutSeconds: s.TimeoutSeconds,
		APIKeySet:      err == nil && apiKey != "",
		SecretsReady:   sso.SecretsReady(),
	}
}

// buildAISettings validates the form and returns the settings without the API key, requireModel requires the API URL and the model even if disabled.
func buildAISettings(f form.UpdateAISettings, requireModel bool) (*db.AISettings, error) {
	s := &db.AISettings{
		Enabled:        f.Enabled,
		BaseURL:        f.BaseURL,
		Model:          strings.TrimSpace(f.Model),
		TimeoutSeconds: f.TimeoutSeconds,
	}

	check := *s
	check.Enabled = s.Enabled || requireModel
	if err := check.Validate(); err != nil {
		return nil, err
	}
	s.BaseURL, _ = db.NormalizeAIBaseURL(s.BaseURL)
	return s, nil
}

// formAPIKey returns the plaintext API key of the form: the new one if given, empty if cleared, otherwise the saved one if readable.
func formAPIKey(saved *db.AISettings, f form.UpdateAISettings) string {
	if apiKey := strings.TrimSpace(f.APIKey); apiKey != "" {
		return apiKey
	}
	if f.ClearAPIKey {
		return ""
	}
	apiKey, _ := ai.OpenAPIKey(saved.SealedAPIKey)
	return apiKey
}

// GetAISettings
// @Summary Get the AI model settings
// @Description Requires the admin. The API key is not returned.
// @Produce json
// @Success 200 {object} dto.AdminAISettings
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID getAdminAISettings
// @Router /admin/ai [get]
func (adminRoute) GetAISettings(ctx context.Context) error {
	s, err := db.Settings.GetAI(ctx.Request().Context())
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get AI settings")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(toAdminAISettings(s))
}

// UpdateAISettings
// @Summary Update the AI model settings
// @Description Requires the admin. The API key is encrypted by auth.secret_key, empty keeps the saved one. It updates the global AI model configuration immediately.
// @Accept json
// @Produce json
// @Param data body form.UpdateAISettings true "AI model settings"
// @Success 200 {object} dto.AdminAISettings
// @Failure 400 {string} string "Invalid settings"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID updateAdminAISettings
// @Router /admin/ai [put]
func (adminRoute) UpdateAISettings(ctx context.Context, f form.UpdateAISettings) error {
	c := ctx.Request().Context()
	s, err := buildAISettings(f, false)
	if err != nil {
		return ctx.ApiErrorFrom(http.StatusBadRequest, err)
	}
	saved, err := db.Settings.GetAI(c)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to get AI settings")
		return ctx.ApiServerError()
	}

	switch {
	case strings.TrimSpace(f.APIKey) != "":
		if !sso.SecretsReady() {
			return ctx.ApiError(http.StatusBadRequest, "ai::secret_key_required")
		}
		if s.SealedAPIKey, err = ai.SealAPIKey(strings.TrimSpace(f.APIKey)); err != nil {
			logrus.WithContext(c).WithError(err).Error("Failed to seal the API key")
			return ctx.ApiServerError()
		}
	case !f.ClearAPIKey:
		s.SealedAPIKey = saved.SealedAPIKey
	}

	if err := db.Settings.SaveAI(c, *s); err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to save AI settings")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(toAdminAISettings(s))
}

// TestAISettings
// @Summary Test the AI model settings
// @Description Requires the admin. Send a short message to the unsaved model, the saved API key is used if it is not given.
// @Accept json
// @Produce json
// @Param data body form.UpdateAISettings true "AI model settings to test"
// @Success 200 {object} dto.TestAISettingsResp
// @Failure 400 {string} string "Invalid settings or the request failed"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID testAdminAISettings
// @Router /admin/ai/test [post]
func (adminRoute) TestAISettings(ctx context.Context, f form.UpdateAISettings) error {
	c := ctx.Request().Context()
	s, err := buildAISettings(f, true)
	if err != nil {
		return ctx.ApiErrorFrom(http.StatusBadRequest, err)
	}
	saved, err := db.Settings.GetAI(c)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to get AI settings")
		return ctx.ApiServerError()
	}

	start := time.Now()
	reply, err := ai.NewAIClient(s, formAPIKey(saved, f)).Complete(c, []openai.Message{
		{Role: "system", Content: "This is a connectivity test. Reply with OK only."},
		{Role: "user", Content: "ping"},
	})
	if err != nil {
		msg := errors.Cause(err).Error()
		var statusErr *openai.StatusError
		if errors.As(err, &statusErr) {
			msg = statusErr.Error()
		}
		if utf8.RuneCountInString(msg) > 300 {
			msg = string([]rune(msg)[:300])
		}
		return ctx.ApiError(http.StatusBadRequest, "ai::test_failed", msg)
	}

	if utf8.RuneCountInString(reply) > 500 {
		reply = string([]rune(reply)[:500])
	}
	return ctx.ApiSuccess(dto.TestAISettingsResp{Reply: reply, DurationMs: time.Since(start).Milliseconds()})
}
