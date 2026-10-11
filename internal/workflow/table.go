package workflow

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"

	"github.com/wuhan005/sayrud/internal/db"
)

// tableInfo is a table and its fields cached during a run.
type tableInfo struct {
	table  *db.SLTable
	fields []*db.SLField
	byUID  map[string]*db.SLField
}

func newTableInfo(table *db.SLTable, fields []*db.SLField) *tableInfo {
	return &tableInfo{table: table, fields: fields, byUID: lo.KeyBy(fields, func(f *db.SLField) string { return f.UID })}
}

// FieldValueError means a value can not be written to a field and results in a 422 response.
type FieldValueError struct {
	Field   string
	Message string
}

func (e *FieldValueError) Error() string {
	return fmt.Sprintf("field %q: %s", e.Field, e.Message)
}

type selectOption struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
}

func fieldOptions(field *db.SLField) []selectOption {
	metadata, _ := field.Metadata.Data().(map[string]interface{})
	raw, _ := json.Marshal(metadata["options"])
	var options []selectOption
	_ = json.Unmarshal(raw, &options)
	return options
}

// WritableField reports whether the field can be written by the API: formula, attachment and shortcut-generated fields are not writable.
func WritableField(field *db.SLField) bool {
	return field.Type != db.FormulaFieldType && field.Type != db.AttachmentFieldType && field.Shortcut == nil && !field.Type.IsUnknown()
}

// toCellValue converts a value to the field's stored value; single and multiple selects accept option names or UIDs.
func toCellValue(field *db.SLField, v interface{}) (interface{}, error) {
	if v == nil {
		return nil, nil
	}
	mismatch := func(expect string) error {
		return &FieldValueError{Field: field.Label, Message: "expects " + expect}
	}
	switch field.Type {
	case db.TextFieldType:
		return Text(v), nil
	case db.NumberFieldType:
		switch t := v.(type) {
		case float64:
			return t, nil
		case string:
			if strings.TrimSpace(t) == "" {
				return nil, nil
			}
			f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
			if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				return nil, mismatch("a number")
			}
			return f, nil
		}
		return nil, mismatch("a number")
	case db.CheckboxFieldType:
		switch t := v.(type) {
		case bool:
			return t, nil
		case string:
			if t == "true" || t == "false" {
				return t == "true", nil
			}
		}
		return nil, mismatch("a boolean")
	case db.DateTimeFieldType:
		s, ok := v.(string)
		if !ok {
			return nil, mismatch("a date time")
		}
		if s == "" {
			return nil, nil
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.Format(time.RFC3339), nil
		}
		if t, err := time.Parse(time.DateOnly, s); err == nil {
			return t.Format(time.RFC3339), nil
		}
		return nil, mismatch("an RFC 3339 date time")
	case db.SingleSelectFieldType:
		s, ok := v.(string)
		if !ok {
			return nil, mismatch("an option name")
		}
		if s == "" {
			return nil, nil
		}
		uid, ok := optionUID(field, s)
		if !ok {
			return nil, &FieldValueError{Field: field.Label, Message: fmt.Sprintf("has no option %q", s)}
		}
		return uid, nil
	case db.MultiSelectFieldType:
		var names []interface{}
		switch t := v.(type) {
		case string:
			if t == "" {
				return nil, nil
			}
			names = []interface{}{t}
		case []interface{}:
			names = t
		default:
			return nil, mismatch("option names")
		}
		uids := make([]interface{}, 0, len(names))
		for _, n := range names {
			s, ok := n.(string)
			if !ok {
				return nil, mismatch("option names")
			}
			uid, ok := optionUID(field, s)
			if !ok {
				return nil, &FieldValueError{Field: field.Label, Message: fmt.Sprintf("has no option %q", s)}
			}
			uids = append(uids, uid)
		}
		return lo.Uniq(uids), nil
	}
	return nil, &FieldValueError{Field: field.Label, Message: "is not writable"}
}

