package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// RawInput is the raw request input to be validated.
type RawInput struct {
	Query  url.Values
	Header http.Header
	// Body holds the properties of the JSON request body, or nil if no body was provided.
	Body map[string]json.RawMessage
}

// Parameter error codes.
const (
	CodeRequired     = "required"
	CodeNull         = "null"
	CodeEmpty        = "empty"
	CodeInvalidType  = "invalid_type"
	CodeTooShort     = "too_short"
	CodeTooLong      = "too_long"
	CodeTooSmall     = "too_small"
	CodeTooLarge     = "too_large"
	CodeTooManyItems = "too_many_items"
	CodePattern      = "pattern"
	CodeFormat       = "format"
	CodeNotOption    = "not_option"
)

// ParamError is a validation error of a single parameter; Path looks like body.email.
type ParamError struct {
	Path    string `json:"path"`
	Param   string `json:"param"`
	Code    string `json:"code"`
	Message string `json:"message"`
} // @name WorkflowParamError

// ValidateParams strictly parses and validates the parameters and returns the values keyed by parameter name; missing optional parameters without a default are omitted from the result.
func ValidateParams(params []Param, raw RawInput) (map[string]interface{}, []ParamError) {
	input := make(map[string]interface{}, len(params))
	var errs []ParamError
	for i := range params {
		p := &params[i]
		value, present, err := readParam(p, raw)
		if err != nil {
			errs = append(errs, *err)
			continue
		}
		if !present {
			if p.Required {
				errs = append(errs, paramError(p, CodeRequired, "is required"))
				continue
			}
			if len(p.Default) > 0 && string(p.Default) != "null" {
				if v, err := decodeTyped(p, p.Default); err == nil {
					input[p.Key] = v
				}
			}
			continue
		}
		if value == nil {
			if p.Required {
				errs = append(errs, paramError(p, CodeNull, "must not be null"))
				continue
			}
			input[p.Key] = nil
			continue
		}
		if err := checkRules(p, value); err != nil {
			errs = append(errs, *err)
			continue
		}
		input[p.Key] = value
	}
	return input, errs
}

func paramPath(p *Param) string {
	return p.Source + "." + p.Key
}

func paramError(p *Param, code, message string) ParamError {
	return ParamError{Path: paramPath(p), Param: p.Key, Code: code, Message: paramPath(p) + " " + message}
}

// readParam reads the typed value of a parameter; present is false if it is missing, and value is nil for an explicit null.
func readParam(p *Param, raw RawInput) (value interface{}, present bool, perr *ParamError) {
	switch p.Source {
	case SourceBody:
		data, ok := raw.Body[p.Key]
		if !ok {
			return nil, false, nil
		}
		return decodeJSONParam(p, data)
	case SourceQuery:
		values, ok := raw.Query[p.Key]
		if !ok {
			return nil, false, nil
		}
		return parseStrings(p, values)
	case SourceHeader:
		name := http.CanonicalHeaderKey(p.Key)
		values, ok := raw.Header[name]
		if !ok {
			return nil, false, nil
		}
		if p.Type == TypeArray {
			var parts []string
			for _, v := range values {
				for _, part := range strings.Split(v, ",") {
					parts = append(parts, strings.TrimSpace(part))
				}
			}
			values = parts
		}
		return parseStrings(p, values)
	}
	return nil, false, nil
}

func decodeJSONParam(p *Param, data json.RawMessage) (interface{}, bool, *ParamError) {
	if string(bytes.TrimSpace(data)) == "null" {
		return nil, true, nil
	}
	v, err := decodeTyped(p, data)
	if err != nil {
		return nil, true, err
	}
	if s, ok := v.(string); ok && s == "" && p.Required {
		e := paramError(p, CodeEmpty, "must not be empty")
		return nil, true, &e
	}
	return v, true, nil
}

// decodeTyped decodes the value by its original JSON type, without loose conversion between strings and numbers.
func decodeTyped(p *Param, data json.RawMessage) (interface{}, *ParamError) {
	invalid := func() *ParamError {
		e := paramError(p, CodeInvalidType, "must be "+typeName(p))
		return &e
	}
	var v interface{}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, invalid()
	}
	switch p.Type {
	case TypeString, TypeEnum, TypeDate, TypeDateTime:
		s, ok := v.(string)
		if !ok {
			return nil, invalid()
		}
		return normalizeString(p, s)
	case TypeNumber, TypeInteger:
		n, ok := v.(json.Number)
		if !ok {
			return nil, invalid()
		}
		return parseNumber(p, n.String())
	case TypeBoolean:
		b, ok := v.(bool)
		if !ok {
			return nil, invalid()
		}
		return b, nil
	case TypeArray:
		list, ok := v.([]interface{})
		if !ok {
			return nil, invalid()
		}
		out := make([]interface{}, 0, len(list))
		for _, item := range list {
			switch p.ItemType {
			case TypeNumber:
				n, ok := item.(json.Number)
				if !ok {
					return nil, invalid()
				}
				f, err := n.Float64()
				if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
					return nil, invalid()
				}
				out = append(out, f)
			default:
				s, ok := item.(string)
				if !ok {
					return nil, invalid()
				}
				out = append(out, s)
			}
		}
		return out, nil
	}
	return nil, invalid()
}

