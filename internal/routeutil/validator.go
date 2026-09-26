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
	fieldSets := lo.KeyBy(fields, func(field *db.SLField) string { return field.UID })

	result := make(map[string]interface{}, len(data))
	for uid, value := range data {
		field, ok := fieldSets[uid]
		if !ok {
			return nil, errors.Wrapf(db.ErrSLFieldNotFound, "field %q", uid)
		}
		if field.Type == db.FormulaFieldType || isEmptyValue(value) {
			continue
		}
		if !checkValue(field, value) {
			return nil, errors.Wrapf(ErrFieldTypeMismatch, "field %q", field.Label)
		}
		if field.Type == db.CheckboxFieldType && value == false {
			continue
		}
		result[uid] = value
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "encode data")
	}
	return jsonBytes, nil
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

func checkValue(field *db.SLField, value interface{}) bool {
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
		return ok && lo.Contains(optionUIDs(field), v)
	case db.MultiSelectFieldType:
		values, ok := value.([]interface{})
		if !ok {
			return false
		}
		options := optionUIDs(field)
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

// optionUIDs returns the option UIDs of the select field, which are stored in metadata as `{"options": [{"uid": "..."}]}`.
func optionUIDs(field *db.SLField) []string {
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
