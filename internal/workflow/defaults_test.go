package workflow

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
)

func TestApplyFieldDefaults(t *testing.T) {
	options := []interface{}{map[string]interface{}{"uid": "opt1", "name": "First"}}
	field := func(uid string, kind db.SLFieldType, value interface{}) *db.SLField {
		return &db.SLField{UID: uid, Type: kind, Metadata: datatypes.NewJSONType[db.SLFieldMetadata](map[string]interface{}{"default": value, "options": options})}
	}
	automatic := field("automatic", db.TextFieldType, "generated")
	automatic.Shortcut = &db.FieldShortcut{ID: "generate"}
	info := newTableInfo(&db.SLTable{}, []*db.SLField{
		field("date", db.DateTimeFieldType, "now"),
		field("otherDate", db.DateTimeFieldType, "now"),
		field("text", db.TextFieldType, "Default"),
		field("number", db.NumberFieldType, float64(0)),
		field("single", db.SingleSelectFieldType, "opt1"),
		field("multi", db.MultiSelectFieldType, []interface{}{"opt1"}),
		field("explicit", db.TextFieldType, "Default"),
		field("null", db.DateTimeFieldType, "now"),
		field("empty", db.MultiSelectFieldType, []interface{}{"opt1"}),
		field("blank", db.TextFieldType, "Default"),
		field("noDate", db.DateTimeFieldType, ""),
		field("noNumber", db.NumberFieldType, nil),
		field("noText", db.TextFieldType, ""),
		field("noOptions", db.MultiSelectFieldType, []interface{}{}),
		field("staleOption", db.SingleSelectFieldType, "deleted"),
		field("formula", db.FormulaFieldType, "formula"),
		field("attachment", db.AttachmentFieldType, "file"),
		field("checkbox", db.CheckboxFieldType, true),
		field("unknown", db.UnknownFieldType, "unknown"),
		automatic,
	})
	values := map[string]interface{}{"explicit": "Mapped", "null": nil, "empty": []interface{}{}, "blank": ""}
	before := time.Now().UTC().Truncate(time.Second)
	info.applyDefaults(values)
	date, ok := values["date"].(string)
	require.True(t, ok)
	created, err := time.Parse(time.RFC3339, date)
	require.NoError(t, err)
	assert.False(t, created.Before(before))
	assert.False(t, created.After(time.Now().UTC()))
	assert.Equal(t, time.UTC, created.Location())

	assert.Equal(t, map[string]interface{}{
		"date": date, "otherDate": date, "text": "Default", "number": float64(0),
		"single": "opt1", "multi": []interface{}{"opt1"},
		"explicit": "Mapped", "null": nil, "empty": []interface{}{}, "blank": "",
	}, values)
}

func TestWorkflowCreateDateDefault(t *testing.T) {
	for _, sharedBatch := range []bool{true, false} {
		t.Run(map[bool]string{true: "synchronous", false: "background"}[sharedBatch], func(t *testing.T) {
			f := newLeadFixture(t)
			ctx := context.Background()
			date, err := db.NewSLFieldsStore(f.db).Create(ctx, db.CreateSLFieldOptions{
				SLTableID: f.table.ID, Label: "Created date", Type: db.DateTimeFieldType,
				Metadata: map[string]interface{}{"default": "now", "with_time": false},
			})
			require.NoError(t, err)
			x := &execution{
				ctx: ctx, db: f.db, hub: f.hub, project: f.project, tables: map[string]*tableInfo{},
				scope: Scope{"run": map[string]interface{}{"time": "2020-01-01T00:00:00Z"}},
			}
			write := func(step Step) map[string]interface{} {
				t.Helper()
				var out interface{}
				apply := func() error {
					var err error
					out, err = x.write(&step)
					return err
				}
				if sharedBatch {
					err = f.hub.Batch(ctx, f.project, []*db.SLTable{f.table}, collab.BatchOptions{}, func(b *collab.Batch) error {
						x.batch = b
						defer func() { x.batch = nil }()
						return apply()
					})
				} else {
					err = apply()
				}
				require.NoError(t, err)
				return out.(map[string]interface{})["record"].(map[string]interface{})
			}

			before := time.Now().UTC().Truncate(time.Second)
			record := write(Step{ID: "create", Type: StepCreate, Config: &WriteConfig{TableUID: f.table.UID}})
			fields := record["fields"].(map[string]interface{})
			storedDate, ok := fields[date.UID].(string)
			require.True(t, ok, "unmapped creation date should receive its default")
			created, err := time.Parse(time.RFC3339, storedDate)
			require.NoError(t, err)
			assert.False(t, created.Before(before))
			assert.False(t, created.After(time.Now().UTC()))

			stored, err := db.NewSLRecordsStore(f.db).GetByUID(ctx, record["uid"].(string))
			require.NoError(t, err)
			var data map[string]interface{}
			require.NoError(t, json.Unmarshal(stored.Data, &data))
			assert.Equal(t, storedDate, data[date.UID])

			explicit := write(Step{ID: "explicit", Type: StepCreate, Config: &WriteConfig{
				TableUID: f.table.UID, Fields: []FieldValue{{FieldUID: date.UID, Value: Value{Kind: ValueConst, Const: json.RawMessage(`"2024-02-03T04:05:06Z"`)}}},
			}})
			assert.Equal(t, "2024-02-03T04:05:06Z", explicit["fields"].(map[string]interface{})[date.UID])

			empty := write(Step{ID: "empty", Type: StepCreate, Config: &WriteConfig{
				TableUID: f.table.UID, Fields: []FieldValue{{FieldUID: date.UID, Value: Value{Kind: ValueConst, Const: json.RawMessage(`null`)}}},
			}})
			assert.NotContains(t, empty["fields"], date.UID)

			updated := write(Step{ID: "update", Type: StepUpdate, Config: &WriteConfig{
				TableUID: f.table.UID, RecordUID: &Value{Kind: ValueConst, Const: marshalJSON(empty["uid"])},
				Fields: []FieldValue{{FieldUID: f.name.UID, Value: Value{Kind: ValueConst, Const: json.RawMessage(`"Updated"`)}}},
			}})
			assert.NotContains(t, updated["fields"], date.UID, "updates must not fill creation defaults")
		})
	}
}
