// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routeutil

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
)

var ErrFieldTypeMismatch = errors.New("field type mismatch")

// Validate checks the cell values keyed by field UID against the table fields, and returns the data to be stored.
// Empty values, unchecked checkboxes and formula values are dropped, as they are not stored.
// It returns db.ErrSLFieldNotFound if any key is not a field of the table, and ErrFieldTypeMismatch if any value mismatches its field type.
func Validate(ctx context.Context, tx *gorm.DB, tableID int64, data map[string]interface{}) (json.RawMessage, error) {
	fields, err := db.NewSLFieldsStore(tx).ListByTableID(ctx, tableID)
	if err != nil {
		return nil, errors.Wrap(err, "list fields")
	}

	result, err := NewRecordValidator(fields).Validate(data)
	if err != nil {
		return nil, err
	}
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "encode data")
	}
	return jsonBytes, nil
}

// RecordValidator checks the record data against the fields of a table.
type RecordValidator struct {
	fields map[string]*db.SLField
}

func NewRecordValidator(fields []*db.SLField) *RecordValidator {
	return &RecordValidator{
		fields: lo.KeyBy(fields, func(field *db.SLField) string { return field.UID }),
	}
}

// Validate returns the data to be stored, see the Validate function for the rules.
func (v *RecordValidator) Validate(data map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(data))
	for uid, value := range data {
		field, ok := v.fields[uid]
		if !ok {
			return nil, errors.Wrapf(db.ErrSLFieldNotFound, "field %q", uid)
		}
		if !IsStoredValue(field, value) {
			continue
		}
		if !CheckValue(field, value) {
			return nil, errors.Wrapf(ErrFieldTypeMismatch, "field %q", field.Label)
		}
		result[uid] = value
	}
	return result, nil
}

// Normalize is the lenient version of Validate, it drops the unknown fields and the mismatched values instead of returning an error.
// dropped reports whether any non-empty value is dropped.
func (v *RecordValidator) Normalize(data map[string]interface{}) (result map[string]interface{}, dropped bool) {
	result = make(map[string]interface{}, len(data))
	for uid, value := range data {
		field, ok := v.fields[uid]
		if !ok {
			dropped = dropped || !isEmptyValue(value)
			continue
		}
		if !IsStoredValue(field, value) {
			continue
		}
		if !CheckValue(field, value) {
			dropped = true
			continue
		}
		result[uid] = value
	}
	return result, dropped
}

// IsStoredValue reports whether the value needs to be stored: empty values, unchecked checkboxes and formula values are not stored.
func IsStoredValue(field *db.SLField, value interface{}) bool {
	if field.Type == db.FormulaFieldType || isEmptyValue(value) {
		return false
	}
	return !(field.Type == db.CheckboxFieldType && value == false)
}

func isEmptyValue(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []interface{}:
		return len(v) == 0
	default:
		return false
	}
}

// CheckValue reports whether the non-empty value matches the field type.
func CheckValue(field *db.SLField, value interface{}) bool {
	switch field.Type {
	case db.TextFieldType:
		_, ok := value.(string)
		return ok
	case db.NumberFieldType:
		v, ok := value.(float64)
		return ok && !math.IsNaN(v) && !math.IsInf(v, 0)
	case db.CheckboxFieldType:
		_, ok := value.(bool)
		return ok
	case db.DateTimeFieldType:
		v, ok := value.(string)
		if !ok {
			return false
		}
		_, err := time.Parse(time.RFC3339, v)
		return err == nil
	case db.SingleSelectFieldType:
		v, ok := value.(string)
		return ok && lo.Contains(OptionUIDs(field), v)
	case db.MultiSelectFieldType:
		values, ok := value.([]interface{})
		if !ok {
			return false
		}
		options := OptionUIDs(field)
		for _, value := range values {
			v, ok := value.(string)
			if !ok || !lo.Contains(options, v) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// OptionUIDs returns the option UIDs of the select field, which are stored in metadata as `{"options": [{"uid": "..."}]}`.
func OptionUIDs(field *db.SLField) []string {
	metadata, _ := field.Metadata.Data().(map[string]interface{})
	options, _ := metadata["options"].([]interface{})

	uids := make([]string, 0, len(options))
	for _, option := range options {
		option, _ := option.(map[string]interface{})
		if uid, ok := option["uid"].(string); ok {
			uids = append(uids, uid)
		}
	}
	return uids
}
