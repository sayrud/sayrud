package db

import (
	"github.com/samber/lo"
)

// SLFieldType is the type of the field values.
type SLFieldType string // @name SLFieldType

// AllFieldTypes are all the known field types, in the order shown to the users.
var AllFieldTypes = []SLFieldType{
	TextFieldType,
	SingleSelectFieldType,
	MultiSelectFieldType,
	DateTimeFieldType,
	NumberFieldType,
	CheckboxFieldType,
	MemberFieldType,
	AttachmentFieldType,
	FormulaFieldType,
	CreatedByFieldType,
	CreatedAtFieldType,
	UpdatedByFieldType,
	UpdatedAtFieldType,
}

// Check reports whether the field type is known.
func (t SLFieldType) Check() bool {
	return lo.Contains(AllFieldTypes, t)
}

// IsUnknown reports whether the field type is unknown or explicitly UnknownFieldType.
func (t SLFieldType) IsUnknown() bool {
	return !t.Check() || t == UnknownFieldType
}

// IsSystem reports whether the values of the field type are read from the record itself instead of its data, so they are read-only.
func (t SLFieldType) IsSystem() bool {
	return t == CreatedByFieldType || t == CreatedAtFieldType || t == UpdatedByFieldType || t == UpdatedAtFieldType
}

// IsWriter reports whether the values of the field type are the writers of the record.
func (t SLFieldType) IsWriter() bool {
	return t == CreatedByFieldType || t == UpdatedByFieldType
}

const (
	TextFieldType         SLFieldType = "text"
	SingleSelectFieldType SLFieldType = "single_select"
	MultiSelectFieldType  SLFieldType = "multi_select"
	DateTimeFieldType     SLFieldType = "datetime"
	NumberFieldType       SLFieldType = "number"
	CheckboxFieldType     SLFieldType = "checkbox"
	MemberFieldType       SLFieldType = "member"
	AttachmentFieldType   SLFieldType = "attachment"
	FormulaFieldType      SLFieldType = "formula"
	CreatedByFieldType    SLFieldType = "created_by"
	CreatedAtFieldType    SLFieldType = "created_at"
	UpdatedByFieldType    SLFieldType = "updated_by"
	UpdatedAtFieldType    SLFieldType = "updated_at"
	UnknownFieldType      SLFieldType = "unknown"
)

// LabelKey returns the message key of the display name of the field type.
func (t SLFieldType) LabelKey() string {
	switch t {
	case TextFieldType:
		return "field_type::text"
	case SingleSelectFieldType:
		return "field_type::single_select"
	case MultiSelectFieldType:
		return "field_type::multi_select"
	case DateTimeFieldType:
		return "field_type::datetime"
	case NumberFieldType:
		return "field_type::number"
	case CheckboxFieldType:
		return "field_type::checkbox"
	case AttachmentFieldType:
		return "field_type::attachment"
	case FormulaFieldType:
		return "field_type::formula"
	case MemberFieldType:
		return "field_type::member"
	case CreatedByFieldType:
		return "field_type::created_by"
	case CreatedAtFieldType:
		return "field_type::created_at"
	case UpdatedByFieldType:
		return "field_type::updated_by"
	case UpdatedAtFieldType:
		return "field_type::updated_at"
	default:
		return "field_type::unknown"
	}
}

// SLFieldMetadata is the type-specific configuration of a field, it is decoded as a JSON object.
type SLFieldMetadata interface{}

// TextMetadata is the metadata of the text fields.
type TextMetadata struct {
	// Default is the default value of the new records, empty means no default value.
	Default string `json:"default"`
}

// SingleSelectMetadata is the metadata of the single select fields.
type SingleSelectMetadata struct {
	// DefaultOptionUID is the UID of the default option of the new records, empty means no default value.
	DefaultOptionUID string `json:"default"`
}

// MultiSelectMetadata is the metadata of the multiple select fields.
type MultiSelectMetadata struct {
	// DefaultOptionUIDs are the UIDs of the default options of the new records.
	DefaultOptionUIDs []string `json:"default"`
}

// DateTimeMetadata is the metadata of the date time fields.
type DateTimeMetadata struct {
	// Format is the display format of the date, e.g. "YYYY/MM/DD".
	Format string `json:"format"`
	// WithTime reports whether the time is displayed along with the date.
	WithTime bool `json:"with_time"`
	// Default is the default value of the new records, "now" means the creation time, empty means no default value.
	Default string `json:"default"`
}

// NumberMetadata is the metadata of the number fields.
type NumberMetadata struct {
	// Format is the display format of the number, e.g. "0.00", "0,000" and "0%".
	Format string `json:"format"`
	// Default is the default value of the new records.
	Default float64 `json:"default"`
}

// CheckboxMetadata is the metadata of the checkbox fields, which has no configuration.
type CheckboxMetadata struct{}

// FormulaMetadata is the metadata of the formula fields.
type FormulaMetadata struct {
	// Expression is the formula expression referencing the fields by `{fieldUID}`.
	Expression string `json:"exp"`
}

// MemberMetadata is the metadata of the member fields, whose values are arrays of user IDs.
type MemberMetadata struct {
	// Multiple allows more than one member in a cell.
	Multiple bool `json:"multiple"`
}

// SystemTimeMetadata is the metadata of the created_at and updated_at fields, the other system fields have no configuration.
type SystemTimeMetadata struct {
	// Format is the display format of the date, e.g. "YYYY/MM/DD".
	Format string `json:"format"`
	// WithTime reports whether the time is displayed along with the date.
	WithTime bool `json:"with_time"`
}

// MemberMultiple reports whether the member field allows more than one member.
func MemberMultiple(field *SLField) bool {
	metadata, _ := field.Metadata.Data().(map[string]interface{})
	multiple, _ := metadata["multiple"].(bool)
	return multiple
}
