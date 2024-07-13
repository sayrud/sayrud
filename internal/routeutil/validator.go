// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routeutil

import (
	"encoding/json"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/jsvm"
)

var ErrFieldTypeMismatch = errors.New("field type mismatch")
var ErrExpressionError = errors.New("expression error")
var ErrConstraintError = errors.New("constraint error")

func Validate(ctx context.Context, vm *goja.Runtime, tableID uint, tx *gorm.DB, data map[string]interface{}) (json.RawMessage, error) {
	slTablesStore := db.NewSLTablesStore(tx)
	slFieldsStore := db.NewSLFieldsStore(tx)
	slRecordsStore := db.NewSLRecordsStore(tx)

	table, err := db.SLTables.GetByID(ctx.Request().Context(), tableID)
	if err != nil {
		return nil, errors.Wrap(err, "get table by id")
	}

	// Get table fields.
	slFields, err := slFieldsStore.GetByTableID(ctx.Request().Context(), tableID)
	if err != nil {
		return nil, errors.Wrap(err, "get fields by table id")
	}

	var incrementIndexFlag bool
	// Validate the input data fields.
	for _, field := range slFields {
		field := field

		if field.Type == db.GeneratedFieldType {
			continue
		}

		val, ok := data[field.UID]
		if !ok {
			// If the filed has default value, set it for this missing field.
			defaultValue, ok := field.Options[db.OptionsDefaultValue]
			if ok {
				defaultValueExpression := cast.ToString(defaultValue)
				defaultValue, err := vm.RunString(defaultValueExpression)
				if err != nil {
					return nil, errors.Wrap(err, "run default value expression")
				}
				data[field.UID] = defaultValue.Export()
			}
			// Set auto increment field value,
			// also set the incrementIndexFlag to ture, to make sure the increment index will be increased after the operation.
			if field.Type == db.IntFieldType && field.IsIncrementIndex() {
				data[field.UID] = table.IncrementIndex + 1
				incrementIndexFlag = true
			}
		} else {
			// Check if the field type match with the value.
			if !field.CheckValue(val) {
				return nil, ErrFieldTypeMismatch
			}
		}

		if field.Type == db.ReferenceFieldType {
			referenceFieldUID := field.ReferenceFieldUID()
			// Make sure the reference field is in the current table.
			referenceField, err := slFieldsStore.GetByUID(ctx.Request().Context(), referenceFieldUID)
			if err != nil {
				return nil, errors.Wrap(err, "get reference field")
			}
			if referenceField.SLTableID != tableID {
				return nil, db.ErrSLFieldNotFound
			}

			// Check the reference field record exists.
			recordUID := cast.ToString(val)
			record, err := slRecordsStore.GetByUID(ctx.Request().Context(), recordUID)
			if err != nil {
				return nil, errors.Wrap(err, "get reference record")
			}
			if record.SLTableID != referenceField.SLTableID {
				return nil, db.ErrSLRecordNotFound
			}

			// Check the constraint.
			recordData := make(map[string]interface{})
			if err := json.Unmarshal(record.Data, &recordData); err != nil {
				return nil, errors.Wrap(err, "unmarshal reference record data")
			}

			this := make(map[string]interface{})
			that := make(map[string]interface{})
			for _, f := range slFields {
				this[f.Name] = data[f.UID]
				that[f.Name] = recordData[f.UID]
			}
			this["uid"] = data["uid"]
			that["uid"] = record.UID

			// Check the reference field constraint.
			constraint := field.Constraint()
			vm, err := jsvm.NewVM(jsvm.NewVMOptions{
				This: this,
				That: that,
			})
			if err != nil {
				return nil, errors.Wrap(err, "new vm")
			}
			result, err := vm.RunString(constraint)
			if err != nil {
				return nil, errors.Wrap(err, "run constraint")
			}
			if !result.ToBoolean() {
				return nil, ErrConstraintError
			}
		}
	}

	if incrementIndexFlag {
		if err := slTablesStore.SetIncrementIndex(ctx.Request().Context(), tableID, table.IncrementIndex+1); err != nil {
			return nil, errors.Wrap(err, "set increment index")
		}
	}

	// Ok, now we can encode the data.
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, errors.Wrap(err, "encode data")
	}
	return jsonBytes, nil
}
