package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAttachmentFieldType(t *testing.T) {
	require.True(t, AttachmentFieldType.Check())
	require.Equal(t, "field_type::attachment", AttachmentFieldType.LabelKey())
}
