package shortcut

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/ai/openai"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/sso"
)

// SealAPIKey encrypts the API key of the model by auth.secret_key, it returns empty if the key is empty.
func SealAPIKey(apiKey string) (string, error) {
	if apiKey == "" {
		return "", nil
	}
	return sso.Seal([]byte(apiKey))
}

// OpenAPIKey decrypts the result of SealAPIKey.
func OpenAPIKey(sealed string) (string, error) {
	if sealed == "" {
		return "", nil
	}
	raw, err := sso.Open(sealed)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// NewAIClient returns the client of the model with the plaintext API key.
func NewAIClient(s *db.AISettings, apiKey string) *openai.Client {
	return openai.NewClient(s.BaseURL, apiKey, s.Model, s.Timeout())
}

// AIConfigured reports whether the AI model is enabled and configured in the admin console.
func AIConfigured(ctx context.Context) (bool, error) {
	s, err := db.Settings.GetAI(ctx)
	if err != nil {
		return false, errors.Wrap(err, "get AI settings")
	}
	return s.Configured(), nil
}

// LoadAIClient returns the client of the model configured in the admin console, or nil if it is disabled or not fully configured.
func LoadAIClient(ctx context.Context) (AIClient, error) {
	s, err := db.Settings.GetAI(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get AI settings")
	}
	if !s.Configured() {
		return nil, nil
	}

	apiKey, err := OpenAPIKey(s.SealedAPIKey)
	if err != nil {
		// The key can not be decrypted after auth.secret_key changes, send without it until the admin enters it again.
		logrus.WithContext(ctx).WithError(err).Warn("Failed to decrypt the API key of the AI model")
	}
	return NewAIClient(s, apiKey), nil
}
