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

var Schemaless schemalessRoute

type schemalessRoute struct{}

func (schemalessRoute) Tabler(ctx context.Context, project *db.Project) error {
	tableUID := ctx.Param("tableUID")
	slTable, err := db.SLTables.GetByUID(ctx.Request().Context(), tableUID)
	if err != nil {
		if errors.Is(err, db.ErrSLTableNotFound) {
			return ctx.ApiError(http.StatusNotFound, "数据表不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get sl table by UID")
		return ctx.ApiServerError()
	}

	if project.ID != slTable.ProjectID {
		return ctx.ApiError(http.StatusNotFound, "数据表不存在")
	}

	ctx.Map(slTable)
	return nil
}

func (schemalessRoute) ListTables(ctx context.Context, project *db.Project) error {
	slTables, total, err := db.SLTables.ListByProjectID(ctx.Request().Context(), project.ID, db.ListSLTableOptions{
		Page:     ctx.QueryInt("page"),
		PageSize: ctx.QueryInt("pageSize"),
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list sl tables")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(map[string]interface{}{
		"data":  slTables,
		"total": total,
	})
}

func (schemalessRoute) CreateTable(ctx context.Context, project *db.Project, f form.CreateTable) error {
	slTable, err := db.SLTables.Create(ctx.Request().Context(), project.ID, db.CreateSLTableOptions{
		Name:  f.Name,
		Label: f.Label,
		Desc:  f.Desc,
	})
	if err != nil {
		if errors.Is(err, db.ErrSLTableExists) {
			return ctx.ApiError(http.StatusConflict, "数据表已存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create sl table")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(slTable)
}

func (schemalessRoute) GetTable(ctx context.Context, table *db.SLTable) error {
	return ctx.ApiSuccess(table)
}

func (schemalessRoute) UpdateTable(ctx context.Context, table *db.SLTable, f form.UpdateTable) error {
	tableID := table.ID
	if err := db.SLTables.Update(ctx.Request().Context(), tableID, db.UpdateSLTableOptions{
		Name:  f.Name,
		Label: f.Label,
		Desc:  f.Desc,
	}); err != nil {
		if errors.Is(err, db.ErrSLTableNotFound) {
			return ctx.ApiError(http.StatusNotFound, "数据表不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update sl table")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

func (schemalessRoute) DeleteTable(ctx context.Context, table *db.SLTable) error {
	tableID := table.ID
	if err := db.SLTables.DeleteByID(ctx.Request().Context(), tableID); err != nil {
		if errors.Is(err, db.ErrSLTableNotFound) {
			return ctx.ApiError(http.StatusNotFound, "数据表不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete sl table")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}
