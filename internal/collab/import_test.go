package collab

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/importer"
)

func importTestField(uid string, fieldType db.SLFieldType, metadata map[string]interface{}) *db.SLField {
	return &db.SLField{
		UID:      uid,
		Label:    uid,
		Type:     fieldType,
		Metadata: datatypes.NewJSONType[db.SLFieldMetadata](metadata),
	}
}

func textCells(texts ...string) []*importer.Cell {
	cells := make([]*importer.Cell, len(texts))
	for i, t := range texts {
		if t != "" {
			cells[i] = &importer.Cell{Text: t}
		}
	}
	return cells
}

func TestImportSchema(t *testing.T) {
	status := importTestField("fldStatus01", db.SingleSelectFieldType, map[string]interface{}{
		"options": []interface{}{
			map[string]interface{}{
				"uid":   "optOpen01",
				"name":  "Open",
				"color": 0.0,
			},
		},
	})
	linked := importTestField("fldLinked01", db.SingleSelectFieldType, map[string]interface{}{
		"options": []interface{}{},
		"optionsReference": map[string]interface{}{
			"tableUID": "tblOther",
			"fieldUID": "fldOther",
		},
	})
	formula := importTestField("fldFormula1", db.FormulaFieldType, map[string]interface{}{"exp": "1"})
	rows := [][]*importer.Cell{textCells("Open", "A", "1", "red, blue"), textCells("Closed", "B", "2", "blue")}

	actions, targets, err := importSchema([]*db.SLField{status, linked, formula}, ImportOptions{
		Rows: rows,
		Columns: []ImportColumn{
			{
				Index:    0,
				FieldUID: "fldStatus01",
			},
			{
				Index:    1,
				FieldUID: "fldLinked01",
			},
			{
				Index: 3,
				Label: " Tags ",
				Type:  db.MultiSelectFieldType,
			},
		},
		ViewName: "Grid",
	})
	require.NoError(t, err)
	require.Len(t, actions, 3)

	assert.Equal(t, ActionSetField, actions[0].Action)
	options := actions[0].Field.Metadata["options"].([]interface{})
	require.Len(t, options, 2)
	assert.Equal(t, "Closed", options[1].(map[string]interface{})["name"])

	assert.Equal(t, ActionAddField, actions[1].Action)
	assert.Equal(t, "Tags", *actions[1].Field.Label)
	assert.Len(t, actions[1].Field.Metadata["options"], 2)
	assert.Equal(t, ActionAddView, actions[2].Action)
	assert.Equal(t, []string{"fldStatus01", "fldLinked01", actions[1].FieldUID}, targets)

	_, _, err = importSchema([]*db.SLField{formula}, ImportOptions{
		Columns: []ImportColumn{{
			Index:    0,
			FieldUID: "fldFormula1",
		}},
	})
	assert.Equal(t, "import::field_not_found", err.(*OperationError).Key)
	_, _, err = importSchema(nil, ImportOptions{
		Columns: []ImportColumn{{
			Index: 0,
			Label: "File",
			Type:  db.AttachmentFieldType,
		}},
	})
	assert.Equal(t, "collab::invalid_field_type", err.(*OperationError).Key)
}

func TestImportValue(t *testing.T) {
	shanghai := time.FixedZone("CST", 8*3600)
	number := importTestField("fldNumber01", db.NumberFieldType, map[string]interface{}{"format": "0%"})
	date := importTestField("fldDate0001", db.DateTimeFieldType, map[string]interface{}{})
	checkbox := importTestField("fldCheck001", db.CheckboxFieldType, map[string]interface{}{})
	tags := importTestField("fldTags0001", db.MultiSelectFieldType, map[string]interface{}{
		"options": []interface{}{
			map[string]interface{}{
				"uid":  "optRed0001",
				"name": "red",
			},
		},
	})

	assert.Equal(t, 0.15, importValue(number, &importer.Cell{Text: "15%"}, shanghai))
	assert.Equal(t, 15.0, importValue(number, &importer.Cell{Text: "15"}, shanghai))
	percentCell := &importer.Cell{
		Text:   "50%",
		Kind:   importer.KindNumber,
		Number: 0.5,
	}
	assert.Equal(t, 0.5, importValue(number, percentCell, shanghai))
	assert.Nil(t, importValue(number, &importer.Cell{Text: "n/a"}, shanghai))

	excelDate := &importer.Cell{
		Kind: importer.KindDate,
		Time: time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC),
	}
	assert.Equal(t, "2024-01-15T01:30:00.000Z", importValue(date, excelDate, shanghai))
	assert.Equal(t, "2024-01-14T16:00:00.000Z", importValue(date, &importer.Cell{Text: "2024年1月15日"}, shanghai))

	assert.Equal(t, true, importValue(checkbox, &importer.Cell{Text: "是"}, shanghai))
	uncheckedCell := &importer.Cell{
		Text: "FALSE",
		Kind: importer.KindBool,
	}
	assert.Nil(t, importValue(checkbox, uncheckedCell, shanghai))

	assert.Equal(t, []interface{}{"optRed0001"}, importValue(tags, &importer.Cell{Text: "red, unknown"}, shanghai))
	assert.Nil(t, importValue(tags, &importer.Cell{Text: "unknown"}, shanghai))
	assert.Nil(t, importValue(tags, nil, shanghai))
}
