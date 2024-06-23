// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/apibuilder"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/form"
)

var Api apiRoute

type apiRoute struct{}

func (apiRoute) Apier(ctx context.Context, project *db.Project) error {
	apiUID := ctx.Param("apiUID")
	api, err := db.Apis.GetByUID(ctx.Request().Context(), apiUID)
	if err != nil {
		if errors.Is(err, db.ErrApiNotFound) {
			return ctx.ApiError(http.StatusNotFound, "API 不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get api by UID")
		return ctx.ApiServerError()
	}

	if api.ProjectID != project.ID {
		return ctx.ApiError(http.StatusNotFound, "API 不存在")
	}

	ctx.Map(api)
	return nil
}

func (apiRoute) List(ctx context.Context, project *db.Project) error {
	apis, total, err := db.Apis.List(ctx.Request().Context(), project.ID, db.ListApiOptions{
		Pagination: dbutil.Pagination{
			Page:     ctx.QueryInt("page"),
			PageSize: ctx.QueryInt("pageSize"),
		},
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list apis")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(map[string]interface{}{
		"apis":  apis,
		"total": total,
	})
}

func (apiRoute) Create(ctx context.Context, project *db.Project, f form.CreateApi) error {
	methods := make([]string, 0, len(f.Methods))
	for _, method := range f.Methods {
		if method == "*" && len(methods) > 1 {
			methods = []string{"*"}
			break
		} else {
			method = strings.ToUpper(method)
			if method != http.MethodGet && method != http.MethodPost && method != http.MethodPut && method != http.MethodDelete {
				return ctx.ApiError(http.StatusBadRequest, "请求方法 %T 不合法", method)
			}
			methods = append(methods, method)
		}
	}

	kind := f.Kind
	if err := kind.ValidateConfig(); err != nil {
		return ctx.ApiError(http.StatusBadRequest, "API 类型不合法")
	}

	path := "/" + strings.TrimSpace(strings.Trim(f.Path, "/"))

	queryParams := f.QueryParams
	if err := queryParams.ValidateConfig(); err != nil {
		switch {
		case errors.Is(err, apibuilder.ErrEmptyParamKey):
			return ctx.ApiError(http.StatusBadRequest, "查询参数的键不能为空")
		case errors.Is(err, apibuilder.ErrInvalidParamType):
			return ctx.ApiError(http.StatusBadRequest, "查询参数的类型不合法")
		case errors.Is(err, apibuilder.ErrEmptyValidatorExpression):
			return ctx.ApiError(http.StatusBadRequest, "查询参数的验证器表达式不能为空")
		case errors.Is(err, apibuilder.ErrDuplicateParamKey):
			return ctx.ApiError(http.StatusBadRequest, "查询参数的键重复")
		default:
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to validate query params")
			return ctx.ApiError(http.StatusInternalServerError, "验证查询参数失败")
		}
	}

	bodyParams := f.BodyParams
	if err := bodyParams.ValidateConfig(); err != nil {
		switch {
		case errors.Is(err, apibuilder.ErrEmptyParamKey):
			return ctx.ApiError(http.StatusBadRequest, "请求体参数的键不能为空")
		case errors.Is(err, apibuilder.ErrInvalidParamType):
			return ctx.ApiError(http.StatusBadRequest, "请求体参数的类型不合法")
		case errors.Is(err, apibuilder.ErrEmptyValidatorExpression):
			return ctx.ApiError(http.StatusBadRequest, "请求体参数的验证器表达式不能为空")
		case errors.Is(err, apibuilder.ErrDuplicateParamKey):
			return ctx.ApiError(http.StatusBadRequest, "请求体参数的键重复")
		default:
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to validate body params")
			return ctx.ApiError(http.StatusInternalServerError, "验证请求体参数失败")
		}
	}

	datasets := f.Datasets
	if err := datasets.ValidateConfig(); err != nil {
		switch {
		case errors.Is(err, apibuilder.ErrEmptyDatasetTableUID):
			return ctx.ApiError(http.StatusBadRequest, "数据表 UID 不能为空")
		case errors.Is(err, apibuilder.ErrEmptyDatasetFields):
			return ctx.ApiError(http.StatusBadRequest, "数据表字段不能为空")
		default:
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to validate datasets")
			return ctx.ApiError(http.StatusInternalServerError, "验证数据集失败")
		}
	}

	api, err := db.Apis.Create(ctx.Request().Context(), db.CreateApiOptions{
		ProjectID:   project.ID,
		Kind:        kind,
		Methods:     methods,
		Path:        path,
		QueryParams: queryParams.ToJSON(),
		BodyParams:  bodyParams.ToJSON(),
		Options:     datasets.ToJSON(),
		Response:    datatypes.JSON(f.Response),
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create api")
		return ctx.ApiServerError()
	}

	return ctx.ApiSuccess(api)
}

func (apiRoute) Get(ctx context.Context, project *db.Project, api *db.Api) error {
	return ctx.ApiSuccess(api)
}

func (apiRoute) Update(ctx context.Context, project *db.Project, api *db.Api, f form.UpdateApi) error {
	return nil
}

func (apiRoute) Delete(ctx context.Context, project *db.Project, api *db.Api) error {
	if err := db.Apis.DeleteByID(ctx.Request().Context(), api.ID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete api")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}
