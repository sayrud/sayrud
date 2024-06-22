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

	type ListTableItem struct {
		UID   string `json:"uid"`
		Name  string `json:"name"`
		Label string `json:"label"`
		Desc  string `json:"desc"`
		Count int64  `json:"count"`
	}

	tables := make([]ListTableItem, 0, len(slTables))
	for _, table := range slTables {
		count, err := db.SLTables.ViewCount(ctx.Request().Context(), table.Project.SchemaName, table.Name)
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get view count")
			return ctx.ApiServerError()
		}

		tables = append(tables, ListTableItem{
			UID:   table.UID,
			Name:  table.Name,
			Label: table.Label,
			Desc:  table.Desc,
			Count: count,
		})
	}

	return ctx.ApiSuccess(map[string]interface{}{
		"tables": tables,
		"total":  total,
	})
}

func (schemalessRoute) CreateTable(ctx context.Context, project *db.Project, tx dbutil.Transactor, f form.CreateTable) error {
	var slTable *db.SLTable
	if err := tx.Transaction(func(tx *gorm.DB) error {
		sLTablesStore := db.NewSLTablesStore(tx)

		var err error
		slTable, err = sLTablesStore.Create(ctx.Request().Context(), project.ID, db.CreateSLTableOptions{
			Name:  f.Name,
			Label: f.Label,
			Desc:  f.Desc,
		})
		if err != nil {
			return errors.Wrap(err, "create sl table")
		}

		slTable.Project = *project

		if err := sLTablesStore.CreateView(ctx.Request().Context(), slTable); err != nil {
			return errors.Wrap(err, "create sl view")
		}
		return nil

	}); err != nil {
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
