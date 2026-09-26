// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

// Recorder maps the record of the recordUID path parameter as *db.SLRecord, it must belong to the current table.
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

// ListRecords
// @Summary List records
// @Description List the records of the table, the newest first.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param limit query int false "Maximum number of records, defaults to 20"
// @Param offset query int false "Number of records to skip"
// @Success 200 {object} dto.ListRecordsResp
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID listRecords
// @Router /projects/{projectUID}/tables/{tableUID}/records [get]
func (schemalessRoute) ListRecords(ctx context.Context, table *db.SLTable) error {
	slRecords, total, err := db.SLRecords.Query(ctx.Request().Context(), table.ID, db.QuerySLRecordsOptions{
		Limit:  ctx.QueryInt("limit"),
		Offset: ctx.QueryInt("offset"),
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list sl records")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ListRecordsResp{
		Records: dto.ToRecords(table, slRecords),
		Total:   total,
	})
}

// QueryRecords
// @Summary Query records
// @Description Filter, group and sort the records of the table. Formula fields cannot be queried.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param data body form.QueryRecords true "Query options"
// @Success 200 {object} dto.ListRecordsResp
// @Failure 400 {string} string "Invalid field, operation, value or order"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID queryRecords
// @Router /projects/{projectUID}/tables/{tableUID}/records/query [post]
func (schemalessRoute) QueryRecords(ctx context.Context, table *db.SLTable, f form.QueryRecords) error {
	slRecords, total, err := db.SLRecords.Query(ctx.Request().Context(), table.ID, db.QuerySLRecordsOptions{
		Filter: lo.Map(f.Filter, func(item form.QueryRecordsFilter, _ int) db.QuerySLRecordsFilter {
			return db.QuerySLRecordsFilter{
				FieldUID:  item.FieldUID,
				Operation: db.FilterOperation(item.Operation),
				Value:     item.Value,
			}
		}),
		Order: lo.Map(f.Order, func(item form.QueryRecordsSort, _ int) db.QuerySLRecordsSort {
			return db.QuerySLRecordsSort{
				FieldUID: item.FieldUID,
				Order:    item.Order,
			}
		}),
		Group: lo.Map(f.Group, func(item form.QueryRecordsGroup, _ int) db.QuerySLRecordsGroup {
			return db.QuerySLRecordsGroup{
				FieldUID: item.FieldUID,
			}
		}),
		Limit:  f.Limit,
		Offset: f.Offset,
	})
	if err != nil {
		switch {
		case errors.Is(err, db.ErrSLFieldNotFound):
			return ctx.ApiError(http.StatusBadRequest, "字段不存在")
		case errors.Is(err, db.ErrSLFieldNotQueryable):
			return ctx.ApiError(http.StatusBadRequest, "公式字段不可用于查询")
		case errors.Is(err, db.ErrUnsupportedFilterOperation):
			return ctx.ApiError(http.StatusBadRequest, "不支持的筛选条件")
		case errors.Is(err, db.ErrInvalidFilterValue):
			return ctx.ApiError(http.StatusBadRequest, "筛选值格式错误")
		case errors.Is(err, db.ErrInvalidSortOrder):
			return ctx.ApiError(http.StatusBadRequest, "排序方式错误")
		default:
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to query sl records")
			return ctx.ApiServerError()
		}
	}

	return ctx.ApiSuccess(dto.ListRecordsResp{
		Records: dto.ToRecords(table, slRecords),
		Total:   total,
	})
}

// GetRecord
// @Summary Get a record
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param recordUID path string true "Record UID"
// @Success 200 {object} dto.Record
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table or record not found"
// @Failure 500 {string} string "Internal server error"
// @ID getRecord
// @Router /projects/{projectUID}/tables/{tableUID}/records/{recordUID} [get]
func (schemalessRoute) GetRecord(ctx context.Context, table *db.SLTable, record *db.SLRecord) error {
	return ctx.ApiSuccess(dto.ToRecord(table, record))
}

// recordErrorResponse returns the response of the record data validation errors, ok is false if the error is unexpected.
func recordErrorResponse(err error) (statusCode int, msg string, ok bool) {
	switch {
	case errors.Is(err, routeutil.ErrFieldTypeMismatch):
		return http.StatusBadRequest, "字段类型不匹配", true
	case errors.Is(err, db.ErrSLFieldNotFound):
		return http.StatusBadRequest, "引用字段不存在", true
	default:
		return 0, "", false
	}
}

