package collab

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/db"
)

func TestFormatNumber(t *testing.T) {
	for _, tc := range []struct {
		value  float64
		format string
		want   string
	}{
		{1234.5, "0", "1235"},
		{1234.5, "0.00", "1234.50"},
		{1234567.891, "0,000.00", "1,234,567.89"},
		{0.125, "0.00%", "12.50%"},
		{-1000, "¥0,000.00", "-¥1,000.00"},
		{3, "", "3"},
	} {
		assert.Equal(t, tc.want, formatNumber(tc.value, tc.format), "%v %q", tc.value, tc.format)
	}
}

func TestParseNumber(t *testing.T) {
	v, ok := parseNumber("¥1,234.50", "0.00")
	assert.True(t, ok)
	assert.Equal(t, 1234.5, v)

	v, ok = parseNumber("10", "0%")
	assert.True(t, ok)
	assert.Equal(t, 0.1, v)

	_, ok = parseNumber("abc", "0")
	assert.False(t, ok)
}

func TestParseDate(t *testing.T) {
	for _, text := range []string{"2026/01/30", "2026-01-30", "2026年1月30日", "2026.1.30"} {
		got, ok := parseDate(text)
		require.True(t, ok, text)
		assert.Equal(t, "2026-01-30", formatDate(got, "YYYY-MM-DD", false), text)
	}
	_, ok := parseDate("not a date")
	assert.False(t, ok)
}

func TestAttachmentConversions(t *testing.T) {
	field := newTestField("fldFiles", db.AttachmentFieldType, map[string]interface{}{})
	values := []interface{}{
		map[string]interface{}{"uid": "filOne", "name": "photo.png", "size": 12.0, "contentType": "image/png", "url": "/_/projects/prjOne/attachments/filOne"},
		map[string]interface{}{"uid": "filTwo", "name": "report.pdf", "size": 24.0, "contentType": "application/pdf", "url": "/_/projects/prjOne/attachments/filTwo"},
	}
	raw, err := json.Marshal(map[string]interface{}{field.UID: values})
	require.NoError(t, err)
	records := []*db.SLRecord{{UID: "recOne", Data: raw}}
	conversion, err := convertField(field, db.TextFieldType, map[string]interface{}{}, records, testBoolText)
	require.NoError(t, err)
	require.Equal(t, "photo.png, report.pdf", conversion.values["recOne"])
	conversion, err = convertField(field, db.AttachmentFieldType, map[string]interface{}{}, records, testBoolText)
	require.NoError(t, err)
	require.Equal(t, values, conversion.values["recOne"])
}

func newTestField(uid string, fieldType db.SLFieldType, metadata map[string]interface{}) *db.SLField {
	return &db.SLField{UID: uid, Type: fieldType, Metadata: datatypes.NewJSONType[db.SLFieldMetadata](metadata)}
}

func newTestRecord(t *testing.T, uid string, data map[string]interface{}) *db.SLRecord {
	raw, err := json.Marshal(data)
	require.NoError(t, err)
	return &db.SLRecord{UID: uid, Data: raw}
}

func TestConvertField_TextToSingleSelect(t *testing.T) {
	field := newTestField("fldAAAAAAA", db.TextFieldType, map[string]interface{}{})
	records := []*db.SLRecord{
		newTestRecord(t, "rec1", map[string]interface{}{"fldAAAAAAA": "Todo"}),
		newTestRecord(t, "rec2", map[string]interface{}{"fldAAAAAAA": "Done"}),
		newTestRecord(t, "rec3", map[string]interface{}{"fldAAAAAAA": "Todo"}),
		newTestRecord(t, "rec4", map[string]interface{}{}),
	}

	conversion, err := convertField(field, db.SingleSelectFieldType, map[string]interface{}{"default": ""}, records, testBoolText)
	require.NoError(t, err)

	options := conversion.metadata["options"].([]interface{})
	require.Len(t, options, 2)
	uids := map[string]string{}
	for _, option := range options {
		option := option.(map[string]interface{})
		uids[option["name"].(string)] = option["uid"].(string)
	}
	assert.Equal(t, uids["Todo"], conversion.values["rec1"])
	assert.Equal(t, uids["Done"], conversion.values["rec2"])
	assert.Equal(t, uids["Todo"], conversion.values["rec3"])
	assert.NotContains(t, conversion.values, "rec4")
}

func TestConvertField_NumberAndCheckbox(t *testing.T) {
	number := newTestField("fldBBBBBBB", db.NumberFieldType, map[string]interface{}{"format": "0.00"})
	records := []*db.SLRecord{
		newTestRecord(t, "rec1", map[string]interface{}{"fldBBBBBBB": 2.0}),
		newTestRecord(t, "rec2", map[string]interface{}{"fldBBBBBBB": 0.0}),
	}

	conversion, err := convertField(number, db.CheckboxFieldType, map[string]interface{}{}, records, testBoolText)
	require.NoError(t, err)
	assert.Equal(t, true, conversion.values["rec1"])
	// Unchecked checkboxes are not stored.
	assert.Nil(t, conversion.values["rec2"])

	conversion, err = convertField(number, db.TextFieldType, map[string]interface{}{}, records, testBoolText)
	require.NoError(t, err)
	assert.Equal(t, "2", conversion.values["rec1"])
}

func TestConvertField_MultiToSingleSelect(t *testing.T) {
	options := []interface{}{
		map[string]interface{}{"uid": "optA", "name": "A"},
		map[string]interface{}{"uid": "optB", "name": "B"},
	}
	field := newTestField("fldCCCCCCC", db.MultiSelectFieldType, map[string]interface{}{"options": options})
	records := []*db.SLRecord{
		newTestRecord(t, "rec1", map[string]interface{}{"fldCCCCCCC": []interface{}{"optB", "optA"}}),
	}

	conversion, err := convertField(field, db.SingleSelectFieldType, map[string]interface{}{"options": options}, records, testBoolText)
	require.NoError(t, err)
	assert.Equal(t, "optB", conversion.values["rec1"])
}

func testBoolText(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}

func TestConvertField_CheckboxToText(t *testing.T) {
	checkbox := newTestField("fldCCCCCCC", db.CheckboxFieldType, map[string]interface{}{})
	records := []*db.SLRecord{
		newTestRecord(t, "rec1", map[string]interface{}{"fldCCCCCCC": true}),
		newTestRecord(t, "rec2", map[string]interface{}{"fldCCCCCCC": false}),
	}

	conversion, err := convertField(checkbox, db.TextFieldType, map[string]interface{}{}, records, testBoolText)
	require.NoError(t, err)
	assert.Equal(t, "Yes", conversion.values["rec1"])
	assert.Equal(t, "No", conversion.values["rec2"])
}
