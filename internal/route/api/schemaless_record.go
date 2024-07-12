// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/jsvm"
)

func (schemalessRoute) Recorder(ctx context.Context, table *db.SLTable) error {
	recordUID := ctx.Param("recordUID")
	slRecord, err := db.SLRecords.GetByUID(ctx.Request().Context(), recordUID)
	if err != nil {
		if errors.Is(err, db.ErrSLRecordNotFound) {
			return ctx.ApiError(http.StatusNotFound, "数据表记录不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get sl record by UID")
		return ctx.ApiServerError()
	}

	if slRecord.SLTableID != table.ID {
		return ctx.ApiError(http.StatusNotFound, "数据表记录不存在")
	}

	ctx.Map(slRecord)
	return nil
}

func (schemalessRoute) ListRecords(ctx context.Context, table *db.SLTable) error {
	slRecords, total, err := db.SLRecords.GetView(ctx.Request().Context(), table, db.GetViewOptions{
		Page:     ctx.QueryInt("page"),
		PageSize: ctx.QueryInt("pageSize"),
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list sl records")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(map[string]interface{}{
		"total":   total,
		"records": slRecords,
	})
}

func (schemalessRoute) QueryRecords(ctx context.Context, table *db.SLTable) error {
	slRecords, err := db.SLRecords.Query(ctx.Request().Context(), table.ID, db.QuerySLRecordsOptions{
		FieldUID:   ctx.Query("fieldUID"),
		FieldValue: ctx.Query("fieldValue"),
	})
	if err != nil {
		if errors.Is(err, db.ErrSLFieldNotFound) {
			return ctx.ApiError(http.StatusBadRequest, "字段不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to query sl records")
		return ctx.ApiServerError()
	}

	return ctx.ApiSuccess(slRecords)
}

func (schemalessRoute) GetRecord(ctx context.Context, record *db.SLRecord) error {
	return ctx.ApiSuccess(record)
}

var ErrMissingRequiredField = errors.New("missing required field")
var ErrFieldTypeMismatch = errors.New("field type mismatch")
var ErrExpressionError = errors.New("expression error")
var ErrConstraintError = errors.New("constraint error")

type dbFuncOptions struct {
	tableID   uint
	recordID  uint
	tx        *gorm.DB
	jsonBytes json.RawMessage
}

func (schemalessRoute) validate(ctx context.Context, tableID uint, tx *gorm.DB, data map[string]interface{}) (json.RawMessage, error) {
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
			// Check if the field is a required field.
			if field.IsRequired() {
				return nil, ErrMissingRequiredField
			}
			// If the filed has default value, set it for this missing field.
			defaultValue, ok := field.Options[db.OptionsDefaultValue]
			if ok {
				data[field.UID] = defaultValue
			}
			// Set auto increment field value,
			// also set the incrementIndexFlag to ture, to make sure the increment index will be increased after the operation.
			if field.Type == db.IntFieldType && field.IsIncrementIndex() {
				data[field.UID] = table.IncrementIndex + 1
				incrementIndexFlag = true
			}
		} else {
			if val == nil {
				if field.IsRequired() {
					return nil, ErrMissingRequiredField
				}
			} else {
				// Check if the field type match with the value.
				if !field.CheckValue(val) {
					return nil, ErrFieldTypeMismatch
				}
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

func (s schemalessRoute) CreateRecord(ctx context.Context, t *db.SLTable, tx dbutil.Transactor, f form.CreateRecord) error {
	var slRecord *db.SLRecord
	if err := tx.Transaction(func(tx *gorm.DB) error {
		jsonBytes, err := s.validate(ctx, t.ID, tx, f.Data)
		if err != nil {
			return errors.Wrap(err, "validate")
		}

		slRecordsStore := db.NewSLRecordsStore(tx)
		slRecord, err = slRecordsStore.Create(ctx.Request().Context(), t.ID, jsonBytes)
		if err != nil {
			return errors.Wrap(err, "create sl records")
		}
		return nil

	}); err != nil {
		switch {
		case errors.Is(err, ErrMissingRequiredField):
			return ctx.ApiError(http.StatusBadRequest, "缺少必填字段")
		case errors.Is(err, ErrFieldTypeMismatch):
			return ctx.ApiError(http.StatusBadRequest, "字段类型不匹配")
		case errors.Is(err, db.ErrSLFieldNotFound):
			return ctx.ApiError(http.StatusBadRequest, "引用字段不存在")
		case errors.Is(err, db.ErrSLRecordNotFound):
			return ctx.ApiError(http.StatusBadRequest, "引用记录不存在")
		case errors.Is(err, ErrExpressionError):
			return ctx.ApiError(http.StatusBadRequest, "表达式错误")
		case errors.Is(err, ErrConstraintError):
			return ctx.ApiError(http.StatusBadRequest, "约束条件错误")
		default:
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create sl record")
			return ctx.ApiServerError()
		}
	}
	return ctx.ApiSuccess(slRecord)
}

func (s schemalessRoute) UpdateRecord(ctx context.Context, record *db.SLRecord, tx dbutil.Transactor, f form.UpdateRecord) error {
	if err := tx.Transaction(func(tx *gorm.DB) error {
		// Current record UID will be used in the constraint expression.
		f.Data["uid"] = record.UID

		jsonBytes, err := s.validate(ctx, record.SLTableID, tx, f.Data)
		if err != nil {
			return errors.Wrap(err, "validate")
		}

		slRecordsStore := db.NewSLRecordsStore(tx)
		if err := slRecordsStore.Update(ctx.Request().Context(), record.ID, jsonBytes); err != nil {
			return errors.Wrap(err, "update sl records")
		}
		return nil
	}); err != nil {
		switch {
		case errors.Is(err, ErrMissingRequiredField):
			return ctx.ApiError(http.StatusBadRequest, "缺少必填字段")
		case errors.Is(err, ErrFieldTypeMismatch):
			return ctx.ApiError(http.StatusBadRequest, "字段类型不匹配")
		case errors.Is(err, db.ErrSLFieldNotFound):
			return ctx.ApiError(http.StatusBadRequest, "引用字段不存在")
		case errors.Is(err, db.ErrSLRecordNotFound):
			return ctx.ApiError(http.StatusBadRequest, "引用记录不存在")
		case errors.Is(err, ErrExpressionError):
			return ctx.ApiError(http.StatusBadRequest, "表达式错误")
		case errors.Is(err, ErrConstraintError):
			return ctx.ApiError(http.StatusBadRequest, "约束条件错误")
		default:
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create sl record")
			return ctx.ApiServerError()
		}
	}
	return ctx.Status(http.StatusNoContent)
}

func (schemalessRoute) DeleteRecord(ctx context.Context, record *db.SLRecord) error {
	recordID := record.ID

	if err := db.SLRecords.DeleteByID(ctx.Request().Context(), recordID); err != nil {
		if errors.Is(err, db.ErrSLRecordNotFound) {
			return ctx.ApiError(http.StatusNotFound, "数据表记录不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete sl record")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}
