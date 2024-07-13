// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/routeutil"
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

func (s schemalessRoute) CreateRecord(ctx context.Context, t *db.SLTable, tx dbutil.Transactor, f form.CreateRecord) error {
	var slRecord *db.SLRecord
	if err := tx.Transaction(func(tx *gorm.DB) error {
		jsonBytes, err := routeutil.Validate(ctx, t.ID, tx, f.Data)
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
		case errors.Is(err, routeutil.ErrFieldTypeMismatch):
			return ctx.ApiError(http.StatusBadRequest, "字段类型不匹配")
		case errors.Is(err, db.ErrSLFieldNotFound):
			return ctx.ApiError(http.StatusBadRequest, "引用字段不存在")
		case errors.Is(err, db.ErrSLRecordNotFound):
			return ctx.ApiError(http.StatusBadRequest, "引用记录不存在")
		case errors.Is(err, routeutil.ErrExpressionError):
			return ctx.ApiError(http.StatusBadRequest, "表达式错误")
		case errors.Is(err, routeutil.ErrConstraintError):
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

		jsonBytes, err := routeutil.Validate(ctx, record.SLTableID, tx, f.Data)
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
		case errors.Is(err, routeutil.ErrFieldTypeMismatch):
			return ctx.ApiError(http.StatusBadRequest, "字段类型不匹配")
		case errors.Is(err, db.ErrSLFieldNotFound):
			return ctx.ApiError(http.StatusBadRequest, "引用字段不存在")
		case errors.Is(err, db.ErrSLRecordNotFound):
			return ctx.ApiError(http.StatusBadRequest, "引用记录不存在")
		case errors.Is(err, routeutil.ErrExpressionError):
			return ctx.ApiError(http.StatusBadRequest, "表达式错误")
		case errors.Is(err, routeutil.ErrConstraintError):
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
