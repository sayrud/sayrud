package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/i18n"
)

func TestToManifest(t *testing.T) {
	custom := &db.CustomFieldShortcut{UID: "fscAAAAAAA", Domains: datatypes.NewJSONType([]string{"api.example.com"}), Name: i18n.LocalText{"": "Shortcut"}, Description: i18n.LocalText{"en": "Description"}, ResultType: db.TextFieldType}
	payload, err := json.Marshal(toManifest(custom, false))
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"fscAAAAAAA","kind":"script","name":{"":"Shortcut"},"description":{"en":"Description"},"resultTypes":["text"],"sections":[],"permissions":{"ai":false},"available":false}`, string(payload))

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
		require.Equal(t, tc.aiEnabled, manifest.Permissions.AI)
		require.Equal(t, tc.available, manifest.Available, "%+v", tc)
	}
}
