package dto

import (
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// Field is the schemaless table field returned by the management API.
type Field struct {
	// UID is the identifier of the field, which is also the key of its values in the record data.
	UID string `json:"uid"`
	// TableUID is the table of the field.
	TableUID string `json:"tableUID"`
	// Label is the title of the field.
	Label string `json:"label"`
	// Type is the type of the field.
	Type string `json:"type" enums:"text,single_select,multi_select,datetime,number,checkbox,attachment,formula"`
	// Metadata is the type-specific configuration, e.g. select options or the formula expression.
	Metadata map[string]interface{} `json:"metadata"`
	// Position is the order of the field in the table, starting from 0. The first field is the primary field.
	Position int `json:"position"`
	// Shortcut generates the cell values from the other fields, it is absent if the values are edited by the users.
	Shortcut *db.FieldShortcut `json:"shortcut,omitempty"`
	// CreatedAt is the time the field was created.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt is the time the field was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
} // @name SLField

func ToField(table *db.SLTable, field *db.SLField) *Field {
	metadata, _ := field.Metadata.Data().(map[string]interface{})
	if metadata == nil {
		metadata = map[string]interface{}{}
	}

	return &Field{
		UID:       field.UID,
		TableUID:  table.UID,
		Label:     field.Label,
		Type:      string(field.Type),
		Metadata:  metadata,
		Position:  field.Position,
		Shortcut:  field.Shortcut,
		CreatedAt: field.CreatedAt,
		UpdatedAt: field.UpdatedAt,
	}
}

func ToFields(table *db.SLTable, fields []*db.SLField) []*Field {
	result := make([]*Field, 0, len(fields))
	for _, field := range fields {
		result = append(result, ToField(table, field))
	}
	return result
}
