// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package form

import (
	"github.com/wuhan005/sayrud/internal/apibuilder"
)

type CreateView struct {
	FieldUIDs []string             `json:"fieldUIDs" valid:"required" label:"字段 UID"`
	Filter    *apibuilder.Operator `json:"filter" label:"过滤条件"`
}

type UpdateView struct {
	FieldUIDs []string             `json:"fieldUIDs" valid:"required" label:"字段 UID"`
	Filter    *apibuilder.Operator `json:"filter" label:"过滤条件"`
}
