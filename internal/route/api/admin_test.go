package api

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/i18n"
)

func TestSelfOperationError(t *testing.T) {
	require.NoError(t, selfOperationError(1, 2, "admin::cannot_disable_self"))

	var e *i18n.Error
	require.ErrorAs(t, selfOperationError(1, 1, "admin::cannot_disable_self"), &e)
	require.Equal(t, "admin::cannot_disable_self", e.Key)
}
