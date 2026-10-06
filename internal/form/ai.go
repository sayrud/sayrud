// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package form

// UpdateAISettings saves the global AI model configuration.
type UpdateAISettings struct {
	// Enabled reports whether the global AI model is enabled.
	Enabled bool `json:"enabled"`
	// BaseURL is the API root including the version, e.g. https://api.openai.com/v1.
	BaseURL string `json:"baseURL"`
	// Model is the name of the model to call, it is required if enabled.
	Model string `json:"model"`
	// APIKey replaces the saved one, empty keeps the saved one.
	APIKey string `json:"apiKey,omitempty"`
	// ClearAPIKey removes the saved API key, it is ignored if APIKey is given.
	ClearAPIKey bool `json:"clearAPIKey,omitempty"`
	// TimeoutSeconds is the timeout of a completion request, between 1 and 120.
	TimeoutSeconds int `json:"timeoutSeconds"`
} // @name UpdateAISettings
