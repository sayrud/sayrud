package sso

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// decodeJSON decodes the numbers as json.Number, so the large integer IDs keep their precision.
func decodeJSON(data []byte) (interface{}, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	var v interface{}
	if err := d.Decode(&v); err != nil {
		return nil, errors.Wrap(err, "decode JSON")
	}
	return v, nil
}

// lookupPath returns the value at the dot path, array elements are indexed by numbers, e.g. data.users.0.email.
func lookupPath(v interface{}, path string) (interface{}, bool) {
	if path == "" {
		return nil, false
	}
	for _, part := range strings.Split(path, ".") {
		switch node := v.(type) {
		case map[string]interface{}:
			next, ok := node[part]
			if !ok {
				return nil, false
			}
			v = next
		case []interface{}:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(node) {
				return nil, false
			}
			v = node[i]
		default:
			return nil, false
		}
	}
	return v, v != nil
}

// scalarString converts strings, numbers and booleans to strings, other types become empty.
func scalarString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return strings.TrimSpace(s)
	case json.Number:
		return s.String()
	case float64:
		return strconv.FormatFloat(s, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(s)
	}
	return ""
}

// pathString returns the scalar at the path, or empty if not found.
func pathString(v interface{}, path string) string {
	value, ok := lookupPath(v, path)
	if !ok {
		return ""
	}
	return scalarString(value)
}

// pathStrings returns the strings at the path, a single scalar is one element.
func pathStrings(v interface{}, path string) []string {
	value, ok := lookupPath(v, path)
	if !ok {
		return nil
	}
	list, isList := value.([]interface{})
	if !isList {
		list = []interface{}{value}
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s := scalarString(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}
