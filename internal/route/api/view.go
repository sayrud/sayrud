// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/form"
)

var View viewRoute

type viewRoute struct{}

func (viewRoute) List(ctx context.Context, table *db.Project) error {
	tableID := table.ID

	views, err := db.Views.GetByTableID(ctx.Request().Context(), tableID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list views")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(views)
}

func (viewRoute) Create(ctx context.Context, table *db.Project, f form.CreateView) error {
	fieldUIDs := f.FieldUIDs
	filter := f.Filter

	if err := filter.ValidateConfig(ctx.Request().Context()); err != nil {
		return ctx.ApiError(http.StatusBadRequest, "过滤条件配置错误")
	}
	filterJSON, err := json.Marshal(filter)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to marshal filter")
		return ctx.ApiServerError()
	}

	tableFields, err := db.SLFields.GetByTableID(ctx.Request().Context(), table.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get table fields")
		return ctx.ApiServerError()
	}

	tableFieldsSet := lo.SliceToMap(tableFields, func(field *db.SLField) (string, *db.SLField) {
		return field.UID, field
	})

	fieldIDs := make(pq.Int64Array, 0, len(fieldUIDs))
	for _, fieldUID := range fieldUIDs {
		field, ok := tableFieldsSet[fieldUID]
		if ok {
			fieldIDs = append(fieldIDs, int64(field.ID))
		}
	}

	if _, err := db.Views.Create(ctx.Request().Context(), db.CreateViewOptions{
		SLTableID:  table.ID,
		SLFieldIDs: fieldIDs,
		Filter:     filterJSON,
		Order:      nil,
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create view")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusCreated)
}

func (viewRoute) Viewer(ctx context.Context, table *db.Project) error {
	viewUID := ctx.Param("viewUID")

	view, err := db.Views.GetByUID(ctx.Request().Context(), viewUID)
	if err != nil {
		if errors.Is(err, db.ErrViewNotFound) {
			return ctx.ApiError(http.StatusNotFound, "视图不存在")
		}

		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get view")
		return ctx.ApiServerError()
	}

	if view.SLTableID != table.ID {
		return ctx.ApiError(http.StatusNotFound, "视图不存在")
	}

	ctx.Map(view)
	return nil
}

func (viewRoute) Get(ctx context.Context, view *db.View) error {
	return ctx.ApiSuccess(view)
}

func (viewRoute) Update(ctx context.Context, view *db.View, f form.UpdateView) error {
	fieldUIDs := f.FieldUIDs
	filter := f.Filter

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

	fieldIDs := make(pq.Int64Array, 0, len(fieldUIDs))
	for _, fieldUID := range fieldUIDs {
		field, ok := tableFieldsSet[fieldUID]
		if ok {
			fieldIDs = append(fieldIDs, int64(field.ID))
		}
	}

	if err := db.Views.Update(ctx.Request().Context(), view.ID, db.UpdateViewOptions{
		SLFieldIDs: fieldIDs,
		Filter:     filterJSON,
		Order:      nil,
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

	fmt.Println(page, pageSize)
	return nil
}
