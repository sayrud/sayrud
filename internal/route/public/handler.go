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
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/apibuilder"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/jsvm"
	"github.com/wuhan005/sayrud/internal/middleware"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

var Project publicHandler

type publicHandler struct{}

func (h publicHandler) Middlewares(ctx context.Context) error {
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

	ctx.Map(project)
	ctx.Map(api)

	middlewares := apibuilder.ParseMiddlewares(api.Middlewares)
	if len(middlewares) != 0 {
		for _, mw := range middlewares {
			mw := mw

			if mw.Type == middleware.TypeMain {
				ctx.Next()
			} else {
				handler, err := middleware.Get(mw.Type, mw.Params)
				if err != nil {
					logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get middleware")
					return ctx.ApiServerError()
				}

				_ = handler.Handle(ctx)
				if ctx.ResponseWriter().Written() {
					return nil
				}
			}
		}
	}
	return nil
}

func (h publicHandler) Handler(ctx context.Context, api *db.Api, tx dbutil.Transactor) error {
	if ctx.ResponseWriter().Written() {
		return nil
	}

	path := "/" + ctx.Param("**")
	method := ctx.Request().Method

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
		RequestIP:     ctx.IP(),
		RequestHeader: ctx.Request().Header,
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create JS VM")
		return ctx.ApiServerError()
	}

	var response interface{}
	switch apibuilder.Kind(api.Kind) {
	case apibuilder.KindList:
		options := apibuilder.Options[apibuilder.ListOptions]{}
		listOptions := options.ParseOptions(api.Options)

		response, err = h.listHandler(ctx, listHandlerOptions{
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

		if err := tx.Transaction(func(tx *gorm.DB) error {
			return h.createHandler(ctx, createHandlerOptions{
				queryValues:   queryValues,
				bodyValues:    bodyValues,
				createOptions: createOptions,
				tx:            tx,
				vm:            vm,
			})
		}); err != nil {
			switch {
			case errors.Is(err, routeutil.ErrFieldTypeMismatch):
				return ctx.ApiError(http.StatusBadRequest, "字段类型不匹配")
			case errors.Is(err, db.ErrSLFieldNotFound):
				return ctx.ApiError(http.StatusBadRequest, "引用字段不存在")
			case errors.Is(err, db.ErrSLRecordNotFound):
				return ctx.ApiError(http.StatusBadRequest, "引用记录不存在")
			case errors.Is(err, routeutil.ErrExpressionError):
				return ctx.ApiError(http.StatusBadRequest, "表达式错误")
			case errors.Is(err, routeutil.ErrConstraintError):
				return ctx.ApiError(http.StatusBadRequest, "约束条件错误")
			default:
				logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to handle create")
				return ctx.ApiServerError()
			}
		}

	case apibuilder.KindUpdate:
		options := apibuilder.Options[apibuilder.UpdateOptions]{}
		updateOptions := options.ParseOptions(api.Options)

		if err := tx.Transaction(func(tx *gorm.DB) error {
			return h.updateHandler(ctx, updateHandlerOptions{
				vm:            vm,
				queryValues:   queryValues,
				bodyValues:    bodyValues,
				updateOptions: updateOptions,
				tx:            tx,
			})
		}); err != nil {
			switch {
			case errors.Is(err, routeutil.ErrFieldTypeMismatch):
				return ctx.ApiError(http.StatusBadRequest, "字段类型不匹配")
			case errors.Is(err, db.ErrSLFieldNotFound):
				return ctx.ApiError(http.StatusBadRequest, "引用字段不存在")
			case errors.Is(err, db.ErrSLRecordNotFound):
				return ctx.ApiError(http.StatusBadRequest, "引用记录不存在")
			case errors.Is(err, routeutil.ErrExpressionError):
				return ctx.ApiError(http.StatusBadRequest, "表达式错误")
			case errors.Is(err, routeutil.ErrConstraintError):
				return ctx.ApiError(http.StatusBadRequest, "约束条件错误")
			default:
				logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to handle update")
				return ctx.ApiServerError()
			}
		}

	case apibuilder.KindDelete:
		options := apibuilder.Options[apibuilder.DeleteOptions]{}
		deleteOptions := options.ParseOptions(api.Options)

		response, err = h.deleteHandler(ctx, deleteHandlerOptions{
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
