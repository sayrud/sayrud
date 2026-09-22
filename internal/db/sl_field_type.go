package db

import (
	"github.com/samber/lo"
)

type SLFieldType string

var AllFieldTypes = []SLFieldType{
	TextFieldType,
	SingleSelectFieldType,
	MultiSelectFieldType,
	DateTimeFieldType,
	NumberFieldType,
	CheckboxFieldType,
	FormulaFieldType,
}

func (t SLFieldType) Check() bool {
	return lo.Contains(AllFieldTypes, t)
}

func (t SLFieldType) IsUnknown() bool {
	return t.Check() || t == UnknownFieldType
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

type SLFieldMetadata interface{}

type TextMetadata struct {
	Default string `json:"default"`
}

type SingleSelectMetadata struct {
	DefaultOptionUID string `json:"default"`
}

type MultiSelectMetadata struct {
	DefaultOptionUIDs []string `json:"default"`
}

type DateTimeMetadata struct {
	Format   string `json:"format"`
	WithTime bool   `json:"with_time"`
	Default  string `json:"default"`
}

type NumberMetadata struct {
	Format  string  `json:"format"`
	Default float64 `json:"default"`
}

type CheckboxMetadata struct{}

type FormulaMetadata struct {
	Expression string `json:"exp"`
}