// parseStrings strictly parses the value from query or header strings.
func parseStrings(p *Param, values []string) (interface{}, bool, *ParamError) {
	if p.Type == TypeArray {
		out := make([]interface{}, 0, len(values))
		for _, s := range values {
			if s == "" {
				continue
			}
			if p.ItemType == TypeNumber {
				f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
				if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
					e := paramError(p, CodeInvalidType, "must be "+typeName(p))
					return nil, true, &e
				}
				out = append(out, f)
				continue
			}
			out = append(out, s)
		}
		return out, true, nil
	}

	s := ""
	if len(values) > 0 {
		s = values[0]
	}
	if s == "" {
		if p.Required {
			e := paramError(p, CodeEmpty, "must not be empty")
			return nil, true, &e
		}
		// An empty string for a non-string type counts as not provided, e.g. ?page= is the same as omitting the parameter.
		if p.Type != TypeString {
			return nil, false, nil
		}
		return "", true, nil
	}

	switch p.Type {
	case TypeNumber, TypeInteger:
		v, err := parseNumber(p, strings.TrimSpace(s))
		return v, true, err
	case TypeBoolean:
		switch s {
		case "true":
			return true, true, nil
		case "false":
			return false, true, nil
		}
		e := paramError(p, CodeInvalidType, "must be true or false")
		return nil, true, &e
	default:
		v, err := normalizeString(p, s)
		return v, true, err
	}
}

func parseNumber(p *Param, s string) (interface{}, *ParamError) {
	invalid := func() *ParamError {
		e := paramError(p, CodeInvalidType, "must be "+typeName(p))
		return &e
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return nil, invalid()
	}
	if p.Type == TypeInteger && (f != math.Trunc(f) || math.Abs(f) > 1<<53) {
		return nil, invalid()
	}
	return f, nil
}

func normalizeString(p *Param, s string) (interface{}, *ParamError) {
	switch p.Type {
	case TypeDate:
		t, err := time.Parse(time.DateOnly, s)
		if err != nil {
			e := paramError(p, CodeInvalidType, "must be a date like 2006-01-02")
			return nil, &e
		}
		return t.Format(time.DateOnly), nil
	case TypeDateTime:
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			e := paramError(p, CodeInvalidType, "must be an RFC 3339 date time")
			return nil, &e
		}
		return t.Format(time.RFC3339), nil
	}
	return s, nil
}

func typeName(p *Param) string {
	switch p.Type {
	case TypeNumber:
		return "a number"
	case TypeInteger:
		return "an integer"
	case TypeBoolean:
		return "a boolean"
	case TypeArray:
		if p.ItemType == TypeNumber {
			return "an array of numbers"
		}
		return "an array of strings"
	default:
		return "a string"
	}
}

var (
	emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	phonePattern = regexp.MustCompile(`^\+?[0-9][0-9\-\s()]{4,19}$`)

	patternCache sync.Map
)

func compilePattern(pattern string) (*regexp.Regexp, error) {
	if re, ok := patternCache.Load(pattern); ok {
		return re.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	patternCache.Store(pattern, re)
	return re, nil
}

func checkRules(p *Param, value interface{}) *ParamError {
	fail := func(code, message string) *ParamError {
		e := paramError(p, code, message)
		return &e
	}
	switch v := value.(type) {
	case string:
		n := utf8.RuneCountInString(v)
		if p.MinLength != nil && n < *p.MinLength {
			return fail(CodeTooShort, fmt.Sprintf("must be at least %d characters", *p.MinLength))
		}
		if p.MaxLength != nil && n > *p.MaxLength {
			return fail(CodeTooLong, fmt.Sprintf("must be at most %d characters", *p.MaxLength))
		}
		if p.Type == TypeEnum && !contains(p.Options, v) {
			return fail(CodeNotOption, "must be one of "+strings.Join(p.Options, ", "))
		}
		if v == "" {
			return nil
		}
		if p.Pattern != "" {
			if re, err := compilePattern(p.Pattern); err == nil && !re.MatchString(v) {
				return fail(CodePattern, "does not match the pattern")
			}
		}
		switch p.Format {
		case FormatEmail:
			if !emailPattern.MatchString(v) {
				return fail(CodeFormat, "must be an email address")
			}
		case FormatURL:
			u, err := url.Parse(v)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return fail(CodeFormat, "must be an http(s) URL")
			}
		case FormatPhone:
			if !phonePattern.MatchString(v) {
				return fail(CodeFormat, "must be a phone number")
			}
		}
	case float64:
		if p.Min != nil && v < *p.Min {
			return fail(CodeTooSmall, "must be at least "+strconv.FormatFloat(*p.Min, 'f', -1, 64))
		}
		if p.Max != nil && v > *p.Max {
			return fail(CodeTooLarge, "must be at most "+strconv.FormatFloat(*p.Max, 'f', -1, 64))
		}
	case []interface{}:
		if p.MaxItems != nil && len(v) > *p.MaxItems {
			return fail(CodeTooManyItems, fmt.Sprintf("must have at most %d items", *p.MaxItems))
		}
		if len(p.Options) > 0 {
			for _, item := range v {
				if s, ok := item.(string); ok && !contains(p.Options, s) {
					return fail(CodeNotOption, "items must be one of "+strings.Join(p.Options, ", "))
				}
			}
		}
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
