// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/thanhpk/randstr"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
)

var Project projectRoute

type projectRoute struct{}

// Projecter maps the project of the projectUID path parameter as *db.Project, it must be owned by the signed-in user.
func (projectRoute) Projecter(ctx context.Context, user *db.User) error {
	projectUID := ctx.Param("projectUID")
	project, err := db.Projects.GetByUID(ctx.Request().Context(), projectUID)
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			return ctx.ApiError(http.StatusNotFound, "项目不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project by UID")
		return ctx.ApiServerError()
	}

	if project.OwnerUserID != user.ID {
		return ctx.ApiError(http.StatusForbidden, "无权访问")
	}

	ctx.Map(project)
	return nil
}

// ListProjects
// @Summary List projects
// @Description List the projects owned by the signed-in user, with the number of tables in each project.
// @Produce json
// @Param page query int false "Page number, starting from 1"
// @Param pageSize query int false "Page size, defaults to 20"
// @Success 200 {object} dto.ListProjectsResp
// @Failure 500 {string} string "Internal server error"
// @ID listProjects
// @Router /projects [get]
func (projectRoute) ListProjects(ctx context.Context, user *db.User) error {
	projects, total, err := db.Projects.ListByUserID(ctx.Request().Context(), user.ID, db.ListByUserIDOptions{
		Pagination: dbutil.Pagination{
			Page:     ctx.QueryInt("page"),
			PageSize: ctx.QueryInt("pageSize"),
		},
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list projects")
		return ctx.ApiServerError()
	}

	items := make([]*dto.ProjectListItem, 0, len(projects))
	for _, project := range projects {
		tableCount, err := db.SLTables.CountByProjectID(ctx.Request().Context(), project.ID)
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to count tables")
			return ctx.ApiServerError()
		}
		items = append(items, &dto.ProjectListItem{
			Project:    *dto.ToProject(project),
			TableCount: tableCount,
		})
	}

	return ctx.ApiSuccess(dto.ListProjectsResp{
		Projects: items,
		Total:    total,
	})
}

// CreateProject
// @Summary Create a project
// @Description Create a project owned by the signed-in user, along with its Postgres schema.
// @Accept json
// @Produce json
// @Param data body form.CreateProject true "Project to create"
// @Success 200 {object} dto.Project
// @Failure 400 {string} string "Invalid request body"
// @Failure 409 {string} string "Schema name already exists"
// @Failure 500 {string} string "Internal server error"
// @ID createProject
// @Router /projects [post]
func (projectRoute) CreateProject(ctx context.Context, user *db.User, tx dbutil.Transactor, f form.CreateProject) error {
	schemaName := f.SchemaName
	if schemaName == "" {
		schemaName = "p_" + randstr.Hex(8)
	}

	var project *db.Project
	if err := tx.Transaction(func(tx *gorm.DB) error {
		projectStore := db.NewProjectsStore(tx)

		var err error
		project, err = projectStore.Create(ctx.Request().Context(), db.CreateProjectOptions{
			OwnerUserID: user.ID,
			Name:        f.Name,
			SchemaName:  schemaName,
		})
		if err != nil {
			return errors.Wrap(err, "create project")
		}

		if err := projectStore.CreateSchema(ctx.Request().Context(), project.ID); err != nil {
			return errors.Wrap(err, "create schema")
		}
		return nil
	}); err != nil {
		if errors.Is(err, db.ErrProjectSchemaNameExists) {
			return ctx.ApiError(http.StatusConflict, "项目表名已存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create project")
		return ctx.ApiServerError()
	}

	return ctx.ApiSuccess(dto.ToProject(project))
}

// GetProject
// @Summary Get a project
// @Produce json
// @Param projectUID path string true "Project UID"
// @Success 200 {object} dto.Project
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID getProject
// @Router /projects/{projectUID} [get]
func (projectRoute) GetProject(ctx context.Context, project *db.Project) error {
	return ctx.ApiSuccess(dto.ToProject(project))
}

// UpdateProject
// @Summary Update a project
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param data body form.UpdateProject true "Project properties"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid request body"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateProject
// @Router /projects/{projectUID} [put]
func (projectRoute) UpdateProject(ctx context.Context, project *db.Project, f form.UpdateProject) error {
	if err := db.Projects.Update(ctx.Request().Context(), project.ID, db.UpdateProjectOptions{
		Name: f.Name,
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update project")
		return ctx.ApiServerError()
	}

	return ctx.Status(http.StatusNoContent)
}

// DeleteProject
// @Summary Delete a project
// @Description Delete the project and drop its Postgres schema.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID deleteProject
// @Router /projects/{projectUID} [delete]
func (projectRoute) DeleteProject(ctx context.Context, project *db.Project, tx dbutil.Transactor) error {
	if err := tx.Transaction(func(tx *gorm.DB) error {
		projectsStore := db.NewProjectsStore(tx)

		// Delete the schema firstly.
		if err := projectsStore.DeleteSchema(ctx.Request().Context(), project.ID); err != nil {
			return errors.Wrap(err, "delete schema")
		}

		if err := projectsStore.DeleteByID(ctx.Request().Context(), project.ID); err != nil {
			return errors.Wrap(err, "delete project")
		}

		// TODO: the tables, fields, records are not deleted.
		return nil
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete project")
		return ctx.ApiServerError()
	}

	return ctx.Status(http.StatusNoContent)
}
