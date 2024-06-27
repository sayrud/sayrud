// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"encoding/json"
	"encoding/xml"
	"mime"
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"

	"github.com/wuhan005/sayrud/internal/apibuilder"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/jsvm"
)

var Project publicHandler

type publicHandler struct{}

func (h publicHandler) Handler(ctx context.Context) error {
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

	// Parse query params.
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

	// Parse post params if method is POST or PUT.
	bodyValues := make(map[string]interface{})
	if method == http.MethodPost || method == http.MethodPut {
		bodyParams, err := apibuilder.ParseBodyParams(api.BodyParams)
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to parse body params")
			return ctx.ApiServerError()
		}

		body := make(map[string]interface{})
		contentType := ctx.Request().Header.Get("Content-Type")
		bodyType, _, _ := mime.ParseMediaType(contentType)
		switch bodyType {
		case "multipart/form-data", "application/x-www-form-urlencoded":
			if err := ctx.Request().ParseForm(); err != nil {
				return ctx.ApiError(http.StatusBadRequest, "解析请求表单失败")
			}
			for k, v := range ctx.Request().Form {
				body[k] = v
			}

		case "application/json":
			bodyBytes, err := ctx.Request().Body().Bytes()
			if err != nil {
				return ctx.ApiError(http.StatusBadRequest, "解析 JSON 请求体失败")
			}
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				return ctx.ApiError(http.StatusBadRequest, "解析 JSON 请求体失败")
			}

		case "application/xml":
			bodyBytes, err := ctx.Request().Body().Bytes()
			if err != nil {
				return ctx.ApiError(http.StatusBadRequest, "解析 XML 请求体失败")
			}
			if err := xml.Unmarshal(bodyBytes, &body); err != nil {
				return ctx.ApiError(http.StatusBadRequest, "解析 XML 请求体失败")
			}

		default:
			return ctx.ApiError(http.StatusBadRequest, "不支持的请求体类型: %s", bodyType)
		}

		for _, param := range bodyParams {
			label := param.Label
			if label == "" {
				label = param.Key
			}

			inputValue := cast.ToString(body[param.Key])
			v, err := param.ValidateValue(inputValue)
			if err != nil {
				if errors.Is(err, apibuilder.ErrParamValueRequired) {
					return ctx.ApiError(http.StatusBadRequest, "缺少必填参数: %s", label)
				}
				return ctx.ApiError(http.StatusBadRequest, "参数 %s 格式错误", label)
			}
			bodyValues[param.Key] = v
		}
	}

	// Initialize JavaScript VM.
	vm, err := jsvm.NewVM(jsvm.NewVMOptions{
		RequestMethod: method,
		RequestPath:   path,
		RequestQuery:  queryValues,
		RequestBody:   bodyValues,
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create JS VM")
		return ctx.ApiServerError()
	}

	var response interface{}
	switch api.Kind {
	case apibuilder.KindList:
		options := apibuilder.Options[apibuilder.ListOptions]{}
		listOptions := options.ParseOptions(api.Options)

		response, err = h.listHandler(ctx, listHandlerOptions{
			projectID:   project.ID,
			vm:          vm,
			listOptions: listOptions,
		})
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to handle list")
			return ctx.ApiServerError()
		}

	case apibuilder.KindView:
		options := apibuilder.Options[apibuilder.ViewOptions]{}
		viewOptions := options.ParseOptions(api.Options)

		response, err = h.viewHandler(ctx, viewHandlerOptions{
			projectID:   project.ID,
			vm:          vm,
			viewOptions: viewOptions,
		})
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to handle view")
			return ctx.ApiServerError()
		}

	case apibuilder.KindCreate:
		options := apibuilder.Options[apibuilder.CreateOptions]{}
		createOptions := options.ParseOptions(api.Options)

		if err := h.createHandler(ctx, createHandlerOptions{
			projectID:     project.ID,
			queryValues:   queryValues,
			bodyValues:    bodyValues,
			createOptions: createOptions,
		}); err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to handle create")
			return ctx.ApiServerError()
		}

	case apibuilder.KindUpdate:
		options := apibuilder.Options[apibuilder.UpdateOptions]{}
		updateOptions := options.ParseOptions(api.Options)

		if err := h.updateHandler(ctx, updateHandlerOptions{
			projectID:     project.ID,
			vm:            vm,
			queryValues:   queryValues,
			bodyValues:    bodyValues,
			updateOptions: updateOptions,
		}); err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to handle update")
			return ctx.ApiServerError()
		}

	case apibuilder.KindDelete:
		options := apibuilder.Options[apibuilder.DeleteOptions]{}
		deleteOptions := options.ParseOptions(api.Options)

		response, err = h.deleteHandler(ctx, deleteHandlerOptions{
			projectID:     project.ID,
			vm:            vm,
			deleteOptions: deleteOptions,
		})
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to handle delete")
			return ctx.ApiServerError()
		}

	default:
		return ctx.ApiError(http.StatusInternalServerError, "未知的 API 类型: %s", api.Kind)
	}

	return ctx.ApiSuccess(response)
}
