package db

import (
	"database/sql/driver"
	"encoding/json"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
)

// FieldShortcut is the shortcut attached to a field, which generates the cell values of the field from the other fields of the record.
type FieldShortcut struct {
	// ID is the UID of a custom shortcut.
	ID string `json:"id"`
	// Inputs are the configured values keyed by the form item key, a field_select item stores the field UID,
	// and a prompt item stores the text referencing the fields by `{fieldUID}`.
	Inputs map[string]interface{} `json:"inputs"`
	// AutoUpdate regenerates the cell when any referenced field of the record changes.
	AutoUpdate bool `json:"autoUpdate"`
} // @name FieldShortcut

// Value stores the shortcut as JSON, a nil shortcut is stored as NULL.
func (s FieldShortcut) Value() (driver.Value, error) {
	raw, err := json.Marshal(s)
	if err != nil {
		return nil, errors.Wrap(err, "marshal")
	}
	return string(raw), nil
}

// Scan decodes the JSON column.
func (s *FieldShortcut) Scan(src interface{}) error {
	var raw []byte
	switch v := src.(type) {
	case nil:
		*s = FieldShortcut{}
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return errors.Newf("unexpected type %T", src)
	}
	return json.Unmarshal(raw, s)
}

// ShortcutHostFieldTypes are the field types which can have a shortcut, the formula values are not stored so it can not.
var ShortcutHostFieldTypes = []SLFieldType{
	TextFieldType,
	NumberFieldType,
	SingleSelectFieldType,
	MultiSelectFieldType,
	CheckboxFieldType,
	DateTimeFieldType,
}

// CanHostShortcut reports whether the field type can have a shortcut.
func (t SLFieldType) CanHostShortcut() bool {
	return lo.Contains(ShortcutHostFieldTypes, t)
}
