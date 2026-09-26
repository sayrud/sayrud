// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package form

type CreateProject struct {
	Name string `json:"name" valid:"required" label:"项目名称"`
	// SchemaName is the Postgres schema of the project, a random one is generated if empty.
	SchemaName string `json:"schemaName,omitempty" label:"项目表名"`
} // @name CreateProject

type UpdateProject struct {
	Name string `json:"name" valid:"required" label:"项目名称"`
} // @name UpdateProject
