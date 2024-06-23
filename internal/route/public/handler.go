// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/apibuilder"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
)

var Project publicHandler

type publicHandler struct{}

func (publicHandler) Handler(ctx context.Context) error {
	projectUID := ctx.Param("projectUID")
	path := "/" + ctx.Param("**")
	method := ctx.Request().Method

	project, err := db.Projects.GetByUID(ctx.Request().Context(), projectUID)
	if err != nil {
		if errors.Is(err, db.ErrProjectNotFound) {
			return ctx.ApiError(http.StatusNotFound, "项目不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get project by UID")
		return ctx.ApiServerError()
	}

	api, err := db.Apis.GetByMethodPath(ctx.Request().Context(), project.ID, method, path)
	if err != nil {
		if errors.Is(err, db.ErrApiNotFound) {
			return ctx.ApiError(http.StatusNotFound, "API 不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get api by method and path")
		return ctx.ApiServerError()
	}

	queryParams, err := apibuilder.ParseQueryParams(api.QueryParams)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to parse query params")
		return ctx.ApiServerError()
	}

	queryValues := make(map[string]interface{})
	queries := ctx.Request().URL.Query()
	for _, param := range queryParams {
		label := param.Label
		if label == "" {
			label = param.Key
		}

		inputValue := queries.Get(param.Key)
		v, err := param.ValidateValue(inputValue)
		if err != nil {
			if errors.Is(err, apibuilder.ErrParamValueRequired) {
				return ctx.ApiError(http.StatusBadRequest, "缺少必填参数: %s", label)
			}
			return ctx.ApiError(http.StatusBadRequest, "参数 %s 格式错误", label)
		}
		queryValues[param.Key] = v
	}

	// TODO parse body params
	bodyParams, err := apibuilder.ParseBodyParams(api.BodyParams)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to parse body params")
		return ctx.ApiServerError()
	}
	_ = bodyParams

	datasets, err := apibuilder.ParseDatasets(api.Datasets)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to parse datasets")
		return ctx.ApiServerError()
	}

	// Query datasets.
	datasetsResultSet := make(map[string][]map[string]interface{}, len(datasets))
	datasetsCountSet := make(map[string]int64, len(datasets))
	for _, dataset := range datasets {
		dataset := dataset

		filter, err := dataset.Filter.ToClauseExpression(nil)
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to parse filter expression")
			return ctx.ApiError(http.StatusInternalServerError, "数据集过滤条件解析失败")
		}

		result, count, err := db.SLTables.Query(ctx.Request().Context(), project.ID, dataset.TableUID, db.QuerySLTableOptions{
			Fields: dataset.Fields,
			Filter: filter,
			Order:  dataset.Order,
			Limit:  0,
			Offset: 0,
		})
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to query SL table")
			return ctx.ApiServerError()
		}

		datasetsResultSet[dataset.TableUID] = result
		datasetsCountSet[dataset.TableUID] = count
	}

	return ctx.ApiSuccess(datasetsResultSet)
}
