package i18n

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalText(t *testing.T) {
	var text LocalText
	require.NoError(t, json.Unmarshal([]byte(`"Plain text"`), &text))
	require.Equal(t, "Plain text", text.In("ja"))
	require.NoError(t, json.Unmarshal([]byte(`{"en":" Source ","zh-CN":" 来源 ","ja":" "}`), &text))
	normalized := text.Trim()
	require.Equal(t, " Source ", text["en"], "normalization must not mutate the source definition")
	require.Equal(t, "Source", normalized.In("en-US"))
	require.Equal(t, "来源", normalized.In("zh-TW"))
	require.Equal(t, "Source", normalized.In("fr"))
	require.True(t, normalized.Valid(64, true))
	normalized["zh-CN"] = strings.Repeat("界", 65)
	require.False(t, normalized.Valid(64, true), "validate all translations, not only English")
	for _, raw := range []string{`[]`, `1`, `{"en":1}`} {
		require.Error(t, json.Unmarshal([]byte(raw), &text))
	}
	require.False(t, LocalText{"invalid language": "Source"}.Valid(64, true))
	require.False(t, LocalText{}.Valid(64, true))
}
