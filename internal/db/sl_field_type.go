package db

import (
	"reflect"
)

const (
	IntFieldType       SLFieldType = "int"
	TextFieldType      SLFieldType = "text"
	BoolFieldType      SLFieldType = "bool"
	FloatFieldType     SLFieldType = "float"
	TimestampFieldType SLFieldType = "timestamp"
	DateFieldType      SLFieldType = "date"
)

var FieldTypes = []SLFieldType{
	IntFieldType,
	TextFieldType,
	BoolFieldType,
	FloatFieldType,
	TimestampFieldType,
	DateFieldType,
}

func (t SLFieldType) Label() string {
	switch t {
	case IntFieldType:
		return "整数"
	case TextFieldType:
		return "文本"
	case BoolFieldType:
		return "布尔值"
	case FloatFieldType:
		return "浮点数"
	case TimestampFieldType:
		return "时间戳"
	case DateFieldType:
		return "日期"
	default:
		return "未知类型"
	}
}

var internalKindMatch = map[reflect.Kind]SLFieldType{
	reflect.Bool:    BoolFieldType,
	reflect.Int:     IntFieldType,
	reflect.Int8:    IntFieldType,
	reflect.Int16:   IntFieldType,
	reflect.Int32:   IntFieldType,
	reflect.Int64:   IntFieldType,
	reflect.Uint:    IntFieldType,
	reflect.Uint8:   IntFieldType,
	reflect.Uint16:  IntFieldType,
	reflect.Uint32:  IntFieldType,
	reflect.Uint64:  IntFieldType,
	reflect.Uintptr: IntFieldType,
	reflect.Float32: FloatFieldType,
	reflect.Float64: FloatFieldType,
	reflect.String:  TextFieldType,
}

type SLFieldType string

func (t SLFieldType) Check() bool {
	switch t {
	case IntFieldType, BoolFieldType, TextFieldType, FloatFieldType, TimestampFieldType, DateFieldType:
		return true
	default:
		return false
	}
}
