// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
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
			return ctx.ApiError(http.StatusNotFound, "record::not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get sl record by UID")
		return ctx.ApiServerError()
	}

	if slRecord.SLTableID != table.ID {
		return ctx.ApiError(http.StatusNotFound, "record::not_found")
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
			return ctx.ApiError(http.StatusBadRequest, "record::field_not_found")
		case errors.Is(err, db.ErrSLFieldNotQueryable):
			return ctx.ApiError(http.StatusBadRequest, "record::formula_not_queryable")
		case errors.Is(err, db.ErrUnsupportedFilterOperation):
			return ctx.ApiError(http.StatusBadRequest, "record::unsupported_filter")
		case errors.Is(err, db.ErrInvalidFilterValue):
			return ctx.ApiError(http.StatusBadRequest, "record::invalid_filter_value")
		case errors.Is(err, db.ErrInvalidSortOrder):
			return ctx.ApiError(http.StatusBadRequest, "record::invalid_order")
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
		return http.StatusBadRequest, "record::field_type_mismatch", true
	case errors.Is(err, db.ErrSLFieldNotFound):
		return http.StatusBadRequest, "record::referenced_field_not_found", true
	case errors.Is(err, routeutil.ErrInvalidAttachment):
		return http.StatusBadRequest, "attachment::invalid_reference", true
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
func (schemalessRoute) CreateRecord(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable, tx dbutil.Transactor, f form.CreateRecord) error {
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
			return ctx.ApiError(statusCode, msg)
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create sl record")
		return ctx.ApiServerError()
	}
	notifyDirty(ctx, hub, project, table, collab.DirtyScope{Records: []string{slRecord.UID}})
	notifyRecordsChange(ctx, hub, project, table, map[string][]string{slRecord.UID: dataKeys(slRecord.Data)})
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
func (schemalessRoute) BatchCreateRecords(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable, tx dbutil.Transactor, f form.BatchCreateRecords) error {
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
			return ctx.ApiError(statusCode, msg)
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to batch create sl records")
		return ctx.ApiServerError()
	}
	notifyDirty(ctx, hub, project, table, collab.DirtyScope{AllRecords: true})
	notifyRecordsChange(ctx, hub, project, table, lo.SliceToMap(slRecords, func(r *db.SLRecord) (string, []string) { return r.UID, dataKeys(r.Data) }))

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
func (schemalessRoute) UpdateRecord(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable, record *db.SLRecord, tx dbutil.Transactor, f form.UpdateRecord) error {
	var changed []string
	if err := tx.Transaction(func(tx *gorm.DB) error {
		jsonBytes, err := routeutil.Validate(ctx.Request().Context(), tx, record.SLTableID, f.Data)
		if err != nil {
			return errors.Wrap(err, "validate")
		}
		changed = changedKeys(record.Data, jsonBytes)

		if err := db.NewSLRecordsStore(tx).Update(ctx.Request().Context(), record.ID, jsonBytes); err != nil {
			return errors.Wrap(err, "update sl record")
		}
		return nil
	}); err != nil {
		if statusCode, msg, ok := recordErrorResponse(err); ok {
			return ctx.ApiError(statusCode, msg)
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update sl record")
		return ctx.ApiServerError()
	}
	notifyDirty(ctx, hub, project, table, collab.DirtyScope{Records: []string{record.UID}})
	notifyRecordsChange(ctx, hub, project, table, map[string][]string{record.UID: changed})
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
func (schemalessRoute) DeleteRecord(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable, record *db.SLRecord, tx dbutil.Transactor) error {
	if err := tx.Transaction(func(tx *gorm.DB) error {
		if err := db.NewSLShortcutJobsStore(tx).DeleteByRecords(ctx.Request().Context(), table.ID, []string{record.UID}); err != nil {
			return errors.Wrap(err, "delete shortcut jobs")
		}
		return db.NewSLRecordsStore(tx).DeleteByID(ctx.Request().Context(), record.ID)
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete sl record")
		return ctx.ApiServerError()
	}
	notifyDirty(ctx, hub, project, table, collab.DirtyScope{Records: []string{record.UID}})
	return ctx.Status(http.StatusNoContent)
}

// notifyRecordsChange tells the field shortcuts the changed fields of the records, keyed by record UID.
func notifyRecordsChange(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable, records map[string][]string) {
	hub.NotifyChange(ctx.Request().Context(), project, table, &collab.Change{Records: records})
}

// dataKeys returns the field UIDs of the stored record data.
func dataKeys(raw []byte) []string {
	data := map[string]json.RawMessage{}
	_ = json.Unmarshal(raw, &data)
	return lo.Keys(data)
}

// changedKeys returns the field UIDs whose values differ between the record data.
func changedKeys(before, after []byte) []string {
	a, b := map[string]json.RawMessage{}, map[string]json.RawMessage{}
	_ = json.Unmarshal(before, &a)
	_ = json.Unmarshal(after, &b)

	var keys []string
	for _, k := range lo.Union(lo.Keys(a), lo.Keys(b)) {
		if !bytes.Equal(compactJSON(a[k]), compactJSON(b[k])) {
			keys = append(keys, k)
		}
	}
	return lo.Ternary(keys == nil, []string{}, keys)
}

func compactJSON(raw json.RawMessage) []byte {
	var buf bytes.Buffer
	if len(raw) == 0 || json.Compact(&buf, raw) != nil {
		return raw
	}
	return buf.Bytes()
}
