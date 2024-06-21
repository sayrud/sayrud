// Copyright 2024 E99p1ant. All rights reserved.
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
	projects, err := db.Projects.ListByUserID(ctx.Request().Context(), user.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list projects")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(projects)
}

func (projectRoute) CreateProject(ctx context.Context, user *db.User, f form.CreateProject) error {
	project, err := db.Projects.Create(ctx.Request().Context(), db.CreateProjectOptions{
		OwnerUserID: user.ID,
		Name:        f.Name,
		SchemaName:  f.SchemaName,
	})
	if err != nil {
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

func (projectRoute) DeleteProject(ctx context.Context, project *db.Project) error {
	if err := db.Projects.DeleteByID(ctx.Request().Context(), project.ID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete project")
		return ctx.ApiServerError()
	}

	return ctx.Status(http.StatusNoContent)
}
