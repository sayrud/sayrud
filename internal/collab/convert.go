package collab

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/thanhpk/randstr"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

// fieldConversion is the result of converting a field to another type.
type fieldConversion struct {
	// metadata is the new field metadata, the options found in the text values are appended when converting to a select field.
	metadata map[string]interface{}
	// values are the new values keyed by record UID, nil clears the value. Records not in it are unchanged.
	values map[string]interface{}
}

// convertField converts the record values of the field to the new type: the values are converted to text first,
// then parsed as the new type, except for a few direct conversions such as select to select and number to checkbox.
// boolText returns the text of the checkbox values, in the language of the user who converts the field.
func convertField(old *db.SLField, newType db.SLFieldType, metadata map[string]interface{}, records []*db.SLRecord, boolText func(bool) string) (*fieldConversion, error) {
	conversion := &fieldConversion{metadata: metadata, values: make(map[string]interface{}, len(records))}

	oldValues := make(map[string]interface{}, len(records))
	texts := make(map[string]string, len(records))
	for _, record := range records {
		data := map[string]interface{}{}
		if err := json.Unmarshal(record.Data, &data); err != nil {
			return nil, errors.Wrapf(err, "decode record %q", record.UID)
		}
		oldValues[record.UID] = data[old.UID]
		texts[record.UID] = valueToText(old, data[old.UID], boolText)
	}

	// The formula values are calculated by the clients rather than stored.
	if newType == db.FormulaFieldType {
		for uid := range oldValues {
			conversion.values[uid] = nil
		}
		return conversion, nil
	}

	isSelect := func(t db.SLFieldType) bool { return t == db.SingleSelectFieldType || t == db.MultiSelectFieldType }
	selectToSelect := isSelect(old.Type) && isSelect(newType)
	if isSelect(newType) && !selectToSelect {
		conversion.metadata = appendOptions(metadata, newType, lo.Values(texts))
	}
	newField := &db.SLField{
		UID:      old.UID,
		Type:     newType,
		Metadata: datatypes.NewJSONType[db.SLFieldMetadata](conversion.metadata),
	}

	for uid, raw := range oldValues {
		var next interface{}
		switch {
		case selectToSelect:
			var values []interface{}
			switch v := raw.(type) {
			case []interface{}:
				values = v
			case string:
				values = lo.Ternary(v == "", nil, []interface{}{v})
			}
			if newType == db.MultiSelectFieldType {
				next = values
			} else if len(values) > 0 {
				next = values[0]
			}
		case old.Type == db.CheckboxFieldType && newType == db.NumberFieldType:
			next = lo.Ternary(raw == true, 1.0, 0.0)
		case old.Type == db.NumberFieldType && newType == db.CheckboxFieldType:
			v, _ := raw.(float64)
			next = v != 0
		case old.Type == db.NumberFieldType && newType == db.TextFieldType:
			if v, ok := raw.(float64); ok {
				next = strconv.FormatFloat(v, 'f', -1, 64)
			}
		case old.Type == db.DateTimeFieldType && newType == db.TextFieldType:
			next = lo.Ternary[interface{}](texts[uid] == "", nil, texts[uid])
		default:
			next = textToValue(newField, texts[uid])
		}

		if !routeutil.IsStoredValue(newField, next) || !routeutil.CheckValue(newField, next) {
			next = nil
		}
		if next == nil && raw == nil {
			continue
		}
		conversion.values[uid] = next
	}
	return conversion, nil
}

// appendOptions appends the options whose names are found in the texts but not in the metadata.
func appendOptions(metadata map[string]interface{}, fieldType db.SLFieldType, texts []string) map[string]interface{} {
	result := make(map[string]interface{}, len(metadata)+1)
	for k, v := range metadata {
		result[k] = v
	}
	options, _ := result["options"].([]interface{})
	options = append([]interface{}{}, options...)

	names := make(map[string]bool)
	for _, option := range options {
		if option, ok := option.(map[string]interface{}); ok {
			if name, ok := option["name"].(string); ok {
				names[name] = true
			}
		}
	}
	for _, text := range texts {
		parts := []string{text}
		if fieldType == db.MultiSelectFieldType {
			parts = multiSelectSeparator.Split(text, -1)
		}
		for _, part := range parts {
			name := strings.TrimSpace(part)
			if name == "" || names[name] {
				continue
			}
			names[name] = true
			options = append(options, map[string]interface{}{
				"uid":   "opt" + randstr.String(6),
				"name":  name,
				"color": float64(len(options)),
			})
		}
	}
	result["options"] = options
	return result
}

func metadataString(field *db.SLField, key string) string {
	metadata, _ := field.Metadata.Data().(map[string]interface{})
	value, _ := metadata[key].(string)
	return value
}

func optionName(field *db.SLField, uid string) string {
	metadata, _ := field.Metadata.Data().(map[string]interface{})
	options, _ := metadata["options"].([]interface{})
	for _, option := range options {
		if option, ok := option.(map[string]interface{}); ok && option["uid"] == uid {
			name, _ := option["name"].(string)
			return name
		}
	}
	return ""
}

func optionUID(field *db.SLField, name string) string {
	metadata, _ := field.Metadata.Data().(map[string]interface{})
	options, _ := metadata["options"].([]interface{})
	for _, option := range options {
		if option, ok := option.(map[string]interface{}); ok && option["name"] == name {
			uid, _ := option["uid"].(string)
			return uid
		}
	}
	return ""
}

