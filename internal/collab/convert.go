package collab

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
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
// directory names the members and matches them when converting from or to a member or writer field, it may be nil otherwise.
func convertField(old *db.SLField, newType db.SLFieldType, metadata map[string]interface{}, records []*db.SLRecord, boolText func(bool) string, directory *routeutil.Directory) (*fieldConversion, error) {
	conversion := &fieldConversion{metadata: metadata, values: make(map[string]interface{}, len(records))}

	oldValues := make(map[string]interface{}, len(records))
	for _, record := range records {
		data := map[string]interface{}{}
		if err := json.Unmarshal(record.Data, &data); err != nil {
			return nil, errors.Wrapf(err, "decode record %q", record.UID)
		}
		oldValues[record.UID] = recordValue(old, record, data)
	}
	if err := preloadNames(directory, old, lo.Values(oldValues)); err != nil {
		return nil, err
	}
	texts := make(map[string]string, len(records))
	for uid, value := range oldValues {
		texts[uid] = valueToText(old, value, boolText, directory)
	}

	// The formula values are calculated by the clients, and the system values are read from the records.
	if newType == db.FormulaFieldType || newType.IsSystem() {
		for uid := range oldValues {
			conversion.values[uid] = nil
		}
		return conversion, nil
	}

	isSelect := func(t db.SLFieldType) bool { return t == db.SingleSelectFieldType || t == db.MultiSelectFieldType }
	selectToSelect := isSelect(old.Type) && isSelect(newType)
	if isSelect(newType) && !selectToSelect && metadata["optionsReference"] == nil {
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
		case old.Type == db.AttachmentFieldType && newType == db.AttachmentFieldType:
			next = raw
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
		case old.Type == db.MemberFieldType && newType == db.MemberFieldType:
			values, _ := raw.([]interface{})
			if len(values) > 1 && !db.MemberMultiple(newField) {
				values = values[:1]
			}
			next = values
		case old.Type.IsWriter() && newType == db.MemberFieldType:
			if writer, _ := raw.(db.SLRecordWriter); writer.UserID != 0 {
				next = []interface{}{float64(writer.UserID)}
			}
		case (old.Type == db.CreatedAtFieldType || old.Type == db.UpdatedAtFieldType) && newType == db.DateTimeFieldType:
			next = raw
		default:
			next = textToValue(newField, texts[uid], directory)
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

// recordValue returns the value of the field in the record: the system values are read from the record, the others from its data.
func recordValue(field *db.SLField, record *db.SLRecord, data map[string]interface{}) interface{} {
	switch field.Type {
	case db.CreatedAtFieldType:
		return record.CreatedAt.UTC().Format(storedTimeLayout)
	case db.UpdatedAtFieldType:
		return record.UpdatedAt.UTC().Format(storedTimeLayout)
	case db.CreatedByFieldType:
		return record.CreatedBy()
	case db.UpdatedByFieldType:
		return record.UpdatedBy()
	default:
		return data[field.UID]
	}
}

// preloadNames loads the users and the workflows of the member and writer values in batch.
func preloadNames(directory *routeutil.Directory, field *db.SLField, values []interface{}) error {
	var userIDs, workflowIDs []int64
	for _, value := range values {
		switch v := value.(type) {
		case db.SLRecordWriter:
			userIDs, workflowIDs = append(userIDs, v.UserID), append(workflowIDs, v.WorkflowID)
		case []interface{}:
			if field.Type == db.MemberFieldType {
				userIDs = append(userIDs, routeutil.MemberIDs(v)...)
			}
		}
	}
	return directory.Preload(userIDs, workflowIDs)
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

func optionNames(field *db.SLField) map[string]string {
	metadata, _ := field.Metadata.Data().(map[string]interface{})
	options, _ := metadata["options"].([]interface{})
	names := make(map[string]string, len(options))
	for _, raw := range options {
		option, _ := raw.(map[string]interface{})
		uid, _ := option["uid"].(string)
		name, _ := option["name"].(string)
		names[uid] = name
	}
	return names
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
func valueToText(field *db.SLField, value interface{}, boolText func(bool) string, directory *routeutil.Directory) string {
	switch v := value.(type) {
	case nil:
		return ""
	case db.SLRecordWriter:
		return directory.WriterName(v)
	case string:
		switch field.Type {
		case db.SingleSelectFieldType:
			return optionName(field, v)
		case db.DateTimeFieldType, db.CreatedAtFieldType, db.UpdatedAtFieldType:
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
		if field.Type == db.MemberFieldType {
			return directory.MemberNames(v)
		}
		names := make([]string, 0, len(v))
		if field.Type == db.AttachmentFieldType {
			for _, value := range v {
				if attachment, ok := value.(map[string]interface{}); ok {
					if name, ok := attachment["name"].(string); ok && name != "" {
						names = append(names, name)
					}
				}
			}
			return strings.Join(names, ", ")
		}
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
// The select options must exist before parsing, the unknown option names are skipped, and so are the texts matching no member.
func textToValue(field *db.SLField, text string, directory *routeutil.Directory) interface{} {
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
	case db.MemberFieldType:
		if ids := directory.ParseMembers(t, db.MemberMultiple(field)); len(ids) > 0 {
			return ids
		}
		return nil
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
		layout += " 15:04:05"
	}
	return t.Local().Format(layout)
}

var (
	dateNormalizer = strings.NewReplacer("年", "/", "月", "/", "日", "", ".", "/", "-", "/")
	dateLayouts    = []string{"2006/1/2 15:04:05", "2006/1/2 15:04", "2006/1/2"}
)

// parseDate parses the date text in local time zone, and returns the time in RFC 3339 format.
func parseDate(text string) (string, bool) {
	return parseDateIn(text, time.Local)
}

// parseDateIn parses the date text without a time zone in the location, and returns the time in RFC 3339 format.
func parseDateIn(text string, loc *time.Location) (string, bool) {
	if text == "" {
		return "", false
	}
	if t, err := time.Parse(time.RFC3339, text); err == nil {
		return t.UTC().Format(storedTimeLayout), true
	}
	normalized := dateNormalizer.Replace(text)
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, normalized, loc); err == nil {
			return t.UTC().Format(storedTimeLayout), true
		}
	}
	return "", false
}

// storedTimeLayout is the format of the stored datetime values, the same as toISOString of the frontend.
const storedTimeLayout = "2006-01-02T15:04:05.000Z"
