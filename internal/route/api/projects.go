// Copyright 2024 E99p1ant. All rights reserved.
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

var Project projectRoute

type projectRoute struct{}

func (projectRoute) Projecter(ctx context.Context, user *db.User) error {
	projectUID := ctx.Param("projectUID")
	slField, err := db.Projects.GetByUID(ctx.Request().Context(), projectUID)
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			return ctx.ApiError(http.StatusNotFound, "项目不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project by UID")
		return ctx.ApiServerError()
	}

	if slField.OwnerUserID != user.ID {
		return ctx.ApiError(http.StatusForbidden, "无权访问")
	}

	ctx.Map(slField)
	return nil
}

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
	return ctx.ApiSuccess(map[string]interface{}{
		"projects": projects,
		"total":    total,
	})
}

func (projectRoute) CreateProject(ctx context.Context, user *db.User, tx dbutil.Transactor, f form.CreateProject) error {
	var project *db.Project
	if err := tx.Transaction(func(tx *gorm.DB) error {
		projectStore := db.NewProjectsStore(tx)

		var err error
		project, err = projectStore.Create(ctx.Request().Context(), db.CreateProjectOptions{
			OwnerUserID: user.ID,
			Name:        f.Name,
			SchemaName:  f.SchemaName,
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

	return ctx.ApiSuccess(project)
}

func (projectRoute) GetProject(ctx context.Context, project *db.Project) error {
	return ctx.ApiSuccess(project)
}

func (projectRoute) UpdateProject(ctx context.Context, project *db.Project, f form.UpdateProject) error {
	if err := db.Projects.Update(ctx.Request().Context(), project.ID, db.UpdateProjectOptions{
		Name: f.Name,
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update project")
		return ctx.ApiServerError()
	}

	return ctx.Status(http.StatusNoContent)
}

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