// CreateRecord
// @Summary Create a record
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param data body form.CreateRecord true "Record to create"
// @Success 200 {object} dto.Record
// @Failure 400 {string} string "Field not found or value mismatches the field type"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID createRecord
// @Router /projects/{projectUID}/tables/{tableUID}/records [post]
func (schemalessRoute) CreateRecord(ctx context.Context, table *db.SLTable, tx dbutil.Transactor, f form.CreateRecord) error {
	var slRecord *db.SLRecord
	if err := tx.Transaction(func(tx *gorm.DB) error {
		jsonBytes, err := routeutil.Validate(ctx.Request().Context(), tx, table.ID, f.Data)
		if err != nil {
			return errors.Wrap(err, "validate")
		}

		slRecord, err = db.NewSLRecordsStore(tx).Create(ctx.Request().Context(), table.ID, jsonBytes)
		if err != nil {
			return errors.Wrap(err, "create sl record")
		}
		return nil
	}); err != nil {
		if statusCode, msg, ok := recordErrorResponse(err); ok {
			return ctx.ApiError(statusCode, "%s", msg)
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create sl record")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToRecord(table, slRecord))
}

// BatchCreateRecords
// @Summary Create records in batch
// @Description Create multiple records atomically, none is created if any data is invalid.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param data body form.BatchCreateRecords true "Records to create"
// @Success 200 {array} dto.Record
// @Failure 400 {string} string "Field not found or value mismatches the field type"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID batchCreateRecords
// @Router /projects/{projectUID}/tables/{tableUID}/records/batch [post]
func (schemalessRoute) BatchCreateRecords(ctx context.Context, table *db.SLTable, tx dbutil.Transactor, f form.BatchCreateRecords) error {
	var slRecords []*db.SLRecord
	if err := tx.Transaction(func(tx *gorm.DB) error {
		dataList := make([]json.RawMessage, 0, len(f.Data))
		for _, data := range f.Data {
			jsonBytes, err := routeutil.Validate(ctx.Request().Context(), tx, table.ID, data)
			if err != nil {
				return errors.Wrap(err, "validate")
			}
			dataList = append(dataList, jsonBytes)
		}

		var err error
		slRecords, err = db.NewSLRecordsStore(tx).Import(ctx.Request().Context(), table.ID, db.ImportSLRecordsOptions{
			Data: dataList,
		})
		if err != nil {
			return errors.Wrap(err, "import sl records")
		}
		return nil
	}); err != nil {
		if statusCode, msg, ok := recordErrorResponse(err); ok {
			return ctx.ApiError(statusCode, "%s", msg)
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to batch create sl records")
		return ctx.ApiServerError()
	}

	return ctx.ApiSuccess(dto.ToRecords(table, slRecords))
}

// UpdateRecord
// @Summary Update a record
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param recordUID path string true "Record UID"
// @Param data body form.UpdateRecord true "Record data"
// @Success 204 "No Content"
// @Failure 400 {string} string "Field not found or value mismatches the field type"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table or record not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateRecord
// @Router /projects/{projectUID}/tables/{tableUID}/records/{recordUID} [put]
func (schemalessRoute) UpdateRecord(ctx context.Context, record *db.SLRecord, tx dbutil.Transactor, f form.UpdateRecord) error {
	if err := tx.Transaction(func(tx *gorm.DB) error {
		jsonBytes, err := routeutil.Validate(ctx.Request().Context(), tx, record.SLTableID, f.Data)
		if err != nil {
			return errors.Wrap(err, "validate")
		}

		if err := db.NewSLRecordsStore(tx).Update(ctx.Request().Context(), record.ID, jsonBytes); err != nil {
			return errors.Wrap(err, "update sl record")
		}
		return nil
	}); err != nil {
		if statusCode, msg, ok := recordErrorResponse(err); ok {
			return ctx.ApiError(statusCode, "%s", msg)
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update sl record")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// DeleteRecord
// @Summary Delete a record
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param recordUID path string true "Record UID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table or record not found"
// @Failure 500 {string} string "Internal server error"
// @ID deleteRecord
// @Router /projects/{projectUID}/tables/{tableUID}/records/{recordUID} [delete]
func (schemalessRoute) DeleteRecord(ctx context.Context, record *db.SLRecord) error {
	if err := db.SLRecords.DeleteByID(ctx.Request().Context(), record.ID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete sl record")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}