// valueToText returns the display text of the value, the same as valueToText of the frontend.
func valueToText(field *db.SLField, value interface{}, boolText func(bool) string) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		switch field.Type {
		case db.SingleSelectFieldType:
			return optionName(field, v)
		case db.DateTimeFieldType:
			metadata, _ := field.Metadata.Data().(map[string]interface{})
			withTime, _ := metadata["with_time"].(bool)
			return formatDate(v, metadataString(field, "format"), withTime)
		default:
			return v
		}
	case float64:
		if field.Type == db.NumberFieldType {
			return formatNumber(v, metadataString(field, "format"))
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return boolText(v)
	case []interface{}:
		names := make([]string, 0, len(v))
		for _, uid := range v {
			if uid, ok := uid.(string); ok {
				if name := optionName(field, uid); name != "" {
					names = append(names, name)
				}
			}
		}
		return strings.Join(names, ", ")
	default:
		return ""
	}
}

var multiSelectSeparator = regexp.MustCompile(`[,，、;；\n]`)

// textToValue parses the text as the value of the field, the same as textToValue of the frontend.
// The select options must exist before parsing, the unknown option names are skipped.
func textToValue(field *db.SLField, text string) interface{} {
	t := strings.TrimSpace(text)
	switch field.Type {
	case db.TextFieldType:
		return lo.Ternary[interface{}](text == "", nil, text)
	case db.NumberFieldType:
		if v, ok := parseNumber(t, metadataString(field, "format")); ok {
			return v
		}
		return nil
	case db.CheckboxFieldType:
		return lo.Contains([]string{"true", "1", "是", "yes", "y", "✓", "√", "checked"}, strings.ToLower(t))
	case db.DateTimeFieldType:
		if v, ok := parseDate(t); ok {
			return v
		}
		return nil
	case db.SingleSelectFieldType:
		if uid := optionUID(field, t); uid != "" {
			return uid
		}
		return nil
	case db.MultiSelectFieldType:
		uids := make([]interface{}, 0)
		for _, part := range multiSelectSeparator.Split(t, -1) {
			if uid := optionUID(field, strings.TrimSpace(part)); uid != "" && !lo.Contains(uids, interface{}(uid)) {
				uids = append(uids, uid)
			}
		}
		return uids
	default:
		return nil
	}
}

var numberFormatPattern = regexp.MustCompile(`^([^0-9]*)(0(?:,000)?)(?:\.(0+))?(%?)$`)

// formatNumber formats the number by the frontend number format, e.g. "0,000.00", "0%" and "¥0,000.00".
func formatNumber(value float64, format string) string {
	if format == "" {
		format = "0"
	}
	m := numberFormatPattern.FindStringSubmatch(format)
	if m == nil {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	prefix, intPart, decimals, percent := m[1], m[2], m[3], m[4]
	if percent != "" {
		value *= 100
	}

	// Round half away from zero as toLocaleString of the frontend, FormatFloat rounds half to even.
	scale := math.Pow10(len(decimals))
	text := strconv.FormatFloat(math.Round(math.Abs(value)*scale)/scale, 'f', len(decimals), 64)
	if strings.Contains(intPart, ",") {
		integer, fraction, _ := strings.Cut(text, ".")
		var b strings.Builder
		for i, c := range integer {
			if i > 0 && (len(integer)-i)%3 == 0 {
				b.WriteByte(',')
			}
			b.WriteRune(c)
		}
		text = b.String() + lo.Ternary(fraction == "", "", "."+fraction)
	}
	return lo.Ternary(value < 0, "-", "") + prefix + text + percent
}

var numberCleaner = strings.NewReplacer(",", "", "¥", "", "$", "", " ", "", "\t", "")

// parseNumber parses the number text which may contain the thousands separator, currency symbol and percent sign.
func parseNumber(text, format string) (float64, bool) {
	t := numberCleaner.Replace(strings.TrimSpace(text))
	if t == "" {
		return 0, false
	}
	isPercent := strings.HasSuffix(t, "%")
	v, err := strconv.ParseFloat(strings.TrimSuffix(t, "%"), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	// "10" is treated as 10% in the percent format.
	if isPercent || strings.HasSuffix(format, "%") {
		v /= 100
	}
	return v, true
}

var dateFormatReplacer = strings.NewReplacer("YYYY", "2006", "MM", "01", "DD", "02")

// formatDate formats the RFC 3339 time in local time zone by the frontend date format, e.g. "YYYY/MM/DD".
func formatDate(value, format string, withTime bool) string {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}
	if format == "" {
		format = "YYYY/MM/DD"
	}
	layout := dateFormatReplacer.Replace(format)
	if withTime {
		layout += " 15:04"
	}
	return t.Local().Format(layout)
}

var (
	dateNormalizer = strings.NewReplacer("年", "/", "月", "/", "日", "", ".", "/", "-", "/")
	dateLayouts    = []string{"2006/1/2 15:04:05", "2006/1/2 15:04", "2006/1/2"}
)

// parseDate parses the date text in local time zone, and returns the time in RFC 3339 format.
func parseDate(text string) (string, bool) {
	if text == "" {
		return "", false
	}
	if t, err := time.Parse(time.RFC3339, text); err == nil {
		return t.UTC().Format("2006-01-02T15:04:05.000Z"), true
	}
	normalized := dateNormalizer.Replace(text)
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, normalized, time.Local); err == nil {
			return t.UTC().Format("2006-01-02T15:04:05.000Z"), true
		}
	}
	return "", false
}
