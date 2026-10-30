package workflow

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/db"
)

func TestRecordOutputFormulas(t *testing.T) {
	field := func(uid string, kind db.SLFieldType, metadata map[string]interface{}) *db.SLField {
		return &db.SLField{UID: uid, Type: kind, Metadata: datatypes.NewJSONType[db.SLFieldMetadata](metadata)}
	}
	formula := func(uid, exp string) *db.SLField {
		return field(uid, db.FormulaFieldType, map[string]interface{}{"exp": exp})
	}
	options := []interface{}{map[string]interface{}{"uid": "optA", "name": "A"}, map[string]interface{}{"uid": "optB", "name": "B"}}
	info := newTableInfo(&db.SLTable{}, []*db.SLField{
		field("name", db.TextFieldType, nil),
		field("price", db.NumberFieldType, nil),
		field("qty", db.NumberFieldType, nil),
		field("level", db.SingleSelectFieldType, map[string]interface{}{"options": options}),
		field("tags", db.MultiSelectFieldType, map[string]interface{}{"options": options}),
		field("due", db.DateTimeFieldType, nil),
		field("done", db.CheckboxFieldType, nil),
		field("files", db.AttachmentFieldType, nil),
		field("missingValue", db.TextFieldType, nil),
		formula("total", "{price} * {qty}"),
		formula("nested", "{total} + 1"),
		formula("label", `{name} & "-" & {level} & "-" & {tags}`),
		formula("fileNames", "{files}"),
		formula("dueNext", "{due} + 1"),
		formula("dueYear", "YEAR({due})"),
		formula("status", `IF({done}, "done", "todo")`),
		formula("blankName", "ISBLANK({missingValue})"),
		formula("loopA", "{loopB}"),
		formula("loopB", "{loopA}"),
		formula("gone", "{fldGone}"),
		formula("divZero", "1 / 0"),
		formula("syntax", "1 +"),
		formula("empty", " "),
	})
	data, _ := json.Marshal(map[string]interface{}{
		"name": "Ann", "price": 2.5, "qty": float64(4), "level": "optB", "tags": []interface{}{"optA", "optB"},
		"due":   "2026-12-31T20:00:00Z",
		"files": []interface{}{map[string]interface{}{"name": "a.png"}, map[string]interface{}{"name": "b.pdf"}},
	})
	record := &db.SLRecord{UID: "rec1", Data: data}

	fields := recordOutput(info, record, time.UTC)["fields"].(map[string]interface{})
	assert.Equal(t, 10.0, fields["total"])
	assert.Equal(t, 11.0, fields["nested"], "formulas can reference formulas")
	assert.Equal(t, "Ann-B-A, B", fields["label"], "options are referenced by name")
	assert.Equal(t, "a.png, b.pdf", fields["fileNames"])
	assert.Equal(t, "2027-01-01T20:00:00Z", fields["dueNext"])
	assert.Equal(t, 2026.0, fields["dueYear"])
	assert.Equal(t, "todo", fields["status"], "an empty checkbox is false")
	assert.Equal(t, true, fields["blankName"])
	for _, uid := range []string{"loopA", "loopB", "gone", "divZero", "syntax", "empty"} {
		assert.NotContains(t, fields, uid)
	}
	assert.Equal(t, "B", fields["level"], "other fields are kept")

	shanghai, err := time.LoadLocation("Asia/Shanghai")
	assert.NoError(t, err)
	fields = recordOutput(info, record, shanghai)["fields"].(map[string]interface{})
	assert.Equal(t, 2027.0, fields["dueYear"], "dates are handled in the time zone of the workflow")
}
