// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/form"
)

func (schemalessRoute) Recorder(ctx context.Context) error {
	recordUID := ctx.Param("recordUID")
	slRecord, err := db.SLRecords.GetByUID(ctx.Request().Context(), recordUID)
	if err != nil {
		if errors.Is(err, db.ErrSLRecordNotFound) {
			return ctx.ApiError(http.StatusNotFound, "数据表记录不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get sl record by ID")
		return ctx.ApiServerError()
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
		"total": total,
		"data":  slRecords,
	})
}

func (schemalessRoute) GetRecord(ctx context.Context, record *db.SLRecord) error {
	return ctx.ApiSuccess(record)
}

func (schemalessRoute) CreateRecord(ctx context.Context, table *db.SLTable, f form.CreateRecord) error {
	tableID := table.ID

	slRecord, err := db.SLRecords.Create(ctx.Request().Context(), tableID, f.Data)
	if err != nil {
		if errors.Is(err, db.ErrMissingRequiredField) {
			return ctx.ApiError(http.StatusBadRequest, "缺少必填字段")
		}
		if errors.Is(err, db.ErrFieldTypeMismatch) {
			return ctx.ApiError(http.StatusBadRequest, "字段类型不匹配")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create sl record")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(slRecord)
}

func (schemalessRoute) UpdateRecord(ctx context.Context, record *db.SLRecord, f form.UpdateRecord) error {
	recordID := record.ID

	if err := db.SLRecords.Update(ctx.Request().Context(), recordID, f.Data); err != nil {
		if errors.Is(err, db.ErrMissingRequiredField) {
			return ctx.ApiError(http.StatusBadRequest, "缺少必填字段")
		}
		if errors.Is(err, db.ErrFieldTypeMismatch) {
			return ctx.ApiError(http.StatusBadRequest, "字段类型不匹配")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update sl record")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess("更新数据表记录成功")
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
	return ctx.ApiSuccess("删除数据表记录成功")
}
