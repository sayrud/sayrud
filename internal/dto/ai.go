package dto

// AdminAISettings is the global AI model configuration, the API key is not returned.
type AdminAISettings struct {
	// Enabled reports whether the global AI model is enabled.
	Enabled bool `json:"enabled"`
	// BaseURL is the API root including the version, e.g. https://api.openai.com/v1.
	BaseURL string `json:"baseURL"`
	// Model is the name of the model to call.
	Model string `json:"model"`
	// TimeoutSeconds is the timeout of a completion request.
	TimeoutSeconds int `json:"timeoutSeconds"`
	// APIKeySet reports whether an API key is saved and readable.
	APIKeySet bool `json:"apiKeySet"`
	// SecretsReady is false if auth.secret_key is not configured, the API key can not be saved.
	SecretsReady bool `json:"secretsReady"`
} // @name AdminAISettings

// TestAISettingsResp is the reply of the model to the test message.
type TestAISettingsResp struct {
	// Reply is the content returned by the model, truncated to 500 characters.
	Reply string `json:"reply"`
	// DurationMs is the time of the request in milliseconds.
	DurationMs int64 `json:"durationMs"`
} // @name TestAISettingsResp
