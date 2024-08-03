// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	gocontext "context"
	"encoding/json"
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

type validatedData struct {
	Methods     []string
	Kind        apibuilder.Kind
	QueryParams apibuilder.QueryParams
	BodyParams  apibuilder.BodyParams
	Datasets    apibuilder.Datasets
	Middlewares apibuilder.Middlewares
	Options     datatypes.JSON
}

func validateApiForm(ctx context.Context, validateCtx gocontext.Context, f form.CreateUpdateApi) (*validatedData, error) {
	methods := make([]string, 0, len(f.Methods))
	for _, method := range f.Methods {
		if method == "*" && len(methods) > 1 {
			methods = []string{"*"}
			break
		} else {
			method = strings.ToUpper(method)
			if method != http.MethodGet && method != http.MethodPost && method != http.MethodPut && method != http.MethodDelete {
				return nil, ctx.ApiError(http.StatusBadRequest, "请求方法 %T 不合法", method)
			}
			methods = append(methods, method)
		}
	}

	kind := f.Kind
	if err := kind.ValidateConfig(validateCtx); err != nil {
		return nil, ctx.ApiError(http.StatusBadRequest, "API 类型不合法")
	}
	validateCtx = apibuilder.WithKind(validateCtx, kind)

	queryParams := f.QueryParams
	if err := queryParams.ValidateConfig(validateCtx); err != nil {
		switch {
		case errors.Is(err, apibuilder.ErrEmptyParamKey):
			return nil, ctx.ApiError(http.StatusBadRequest, "查询参数的键不能为空")
		case errors.Is(err, apibuilder.ErrInvalidParamType):
			return nil, ctx.ApiError(http.StatusBadRequest, "查询参数的类型不合法")
		case errors.Is(err, apibuilder.ErrEmptyValidatorExpression):
			return nil, ctx.ApiError(http.StatusBadRequest, "查询参数的验证器表达式不能为空")
		case errors.Is(err, apibuilder.ErrDuplicateParamKey):
			return nil, ctx.ApiError(http.StatusBadRequest, "查询参数的键重复")
		default:
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to validate query params")
			return nil, ctx.ApiError(http.StatusInternalServerError, "验证查询参数失败")
		}
	}

	bodyParams := f.BodyParams
	if err := bodyParams.ValidateConfig(validateCtx); err != nil {
		switch {
		case errors.Is(err, apibuilder.ErrEmptyParamKey):
			return nil, ctx.ApiError(http.StatusBadRequest, "请求体参数的键不能为空")
		case errors.Is(err, apibuilder.ErrInvalidParamType):
			return nil, ctx.ApiError(http.StatusBadRequest, "请求体参数的类型不合法")
		case errors.Is(err, apibuilder.ErrEmptyValidatorExpression):
			return nil, ctx.ApiError(http.StatusBadRequest, "请求体参数的验证器表达式不能为空")
		case errors.Is(err, apibuilder.ErrDuplicateParamKey):
			return nil, ctx.ApiError(http.StatusBadRequest, "请求体参数的键重复")
		default:
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to validate body params")
			return nil, ctx.ApiError(http.StatusInternalServerError, "验证请求体参数失败")
		}
	}

	datasets := f.Datasets
	if err := datasets.ValidateConfig(validateCtx); err != nil {
		switch {
		case errors.Is(err, apibuilder.ErrEmptyDatasets):
			return nil, ctx.ApiError(http.StatusBadRequest, "数据集不能为空")
		case errors.Is(err, apibuilder.ErrEmptyDatasetTableUID):
			return nil, ctx.ApiError(http.StatusBadRequest, "数据表 UID 不能为空")
		case errors.Is(err, apibuilder.ErrDatasetTableNotFound):
			return nil, ctx.ApiError(http.StatusBadRequest, "数据表不存在")
		case errors.Is(err, apibuilder.ErrEmptyDatasetFields):
			return nil, ctx.ApiError(http.StatusBadRequest, "数据表字段不能为空")
		case errors.Is(err, apibuilder.ErrFieldNotFound):
			return nil, ctx.ApiError(http.StatusBadRequest, "字段不存在")
		default:
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to validate datasets")
			return nil, ctx.ApiError(http.StatusInternalServerError, "验证数据集失败")
		}
	}
	tableUID := datasets[0].TableUID

	filter := f.Filter
	if kind == apibuilder.KindUpdate || kind == apibuilder.KindDelete {
		if filter != nil {
			if err := filter.ValidateConfig(validateCtx); err != nil {
				return nil, ctx.ApiError(http.StatusBadRequest, "筛选条件校验不通过: %s", err)
			}
		}
	}

	fieldMapping := f.FieldMapping
	if kind == apibuilder.KindCreate || kind == apibuilder.KindUpdate {
		for param, fieldUID := range fieldMapping {
			groups := strings.SplitN(param, ":", 2)
			if len(groups) != 2 {
				return nil, ctx.ApiError(http.StatusBadRequest, "参数映射格式错误")
			}

			paramKey := groups[1]
			switch groups[0] {
			case "query":
				if !queryParams.HasKey(paramKey) {
					return nil, ctx.ApiError(http.StatusBadRequest, "参数映射字段不存在: %q", groups[0])
				}
			case "body":
				if !bodyParams.HasKey(paramKey) {
					return nil, ctx.ApiError(http.StatusBadRequest, "参数映射字段不存在: %q", groups[0])
				}
			default:
				return nil, ctx.ApiError(http.StatusBadRequest, "参数映射格式错误: %q", groups[0])
			}

			// TODO: Check fieldUID is valid.
			if fieldUID == "" {
				return nil, ctx.ApiError(http.StatusBadRequest, "参数映射值不能为空")
			}
		}
	}

	middlewares := f.Middlewares
	if err := middlewares.ValidateConfig(validateCtx); err != nil {
		switch {
		case errors.Is(err, apibuilder.ErrInvalidMiddlewareType):
			return nil, ctx.ApiError(http.StatusBadRequest, "中间件类型不存在")
		default:
			return nil, ctx.ApiError(http.StatusInternalServerError, "验证中间件失败")
		}
	}

	var options datatypes.JSON
	switch kind {
	case apibuilder.KindList:
		options = apibuilder.Options[apibuilder.ListOptions]{
			Value: apibuilder.ListOptions{
				Datasets:     datasets,
				FieldMapping: fieldMapping,
			},
		}.ToJSON()
	case apibuilder.KindView:
		options = apibuilder.Options[apibuilder.ViewOptions]{
			Value: apibuilder.ViewOptions{
				Datasets:     datasets,
				FieldMapping: fieldMapping,
			},
		}.ToJSON()

	case apibuilder.KindCreate:
		options = apibuilder.Options[apibuilder.CreateOptions]{
			Value: apibuilder.CreateOptions{
				TableUID:     tableUID,
				FieldMapping: fieldMapping,
			},
		}.ToJSON()
	case apibuilder.KindUpdate:
		options = apibuilder.Options[apibuilder.UpdateOptions]{
			Value: apibuilder.UpdateOptions{
				TableUID:     tableUID,
				FieldMapping: fieldMapping,
				Filter:       filter,
			},
		}.ToJSON()
	case apibuilder.KindDelete:
		options = apibuilder.Options[apibuilder.DeleteOptions]{
			Value: apibuilder.DeleteOptions{
				TableUID: tableUID,
				Filter:   filter,
			},
		}.ToJSON()
	}

	return &validatedData{
		Methods:     methods,
		Kind:        kind,
		QueryParams: queryParams,
		BodyParams:  bodyParams,
		Datasets:    datasets,
		Middlewares: middlewares,
		Options:     options,
	}, nil
}

