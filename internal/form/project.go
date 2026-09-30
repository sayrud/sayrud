// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package form

type CreateProject struct {
	Name string `json:"name" valid:"required" label:"项目名称"`
} // @name CreateProject

type UpdateProject struct {
	Name string `json:"name" valid:"required" label:"项目名称"`
} // @name UpdateProject

type AddProjectMember struct {
	Email string `json:"email" valid:"required" label:"邮箱"`
	Role  string `json:"role" valid:"required" label:"权限" enums:"manager,editor,viewer"`
} // @name AddProjectMember

type UpdateProjectMember struct {
	Role string `json:"role" valid:"required" label:"权限" enums:"manager,editor,viewer"`
} // @name UpdateProjectMember
