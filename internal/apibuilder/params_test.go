package apibuilder

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemovedCustomValidators(t *testing.T) {
	ctx := context.Background()
	params, err := ParseBodyParams([]byte(`[{"key":"text","type":"string","customValidators":[{"type":"text_moderation","expression":"text"}]}]`))
	require.NoError(t, err)
	require.ErrorIs(t, params.ValidateConfig(ctx), ErrUnsupportedCustomValidators)
	value, err := params[0].ValidateValue(ctx, "hello")
	require.ErrorIs(t, err, ErrUnsupportedCustomValidators)
	require.Nil(t, value)

	params[0].CustomValidators = nil
	require.NoError(t, params.ValidateConfig(ctx))
	value, err = params[0].ValidateValue(ctx, "hello")
	require.NoError(t, err)
	require.Equal(t, "hello", value)
}
