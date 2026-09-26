package dto

import (
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// Field is the schemaless table field returned by the management API.
type Field struct {
	UID      string `json:"uid"`
	TableUID string `json:"tableUID"`
	Label    string `json:"label"`
	Type     string `json:"type" enums:"text,single_select,multi_select,datetime,number,checkbox,formula"`
	// Metadata is the type-specific configuration, e.g. select options or the formula expression.
	Metadata  map[string]interface{} `json:"metadata"`
	Position  int                    `json:"position"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
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
