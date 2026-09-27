// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package form

type CreateTable struct {
	Name string `json:"name" valid:"required" label:"数据表名称"`
} // @name CreateTable

type UpdateTable struct {
	Name string `json:"name" valid:"required" label:"数据表名称"`
} // @name UpdateTable

type CreateFields struct {
	Fields []CreateField `json:"fields" valid:"required" label:"字段"`
} // @name CreateFields

type CreateField struct {
	Label string `json:"label" valid:"required" label:"字段标题"`
	Type  string `json:"type" valid:"required" label:"字段类型" enums:"text,single_select,multi_select,datetime,number,checkbox,formula"`
	// Metadata is the type-specific configuration, e.g. select options or the formula expression.
	Metadata map[string]interface{} `json:"metadata,omitempty" label:"字段配置"`
} // @name CreateField

// UpdateField only updates the non-null properties.
type UpdateField struct {
	Label *string `json:"label,omitempty" label:"字段标题"`
	Type  *string `json:"type,omitempty" label:"字段类型" enums:"text,single_select,multi_select,datetime,number,checkbox,formula"`
	// Metadata replaces the whole field configuration, it is required when the type changes.
	Metadata map[string]interface{} `json:"metadata,omitempty" label:"字段配置"`
} // @name UpdateField

type UpdateFieldPosition struct {
	// Position is the zero-based index the field moves to, it is clamped into the valid range.
	Position int64 `json:"position" label:"字段位置"`
} // @name UpdateFieldPosition

type CreateRecord struct {
	// Data is the cell values keyed by field UID.
	Data map[string]interface{} `json:"data,omitempty" label:"字段数据"`
} // @name CreateRecord

type BatchCreateRecords struct {
	// Data is the list of cell values keyed by field UID, one item for each record.
	Data []map[string]interface{} `json:"data" valid:"required" label:"字段数据"`
} // @name BatchCreateRecords

type UpdateRecord struct {
	// Data replaces all the cell values of the record, keyed by field UID.
	Data map[string]interface{} `json:"data" label:"字段数据"`
} // @name UpdateRecord

type FetchRecords struct {
	UIDs []string `json:"uids" label:"记录 UID"`
} // @name FetchRecords

// QueryRecords filters, groups and sorts the records. Multiple filters are combined with AND.
type QueryRecords struct {
	Filter []QueryRecordsFilter `json:"filter,omitempty"`
	Order  []QueryRecordsSort   `json:"order,omitempty"`
	// Group sorts the records by the field values before Order, so records in the same group are adjacent.
	Group []QueryRecordsGroup `json:"group,omitempty"`
	// Limit defaults to 20 when it is not positive.
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
} // @name QueryRecords

type QueryRecordsFilter struct {
	FieldUID  string `json:"fieldUID"`
	Operation string `json:"operation" enums:"eq,neq,gt,lt,gte,lte,in,nin,like"`
	// Value is a JSON array (e.g. `["a","b"]`) for in / nin, and a single value for other operations.
	Value string `json:"value"`
} // @name QueryRecordsFilter

type QueryRecordsSort struct {
	FieldUID string `json:"fieldUID"`
	Order    string `json:"order,omitempty" enums:"asc,desc"`
} // @name QueryRecordsSort

type QueryRecordsGroup struct {
	FieldUID string `json:"fieldUID"`
} // @name QueryRecordsGroup
