package shortcut

import (
	"context"
	"net"
	"strings"

	"github.com/pkg/errors"

	"github.com/wuhan005/sayrud/internal/ai"
	"github.com/wuhan005/sayrud/internal/ai/openai"
	"github.com/wuhan005/sayrud/internal/db"
)

// aiCompleter loads the global model on first use, once per script run.
// The script controls the prompts; the endpoint, model and credentials remain on the server.
func aiCompleter() func(context.Context, string, string) (string, error) {
	var client *openai.Client

	return func(ctx context.Context, prompt, system string) (string, error) {
		if client == nil {
			settings, err := db.Settings.GetAI(ctx)
			if err != nil {
				return "", newError("shortcut::internal_error").transient()
			}
			if !settings.Configured() {
				return "", newError("shortcut::ai_unavailable")
			}

			key, err := ai.OpenAPIKey(settings.SealedAPIKey)
			if err != nil {
				return "", newError("shortcut::ai_key_unavailable")
			}
			client = ai.NewAIClient(settings, key)
		}

		messages := make([]openai.Message, 0, 2)
		if system != "" {
			messages = append(messages, openai.Message{Role: "system", Content: system})
		}
		messages = append(messages, openai.Message{Role: "user", Content: prompt})

		reply, err := client.Complete(ctx, messages)
		if err != nil {
			return "", modelError(ctx, err)
		}

		// Even an upstream response that echoes its Authorization header must not expose the key to JavaScript.
		if client.APIKey != "" {
			reply = strings.ReplaceAll(reply, client.APIKey, "******")
		}

		return reply, nil
	}
}

// modelError preserves retry decisions without exposing upstream response bodies or URLs to the script.
func modelError(ctx context.Context, err error) *Error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return newError("shortcut::timeout")
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return newError("shortcut::canceled").transient()
	}

	var statusErr *openai.StatusError
	if errors.As(err, &statusErr) {
		if statusErr.Temporary() {
			return newError("shortcut::model_unavailable").withDetail("HTTP %d", statusErr.StatusCode).transient()
		}
		return newError("shortcut::model_error").withDetail("HTTP %d", statusErr.StatusCode)
	}

	var netErr net.Error
	if errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded) {
		return newError("shortcut::model_unavailable").transient()
	}

	return newError("shortcut::model_error")
}