func optionUID(field *db.SLField, nameOrUID string) (string, bool) {
	options := fieldOptions(field)
	for _, o := range options {
		if o.UID == nameOrUID {
			return o.UID, true
		}
	}
	for _, o := range options {
		if o.Name == nameOrUID {
			return o.UID, true
		}
	}
	return "", false
}

// displayValue converts a stored value to the output value: option UIDs become names.
func displayValue(field *db.SLField, v interface{}) interface{} {
	switch field.Type {
	case db.SingleSelectFieldType:
		s, _ := v.(string)
		for _, o := range fieldOptions(field) {
			if o.UID == s {
				return o.Name
			}
		}
		return nil
	case db.MultiSelectFieldType:
		list, _ := v.([]interface{})
		options := fieldOptions(field)
		out := make([]interface{}, 0, len(list))
		for _, item := range list {
			s, _ := item.(string)
			if o, ok := lo.Find(options, func(o selectOption) bool { return o.UID == s }); ok {
				out = append(out, o.Name)
			}
		}
		return out
	}
	return v
}

// recordOutput returns the shape of a record in variables: uid, createdAt, updatedAt, and fields keyed by field UID.
func recordOutput(info *tableInfo, record *db.SLRecord) map[string]interface{} {
	data := map[string]interface{}{}
	_ = json.Unmarshal(record.Data, &data)
	fields := map[string]interface{}{}
	for _, field := range info.fields {
		if v, ok := data[field.UID]; ok {
			fields[field.UID] = displayValue(field, v)
		}
	}
	return map[string]interface{}{
		"uid":       record.UID,
		"createdAt": record.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt": record.UpdatedAt.UTC().Format(time.RFC3339),
		"fields":    fields,
	}
}

// Default and maximum number of records returned by a query.
const (
	DefaultQueryLimit = 20
	MaxQueryLimit     = 100
)

// buildFilters converts parameterized conditions to storage filters; conditions whose value is nil are ignored.
func buildFilters(info *tableInfo, scope Scope, filters []Filter) ([]db.QuerySLRecordsFilter, error) {
	out := make([]db.QuerySLRecordsFilter, 0, len(filters))
	for i := range filters {
		f := &filters[i]
		field, ok := info.byUID[f.FieldUID]
		if !ok {
			return nil, errors.Wrapf(db.ErrSLFieldNotFound, "field %q", f.FieldUID)
		}
		if f.Op == "empty" || f.Op == "not_empty" {
			out = append(out, db.QuerySLRecordsFilter{FieldUID: field.UID, Operation: db.FilterOperation(f.Op)})
			continue
		}
		v, err := scope.Resolve(f.Value)
		if err != nil {
			return nil, err
		}
		if v == nil {
			continue
		}
		value, err := filterValue(field, f.Op, v)
		if err != nil {
			return nil, err
		}
		out = append(out, db.QuerySLRecordsFilter{FieldUID: field.UID, Operation: db.FilterOperation(f.Op), Value: value})
	}
	return out, nil
}

func filterValue(field *db.SLField, op string, v interface{}) (string, error) {
	scalar := func(item interface{}) (interface{}, error) {
		switch field.Type {
		case db.SingleSelectFieldType, db.MultiSelectFieldType:
			s := Text(item)
			if uid, ok := optionUID(field, s); ok {
				return uid, nil
			}
			return s, nil
		case db.NumberFieldType, db.CheckboxFieldType, db.DateTimeFieldType:
			return toCellValue(field, item)
		}
		return Text(item), nil
	}
	if op == "in" || op == "nin" {
		list, ok := v.([]interface{})
		if !ok {
			list = []interface{}{v}
		}
		items := make([]interface{}, 0, len(list))
		for _, item := range list {
			s, err := scalar(item)
			if err != nil {
				return "", err
			}
			items = append(items, s)
		}
		raw, _ := json.Marshal(items)
		return string(raw), nil
	}
	s, err := scalar(v)
	if err != nil {
		return "", err
	}
	return Text(s), nil
}
