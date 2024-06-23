// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package form

import (
	"encoding/json"

	"github.com/wuhan005/sayrud/internal/apibuilder"
)

type CreateApi struct {
	Methods     []string               `json:"methods" valid:"required" label:"请求方法"`
	Kind        apibuilder.Kind        `json:"kind" valid:"required" label:"API 类型"`
	Path        string                 `json:"path" valid:"required" label:"请求路径"`
	QueryParams apibuilder.QueryParams `json:"queryParams" valid:"required" label:"GET Query 参数"`
	BodyParams  apibuilder.BodyParams  `json:"bodyParams" valid:"required" label:"POST Body 参数"`
	Datasets    apibuilder.Dataset     `json:"datasets" valid:"required" label:"数据集"`
	Response    json.RawMessage        `json:"response" valid:"required" label:"响应模板"`
}

type UpdateApi struct {
}
