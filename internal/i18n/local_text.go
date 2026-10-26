package i18n

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// LocalText is a text of a plugin manifest, given as a string or as the translations keyed by language such as en and zh-CN.
type LocalText map[string]string

func (t *LocalText) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*t = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*t = LocalText{"": s}
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return fmt.Errorf("a text must be a string or an object of translations")
	}
	*t = m
	return nil
}

// Value stores the translations as a JSON object.
func (t LocalText) Value() (driver.Value, error) {
	if t == nil {
		return "{}", nil
	}
	b, err := json.Marshal(map[string]string(t))
	return string(b), err
}

// Scan reads the JSON stored by Value, a JSON string is read as the untranslated text.
func (t *LocalText) Scan(src interface{}) error {
	switch v := src.(type) {
	case nil:
		*t = nil
		return nil
	case []byte:
		return t.UnmarshalJSON(v)
	case string:
		return t.UnmarshalJSON([]byte(v))
	}
	return fmt.Errorf("can not scan %T into a text", src)
}

// In returns the text in the language, falling back to the same base language, English, the untranslated string, then any translation.
func (t LocalText) In(lang string) string {
	if len(t) == 0 {
		return ""
	}
	if s, ok := t[lang]; ok {
		return s
	}
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if base, _, _ := strings.Cut(lang, "-"); base != "" {
		for _, k := range keys {
			if kb, _, _ := strings.Cut(k, "-"); strings.EqualFold(kb, base) {
				return t[k]
			}
		}
	}
	for _, k := range []string{"en", ""} {
		if s, ok := t[k]; ok {
			return s
		}
	}
	return t[keys[0]]
}
