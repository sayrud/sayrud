package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
)

func TestToManifest(t *testing.T) {
	custom := &db.CustomFieldShortcut{UID: "fscAAAAAAA", Name: "Shortcut", Description: "Description", ResultType: db.TextFieldType}
	payload, err := json.Marshal(toManifest(custom, false))
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"fscAAAAAAA","kind":"script","aiEnabled":false,"name":"Shortcut","description":"Description","resultTypes":["text"],"formItems":[],"available":false}`, string(payload))

	for _, tc := range []struct {
		enabled, aiEnabled, configured, available bool
	}{
		{false, false, false, false},
		{false, true, true, false},
		{true, false, false, true},
		{true, true, false, false},
		{true, true, true, true},
	} {
		custom.Enabled, custom.AIEnabled = tc.enabled, tc.aiEnabled
		manifest := toManifest(custom, tc.configured)
		require.Equal(t, tc.aiEnabled, manifest.AIEnabled)
		require.Equal(t, tc.available, manifest.Available, "%+v", tc)
	}
}
