// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package form

import (
	"github.com/wuhan005/sayrud/internal/apibuilder"
)

type CreateView struct {
	TableUID  string               `json:"tableUID" valid:"required" label:"表格 UID"`
	Name      string               `json:"name" valid:"required" label:"名称"`
	FieldUIDs []string             `json:"fieldUIDs" valid:"required" label:"字段 UID"`
	Filter    *apibuilder.Operator `json:"filter" label:"过滤条件"`
	Order     []apibuilder.Order   `json:"order" label:"排序条件"`
}

type UpdateView struct {
	Name      string               `json:"name" valid:"required" label:"名称"`
	FieldUIDs []string             `json:"fieldUIDs" valid:"required" label:"字段 UID"`
	Filter    *apibuilder.Operator `json:"filter" label:"过滤条件"`
	Order     []apibuilder.Order   `json:"order" label:"排序条件"`
}
