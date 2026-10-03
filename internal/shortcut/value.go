package shortcut

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// selectOption is an option of a select field, stored in metadata as `{"options": [{"uid": "...", "name": "..."}]}`.
type selectOption struct {
	UID  string
	Name string
}

func fieldOptions(field *db.SLField) []selectOption {
	metadata, _ := field.Metadata.Data().(map[string]interface{})
	raw, _ := metadata["options"].([]interface{})
	options := make([]selectOption, 0, len(raw))
	for _, item := range raw {
		m, _ := item.(map[string]interface{})
		uid, _ := m["uid"].(string)
		name, _ := m["name"].(string)
		if uid != "" {
			options = append(options, selectOption{UID: uid, Name: name})
		}
	}
	return options
}

// InputValue converts the cell value to the shortcut input: strings for text and dates, numbers, booleans,
// the option name for single select and the option names for multiple select. It returns nil for empty cells.
func InputValue(field *db.SLField, value interface{}) interface{} {
	switch field.Type {
	case db.TextFieldType, db.DateTimeFieldType:
		if s, ok := value.(string); ok && s != "" {
			return s
		}
	case db.NumberFieldType:
		if v, ok := value.(float64); ok {
			return v
		}
	case db.CheckboxFieldType:
		v, _ := value.(bool)
		return v
	case db.SingleSelectFieldType:
		uid, _ := value.(string)
		for _, o := range fieldOptions(field) {
			if o.UID == uid {
				return o.Name
			}
		}
	case db.MultiSelectFieldType:
		uids, _ := value.([]interface{})
		options := fieldOptions(field)
		names := make([]string, 0, len(uids))
		for _, uid := range uids {
			for _, o := range options {
				if o.UID == uid {
					names = append(names, o.Name)
				}
			}
		}
		if len(names) > 0 {
			return names
		}
	}
	return nil
}

// isEmptyInput reports whether the input has no content, an unchecked checkbox is empty.
func isEmptyInput(v interface{}) bool {
	switch v := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case []string:
		return len(v) == 0
	case bool:
		return !v
	}
	return false
}

// InputText returns the text of the input for the prompts.
func InputText(v interface{}) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case []string:
		return strings.Join(v, ", ")
	}
	raw, _ := json.Marshal(v)
	return string(raw)
}

const maxTextResultLength = 100000

// CellValue converts the result of a shortcut to the cell value of the field, it returns *Error "shortcut::invalid_output" if mismatched.
// The select options are matched by name, exactly and then case-insensitively, and unknown names are rejected rather than created.
func CellValue(field *db.SLField, result interface{}) (interface{}, error) {
	if result == nil {
		return nil, nil
	}
	invalid := func(format string, args ...interface{}) error {
		return newError("shortcut::invalid_output").withDetail(format, args...)
	}

	switch field.Type {
	case db.TextFieldType:
		var s string
		switch v := result.(type) {
		case string:
			s = v
		case float64, bool:
			s = InputText(v)
		default:
			raw, _ := json.Marshal(v)
			s = string(raw)
		}

		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		return truncate(s, maxTextResultLength), nil

	case db.NumberFieldType:
		switch v := result.(type) {
		case float64:
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return nil, invalid("%v", v)
			}
			return v, nil
		case string:
			if strings.TrimSpace(v) == "" {
				return nil, nil
			}
			n, ok := parseNumber(v)
			if !ok {
				return nil, invalid("%q is not a number", truncate(v, 50))
			}
			return n, nil
		}
		return nil, invalid("%s is not a number", typeName(result))

	case db.CheckboxFieldType:
		switch v := result.(type) {
		case bool:
			return v, nil
		case float64:
			return v != 0, nil
		case string:
			b, err := strconv.ParseBool(strings.TrimSpace(strings.ToLower(v)))
			if err != nil {
				return nil, invalid("%q is not a boolean", truncate(v, 50))
			}
			return b, nil
		}
		return nil, invalid("%s is not a boolean", typeName(result))

	case db.DateTimeFieldType:
		switch v := result.(type) {
		case float64:
			return time.UnixMilli(int64(v)).UTC().Format(time.RFC3339), nil
		case string:
			if strings.TrimSpace(v) == "" {
				return nil, nil
			}
			t, ok := parseTime(v)
			if !ok {
				return nil, invalid("%q is not a date", truncate(v, 50))
			}
			return t.Format(time.RFC3339), nil
		}
		return nil, invalid("%s is not a date", typeName(result))

	case db.SingleSelectFieldType:
		name, ok := result.(string)
		if !ok {
			return nil, invalid("%s is not an option name", typeName(result))
		}
		if strings.TrimSpace(name) == "" {
			return nil, nil
		}
		uid, ok := matchOption(fieldOptions(field), name)
		if !ok {
			return nil, invalid("%q is not an option", truncate(name, 50))
		}
		return uid, nil

	case db.MultiSelectFieldType:
		var names []interface{}
		switch v := result.(type) {
		case []interface{}:
			names = v
		case []string:
			for _, s := range v {
				names = append(names, s)
			}
		case string:
			names = []interface{}{v}
		default:
			return nil, invalid("%s is not a list of option names", typeName(result))
		}

		options := fieldOptions(field)
		uids := make([]interface{}, 0, len(names))
		seen := map[string]bool{}
		for _, item := range names {
			name, ok := item.(string)
			if !ok {
				return nil, invalid("%s is not an option name", typeName(item))
			}
			if strings.TrimSpace(name) == "" {
				continue
			}
			uid, ok := matchOption(options, name)
			if !ok {
				return nil, invalid("%q is not an option", truncate(name, 50))
			}
			if !seen[uid] {
				seen[uid] = true
				uids = append(uids, uid)
			}
		}
		if len(uids) == 0 {
			return nil, nil
		}
		return uids, nil
	}
	return nil, newError("shortcut::unsupported_type")
}

func matchOption(options []selectOption, name string) (string, bool) {
	name = strings.TrimSpace(name)
	for _, o := range options {
		if o.Name == name || o.UID == name {
			return o.UID, true
		}
	}

	for _, o := range options {
		if strings.EqualFold(strings.TrimSpace(o.Name), name) {
			return o.UID, true
		}
	}
	return "", false
}

var numberPattern = regexp.MustCompile(`[-+]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][-+]?\d+)?`)

// parseNumber parses the number in the text, ignoring the thousands separators, currency symbols and units around it.
func parseNumber(s string) (float64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if v, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) {
		return v, true
	}

	matches := numberPattern.FindAllString(s, 2)
	if len(matches) != 1 {
		return 0, false
	}
	v, err := strconv.ParseFloat(matches[0], 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

var timeLayouts = []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02", "2006/01/02", "2006/01/02 15:04:05"}

func parseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func typeName(v interface{}) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "a string"
	case float64:
		return "a number"
	case bool:
		return "a boolean"
	case []interface{}:
		return "an array"
	case map[string]interface{}:
		return "an object"
	}
	return "an unsupported value"
}
