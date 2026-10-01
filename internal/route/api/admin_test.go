package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSelfOperationError(t *testing.T) {
	require.Equal(t, "", selfOperationError(1, 2, "停用"))
	require.Equal(t, "不能停用自己的账号", selfOperationError(1, 1, "停用"))
}
