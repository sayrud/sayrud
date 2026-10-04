package ai

import (
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
