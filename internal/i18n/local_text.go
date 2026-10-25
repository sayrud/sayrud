package i18n

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
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

var localTextLanguage = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// Trim copies the translations without surrounding whitespace.
func (t LocalText) Trim() LocalText {
	if len(t) == 0 {
		return nil
	}
	result := make(LocalText, len(t))
	for lang, value := range t {
		if value = strings.TrimSpace(value); value != "" {
			result[lang] = value
		}
	}
	return result
}

// Valid checks every translation, including those outside the current language.
func (t LocalText) Valid(maxLength int, required bool) bool {
	if required && strings.TrimSpace(t.In("en")) == "" || len(t) > 20 {
		return false
	}
	for lang, value := range t {
		if lang != "" && !localTextLanguage.MatchString(lang) || utf8.RuneCountInString(value) > maxLength {
			return false
		}
	}
	return true
}