func (h apiRoute) Create(ctx context.Context, project *db.Project, f form.CreateUpdateApi) error {
	validateCtx := gocontext.Background()
	validateCtx = apibuilder.WithProjectID(validateCtx, project.ID)

	data, _ := validateApiForm(ctx, validateCtx, f)
	if ctx.ResponseWriter().Written() {
		return nil
	}

	path := "/" + strings.TrimSpace(strings.Trim(f.Path, "/"))

	var r interface{}
	if err := json.Unmarshal([]byte(f.Response), &r); err != nil {
		return ctx.ApiError(http.StatusBadRequest, "响应模板 JSON 格式错误")
	}

	api, err := db.Apis.Create(ctx.Request().Context(), db.CreateApiOptions{
		ProjectID:   project.ID,
		Kind:        string(data.Kind),
		Methods:     data.Methods,
		Path:        path,
		QueryParams: data.QueryParams.ToJSON(),
		BodyParams:  data.BodyParams.ToJSON(),
		Options:     data.Options,
		Middlewares: data.Middlewares.ToJSON(),
		Response:    datatypes.JSON(f.Response),
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create api")
		return ctx.ApiServerError()
	}

	return ctx.ApiSuccess(api)
}

func (apiRoute) Get(ctx context.Context, api *db.Api) error {
	return ctx.ApiSuccess(api)
}

func (apiRoute) Update(ctx context.Context, project *db.Project, api *db.Api, f form.CreateUpdateApi) error {
	validateCtx := gocontext.Background()
	validateCtx = apibuilder.WithProjectID(validateCtx, project.ID)

	data, _ := validateApiForm(ctx, validateCtx, f)
	if ctx.ResponseWriter().Written() {
		return nil
	}

	path := "/" + strings.TrimSpace(strings.Trim(f.Path, "/"))

	var r interface{}
	if err := json.Unmarshal([]byte(f.Response), &r); err != nil {
		return ctx.ApiError(http.StatusBadRequest, "响应模板 JSON 格式错误")
	}

	if err := db.Apis.Update(ctx.Request().Context(), api.ID, db.UpdateApiOptions{
		Kind:        string(data.Kind),
		Methods:     data.Methods,
		Path:        path,
		QueryParams: data.QueryParams.ToJSON(),
		BodyParams:  data.BodyParams.ToJSON(),
		Options:     data.Options,
		Middlewares: data.Middlewares.ToJSON(),
		Response:    datatypes.JSON(f.Response),
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update api")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

func (apiRoute) Delete(ctx context.Context, project *db.Project, api *db.Api) error {
	if err := db.Apis.DeleteByID(ctx.Request().Context(), api.ID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete api")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}
