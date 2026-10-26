package i18n

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalText(t *testing.T) {
	var text LocalText
	require.NoError(t, json.Unmarshal([]byte(`"Plain text"`), &text))
	require.Equal(t, "Plain text", text.In("ja"))
	require.NoError(t, json.Unmarshal([]byte(`{"en":"Source","zh-CN":"来源"}`), &text))
	require.Equal(t, "Source", text.In("en-US"))
	require.Equal(t, "来源", text.In("zh-TW"))
	require.Equal(t, "Source", text.In("fr"))
	for _, raw := range []string{`[]`, `1`, `{"en":1}`} {
		require.Error(t, json.Unmarshal([]byte(raw), &text))
	}
}

func TestLocalTextScanValue(t *testing.T) {
	value, err := LocalText{"en": "Name"}.Value()
	require.NoError(t, err)
	require.Equal(t, `{"en":"Name"}`, value)
	value, err = LocalText(nil).Value()
	require.NoError(t, err)
	require.Equal(t, "{}", value)

	var text LocalText
	require.NoError(t, text.Scan([]byte(`{"zh-CN":"名称"}`)))
	require.Equal(t, LocalText{"zh-CN": "名称"}, text)
	require.NoError(t, text.Scan(`"Plain"`))
	require.Equal(t, LocalText{"": "Plain"}, text)
	require.NoError(t, text.Scan(nil))
	require.Nil(t, text)
	require.Error(t, text.Scan(1))
}
