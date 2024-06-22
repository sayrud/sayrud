// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package form

type CreateTable struct {
	Name  string `json:"name" valid:"required" label:"数据表名"`
	Label string `json:"label" valid:"required" label:"数据表标签"`
	Desc  string `json:"desc" label:"数据表描述"`
}

type UpdateTable struct {
	Label string `json:"label" valid:"required" label:"数据表标签"`
	Desc  string `json:"desc" label:"数据表描述"`
}

type CreateFields struct {
	Fields []CreateField `json:"fields" valid:"required" label:"字段"`
}

type CreateField struct {
	Name    string                 `json:"name" valid:"required" label:"字段名"`
	Label   string                 `json:"label" valid:"required" label:"字段标签"`
	Type    string                 `json:"type" valid:"required" label:"字段类型"`
	Options map[string]interface{} `json:"options" label:"字段选项"`
}

type UpdateFields struct {
	Fields []UpdateField `json:"fields" valid:"required" label:"字段"`
}

type UpdateField struct {
	UID string `json:"uid" valid:"required" label:"字段 UID"`

	Name     string                 `json:"name" valid:"required" label:"字段名"`
	Label    string                 `json:"label" valid:"required" label:"字段标签"`
	Type     string                 `json:"type" valid:"required" label:"字段类型"`
	Options  map[string]interface{} `json:"options" label:"字段选项"`
	Position int                    `json:"position" label:"字段位置"`
}

type CreateRecord struct {
	Data map[uint]interface{} `json:"data" label:"字段数据"`
}

type UpdateRecord struct {
	Data map[uint]interface{} `json:"data" label:"字段数据"`
}
