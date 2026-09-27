package db

import (
	"github.com/samber/lo"
)

// SLFieldType is the type of the field values.
type SLFieldType string

// AllFieldTypes are all the known field types, in the order shown to the users.
var AllFieldTypes = []SLFieldType{
	TextFieldType,
	SingleSelectFieldType,
	MultiSelectFieldType,
	DateTimeFieldType,
	NumberFieldType,
	CheckboxFieldType,
	FormulaFieldType,
}

// Check reports whether the field type is known.
func (t SLFieldType) Check() bool {
	return lo.Contains(AllFieldTypes, t)
}

// IsUnknown reports whether the field type is unknown or explicitly UnknownFieldType.
func (t SLFieldType) IsUnknown() bool {
	return !t.Check() || t == UnknownFieldType
}

const (
	TextFieldType         SLFieldType = "text"
	SingleSelectFieldType SLFieldType = "single_select"
	MultiSelectFieldType  SLFieldType = "multi_select"
	DateTimeFieldType     SLFieldType = "datetime"
	NumberFieldType       SLFieldType = "number"
	CheckboxFieldType     SLFieldType = "checkbox"
	FormulaFieldType      SLFieldType = "formula"
	UnknownFieldType      SLFieldType = "unknown"
)

// Label returns the display name of the field type.
func (t SLFieldType) Label() string {
	switch t {
	case TextFieldType:
		return "文本"
	case SingleSelectFieldType:
		return "单选"
	case MultiSelectFieldType:
		return "多选"
	case DateTimeFieldType:
		return "日期"
	case NumberFieldType:
		return "数字"
	case CheckboxFieldType:
		return "复选框"
	case FormulaFieldType:
		return "公式"
	default:
		return "未知类型"
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
