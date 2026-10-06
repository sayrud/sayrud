// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
)

var Schemaless schemalessRoute

type schemalessRoute struct{}

// Tabler maps the table of the tableUID path parameter as *db.SLTable, it must belong to the current project.
func (schemalessRoute) Tabler(ctx context.Context, project *db.Project) error {
	tableUID := ctx.Param("tableUID")
	slTable, err := db.SLTables.GetByUID(ctx.Request().Context(), tableUID)
	if err != nil {
		if errors.Is(err, db.ErrSLTableNotFound) {
			return ctx.ApiError(http.StatusNotFound, "table::not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get sl table by UID")
		return ctx.ApiServerError()
	}

	if project.ID != slTable.ProjectID {
		return ctx.ApiError(http.StatusNotFound, "table::not_found")
	}

	ctx.Map(slTable)
	return nil
}

// ListTables
// @Summary List tables
// @Description List the tables of the project, with the number of records in each table.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param page query int false "Page number, starting from 1"
// @Param pageSize query int false "Page size, defaults to 20"
// @Success 200 {object} dto.ListTablesResp
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID listTables
// @Router /projects/{projectUID}/tables [get]
func (schemalessRoute) ListTables(ctx context.Context, project *db.Project) error {
	slTables, total, err := db.SLTables.Query(ctx.Request().Context(), db.QuerySLTableOptions{
		ProjectID: project.ID,
		Pagination: dbutil.Pagination{
			Page:     ctx.QueryInt("page"),
			PageSize: ctx.QueryInt("pageSize"),
		},
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list sl tables")
		return ctx.ApiServerError()
	}

	tableIDs := make([]int64, 0, len(slTables))
	for _, table := range slTables {
		tableIDs = append(tableIDs, table.ID)
	}
	counts, err := db.SLRecords.CountByTableIDs(ctx.Request().Context(), tableIDs)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to count sl records")
		return ctx.ApiServerError()
	}

	tables := make([]*dto.TableListItem, 0, len(slTables))
	for _, table := range slTables {
		tables = append(tables, &dto.TableListItem{
			Table: *dto.ToTable(project, table),
			Count: counts[table.ID],
		})
	}

	return ctx.ApiSuccess(dto.ListTablesResp{
		Tables: tables,
		Total:  total,
	})
}

// CreateTable
// @Summary Create a table
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param data body form.CreateTable true "Table to create"
// @Success 200 {object} dto.Table
// @Failure 400 {string} string "Invalid request body"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 409 {string} string "Table already exists"
// @Failure 500 {string} string "Internal server error"
// @ID createTable
// @Router /projects/{projectUID}/tables [post]
func (schemalessRoute) CreateTable(ctx context.Context, hub *collab.Hub, project *db.Project, f form.CreateTable) error {
	slTable, err := db.SLTables.Create(ctx.Request().Context(), project.ID, db.CreateSLTableOptions{
		Name: f.Name,
	})
	if err != nil {
		if errors.Is(err, db.ErrSLTableExists) {
			return ctx.ApiError(http.StatusConflict, "table::exists")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create sl table")
		return ctx.ApiServerError()
	}

	hub.NotifyProject(project.UID, collab.MessageTablesChanged)
	return ctx.ApiSuccess(dto.ToTable(project, slTable))
}

// GetTable
// @Summary Get a table
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Success 200 {object} dto.Table
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID getTable
// @Router /projects/{projectUID}/tables/{tableUID} [get]
func (schemalessRoute) GetTable(ctx context.Context, project *db.Project, table *db.SLTable) error {
	return ctx.ApiSuccess(dto.ToTable(project, table))
}

// UpdateTable
// @Summary Update a table
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param data body form.UpdateTable true "Table properties"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid request body"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateTable
// @Router /projects/{projectUID}/tables/{tableUID} [put]
func (schemalessRoute) UpdateTable(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable, f form.UpdateTable) error {
	if !validAppearanceUpdate(f.Name, f.Icon, f.Color) {
		return ctx.ApiError(http.StatusBadRequest, "common::invalid_body")
	}

	if err := db.SLTables.Update(ctx.Request().Context(), table.ID, db.UpdateSLTableOptions{
		Name:  f.Name,
		Icon:  f.Icon,
		Color: f.Color,
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update sl table")
		return ctx.ApiServerError()
	}

	hub.NotifyProject(project.UID, collab.MessageTablesChanged)
	return ctx.Status(http.StatusNoContent)
}

// DeleteTable
// @Summary Delete a table
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID deleteTable
// @Router /projects/{projectUID}/tables/{tableUID} [delete]
func (schemalessRoute) DeleteTable(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable) error {
	if err := db.SLTables.DeleteByID(ctx.Request().Context(), table.ID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete sl table")
		return ctx.ApiServerError()
	}
	hub.NotifyProject(project.UID, collab.MessageTablesChanged)
	return ctx.Status(http.StatusNoContent)
}
