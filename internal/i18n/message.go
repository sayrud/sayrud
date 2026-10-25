package i18n

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/language"
)

// Message is a plugin error translated when it is displayed, including after a job is stored.
type Message struct {
	Key      string                 `json:"key"`
	Messages map[string]string      `json:"messages"`
	Params   map[string]interface{} `json:"params,omitempty"`
}

var (
	pluginMessageKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,63}$`)
	messageParamKey  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,31}$`)
	messageParam     = regexp.MustCompile(`\{([A-Za-z][A-Za-z0-9_]{0,31})\}`)
)

// Valid bounds the metadata a plugin may attach to an error.
func (m *Message) Valid() bool {
	if m == nil || !pluginMessageKey.MatchString(m.Key) || len(m.Messages) == 0 || len(m.Messages) > 20 || len(m.Params) > 20 {
		return false
	}
	for lang, text := range m.Messages {
		if len(lang) > 35 || strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > 1000 {
			return false
		}
		if _, err := language.Parse(lang); lang != "" && err != nil {
			return false
		}
	}
	for key, value := range m.Params {
		if !messageParamKey.MatchString(key) {
			return false
		}
		switch v := value.(type) {
		case string:
			if utf8.RuneCountInString(v) > 500 {
				return false
			}
		case float64, bool:
		default:
			return false
		}
	}
	return true
}

// Text falls back to the base language, English, the default text, then another translation.
func (m *Message) Text(lang string) string {
	text := LocalText(m.Messages).In(lang)
	return messageParam.ReplaceAllStringFunc(text, func(placeholder string) string {
		if value, ok := m.Params[placeholder[1:len(placeholder)-1]]; ok {
			return fmt.Sprint(value)
		}
		return placeholder
	})
}
