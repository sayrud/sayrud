package script

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAIComplete(t *testing.T) {
	for _, prefix := range []string{"", "async "} {
		t.Run(prefix+"execute", func(t *testing.T) {
			result, err := Run(context.Background(), Options{
				Code: prefix + `function execute(p, context) {
					return context.ai.complete({prompt: p.prompt, system: "be concise"})
				}`,
				Params: map[string]interface{}{"prompt": "hello"},
				AIComplete: func(ctx context.Context, prompt, system string) (string, error) {
					_, deadline := ctx.Deadline()
					require.True(t, deadline)
					require.Equal(t, "hello", prompt)
					require.Equal(t, "be concise", system)
					return "response", nil
				},
			})
			require.NoError(t, err)
			require.Equal(t, "response", result.Value)
		})
	}

	result, err := Run(context.Background(), Options{
		Code: `async function execute(p, context) {
			return await context.ai.complete({prompt: "hello"})
		}`,
		AIComplete: func(_ context.Context, prompt, system string) (string, error) {
			require.Empty(t, system)
			return prompt, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "hello", result.Value)
}

func TestAICompleteValidation(t *testing.T) {
	for _, argument := range []string{
		``, `null`, `undefined`, `"hello"`, `[]`, `() => ({prompt: "hello"})`, `{}`,
		`{prompt: ""}`, `{prompt: "  "}`, `{prompt: 1}`, `{prompt: null}`, `{prompt: new String("hello")}`,
		`{prompt: "hello", system: null}`, `{prompt: "hello", system: 1}`,
		`{prompt: "hello", model: "other"}`, `{prompt: "hello", url: "https://other"}`,
		`{prompt: "hello", apiKey: "key"}`, `{prompt: "hello", extra: true}`,
		`{prompt: "hello"}, {system: "extra"}`,
	} {
		t.Run(argument, func(t *testing.T) {
			_, err := Run(context.Background(), Options{
				Code: `function execute(p, context) { return context.ai.complete(` + argument + `) }`,
				AIComplete: func(context.Context, string, string) (string, error) {
					t.Fatal("invalid request reached the model")
					return "", nil
				},
			})
			var scriptErr *Error
			require.ErrorAs(t, err, &scriptErr)
			require.Contains(t, scriptErr.Message, "ai.complete:")
		})
	}

	_, err := Run(context.Background(), Options{
		Code:   `function execute(p, context) { return context.ai.complete({prompt: p.prompt, system: "x"}) }`,
		Params: map[string]interface{}{"prompt": strings.Repeat("x", maxRequestBody)},
		AIComplete: func(context.Context, string, string) (string, error) {
			t.Fatal("oversized request reached the model")
			return "", nil
		},
	})
	require.ErrorContains(t, err, "too large")

	_, err = Run(context.Background(), Options{
		Code: `async function execute(p, context) { return await context.ai.complete({prompt: "hello"}) }`,
	})
	require.ErrorIs(t, err, ErrAIDisabled)
}

func TestAICompleteCallLimit(t *testing.T) {
	calls := 0
	_, err := Run(context.Background(), Options{
		Code: `async function execute(p, context) {
			for (let i = 0; i <= p.limit; i++) await context.ai.complete({prompt: "hello"})
		}`,
		Params: map[string]interface{}{"limit": maxRequests},
		AIComplete: func(context.Context, string, string) (string, error) {
			calls++
			return "ok", nil
		},
	})
	require.ErrorContains(t, err, "at most")
	require.Equal(t, maxRequests, calls)
}

type aiTestError struct{ Secret string }

func (*aiTestError) Error() string { return "upstream unavailable" }

func TestAICompleteKeepsHostErrorsPrivate(t *testing.T) {
	want := &aiTestError{Secret: "server-only-secret"}
	for _, prefix := range []string{"", "async "} {
		t.Run(prefix+"execute", func(t *testing.T) {
			await := ""
			if prefix != "" {
				await = "await "
			}
			result, err := Run(context.Background(), Options{
				Code: prefix + `function execute(p, context) {
					try { return ` + await + `context.ai.complete({prompt: "hello"}) }
					catch (e) {
						context.log(e.message, e.value.Secret)
						throw e
					}
				}`,
				AIComplete: func(context.Context, string, string) (string, error) { return "", want },
			})
			var got *aiTestError
			require.ErrorAs(t, err, &got)
			require.Same(t, want, got)
			require.Equal(t, []string{"upstream unavailable undefined"}, result.Logs)
		})
	}
}

func TestAICompleteCanceled(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "timeout"}[deadline], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			timeout := time.Second
			if deadline {
				timeout = 20 * time.Millisecond
			}
			_, err := Run(ctx, Options{
				Code:    `async function execute(p, context) { return await context.ai.complete({prompt: "hello"}) }`,
				Timeout: timeout,
				AIComplete: func(ctx context.Context, _, _ string) (string, error) {
					if !deadline {
						cancel()
					}
					<-ctx.Done()
					return "", ctx.Err()
				},
			})
			if deadline {
				require.ErrorIs(t, err, ErrTimeout)
			} else {
				require.ErrorIs(t, err, context.Canceled)
			}
		})
	}
}
