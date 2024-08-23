// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/json"
	"net/http"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/apibuilder"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/jsvm"
)

var View viewRoute

type viewRoute struct{}

func (viewRoute) List(ctx context.Context, project *db.Project) error {
	projectID := project.ID

	views, total, err := db.Views.List(ctx.Request().Context(), projectID, db.ListViewOptions{
		Pagination: dbutil.Pagination{
			Page:     ctx.QueryInt("page", 1),
			PageSize: ctx.QueryInt("pageSize", 10),
		},
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list views")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(map[string]interface{}{
		"views": views,
		"total": total,
	})
}

func (viewRoute) Create(ctx context.Context, project *db.Project, f form.CreateView) error {
	tableUID := f.TableUID
	name := f.Name
	inputFieldUIDs := f.FieldUIDs
	filter := f.Filter
	order := f.Order

	if err := filter.ValidateConfig(ctx.Request().Context()); err != nil {
		return ctx.ApiError(http.StatusBadRequest, "过滤条件配置错误")
	}
	filterJSON, err := json.Marshal(filter)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to marshal filter")
		return ctx.ApiServerError()
	}

	table, err := db.SLTables.GetByUID(ctx.Request().Context(), tableUID)
	if err != nil {
		if errors.Is(err, db.ErrSLTableNotFound) {
			return ctx.ApiError(http.StatusNotFound, "表格不存在")
		}

		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get table")
		return ctx.ApiServerError()
	}

	if table.ProjectID != project.ID {
		return ctx.ApiError(http.StatusNotFound, "表格不存在")
	}

	tableFields, err := db.SLFields.GetByTableID(ctx.Request().Context(), table.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get table fields")
		return ctx.ApiServerError()
	}

	tableFieldsSet := lo.SliceToMap(tableFields, func(field *db.SLField) (string, *db.SLField) {
		return field.UID, field
	})

	fieldUIDs := make(pq.StringArray, 0, len(inputFieldUIDs))
	for _, fieldUID := range inputFieldUIDs {
		field, ok := tableFieldsSet[fieldUID]
		if ok {
			fieldUIDs = append(fieldUIDs, field.UID)
		}
	}

	orderJSON, err := json.Marshal(order)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to marshal order")
		return ctx.ApiServerError()
	}

	view, err := db.Views.Create(ctx.Request().Context(), db.CreateViewOptions{
		ProjectID:   project.ID,
		SLTableID:   table.ID,
		Name:        name,
		SLFieldUIDs: fieldUIDs,
		Filter:      filterJSON,
		Order:       orderJSON,
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create view")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(view)
}

func (viewRoute) Viewer(ctx context.Context, project *db.Project) error {
	viewUID := ctx.Param("viewUID")

	view, err := db.Views.GetByUID(ctx.Request().Context(), viewUID)
	if err != nil {
		if errors.Is(err, db.ErrViewNotFound) {
			return ctx.ApiError(http.StatusNotFound, "视图不存在")
		}

		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get view")
		return ctx.ApiServerError()
	}

	if view.ProjectID != project.ID {
		return ctx.ApiError(http.StatusNotFound, "API 不存在")
	}

	ctx.Map(view)
	return nil
}

func (viewRoute) Get(ctx context.Context, view *db.View) error {
	return ctx.ApiSuccess(view)
}

func (viewRoute) Update(ctx context.Context, view *db.View, f form.UpdateView) error {
	name := f.Name
	inputFieldUIDs := f.FieldUIDs
	filter := f.Filter
	order := f.Order

	if err := filter.ValidateConfig(ctx.Request().Context()); err != nil {
		return ctx.ApiError(http.StatusBadRequest, "过滤条件配置错误")
	}
	filterJSON, err := json.Marshal(filter)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to marshal filter")
		return ctx.ApiServerError()
	}

	tableFields, err := db.SLFields.GetByTableID(ctx.Request().Context(), view.SLTableID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get table fields")
		return ctx.ApiServerError()
	}

	tableFieldsSet := lo.SliceToMap(tableFields, func(field *db.SLField) (string, *db.SLField) {
		return field.UID, field
	})

	fieldUIDs := make(pq.StringArray, 0, len(inputFieldUIDs))
	for _, fieldUID := range inputFieldUIDs {
		field, ok := tableFieldsSet[fieldUID]
		if ok {
			fieldUIDs = append(fieldUIDs, field.UID)
		}
	}

	orderJSON, err := json.Marshal(order)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to marshal order")
		return ctx.ApiServerError()
	}

	if err := db.Views.Update(ctx.Request().Context(), view.ID, db.UpdateViewOptions{
		Name:        name,
		SLFieldUIDs: fieldUIDs,
		Filter:      filterJSON,
		Order:       orderJSON,
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update view")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

func (viewRoute) Delete(ctx context.Context, view *db.View) error {
	if err := db.Views.DeleteByID(ctx.Request().Context(), view.ID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete view")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

func (viewRoute) Query(ctx context.Context, view *db.View) error {
	page := ctx.QueryInt("page", 1)
	pageSize := ctx.QueryInt("pageSize", 10)
	limit, offset := dbutil.LimitOffset(page, pageSize)

	// Initialize JavaScript VM.
	vm, err := jsvm.NewVM(jsvm.NewVMOptions{})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create JS VM")
		return ctx.ApiServerError()
	}

	tableFields, err := db.SLFields.GetByTableID(ctx.Request().Context(), view.SLTable.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get table fields")
		return ctx.ApiServerError()
	}
	// We return all the fields records.
	fields := make(map[string]string)
	fieldSets := make(map[string]*db.SLField, len(tableFields))
	for _, fieldUID := range tableFields {
		fieldUID := fieldUID

		fields[fieldUID.Name] = ""
		fieldSets[fieldUID.UID] = fieldUID
	}

	filter, err := apibuilder.ParseOperator(view.Filter)
	if err != nil {
		return ctx.ApiError(http.StatusBadRequest, "过滤条件配置错误")
	}
	filterExpression, err := filter.ToClauseExpression(vm)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to convert filter to expression")
		return ctx.ApiServerError()
	}

	orders, err := apibuilder.ParseOrders(view.Order)
	if err != nil {
		return ctx.ApiError(http.StatusBadRequest, "排序条件配置错误")
	}
	orderFields := make([]string, 0, len(orders))
	for _, order := range orders {
		orderField := fieldSets[order.FieldUID]
		// TODO asc desc
		orderFields = append(orderFields, orderField.Name)
	}

	records, total, err := db.SLTables.QueryList(ctx.Request().Context(), &view.SLTable, db.QueryListSLTableOptions{
		Fields: fields,
		Filter: filterExpression,
		Orders: orderFields,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to query list")
		return ctx.ApiServerError()
	}

	return ctx.ApiSuccess(map[string]interface{}{
		"records": records,
		"total":   total,
	})
}
