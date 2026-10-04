// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package form

type CreateProject struct {
	Name string `json:"name" valid:"required"`
} // @name CreateProject

type UpdateProject struct {
	Name *string `json:"name,omitempty"`

	// Icon is a lowercase kebab-case identifier, an empty string resets the icon.
	Icon *string `json:"icon,omitempty" maxLength:"64"`
	// Color is orange, coral, pink, purple, indigo, blue, teal or green; an empty string resets the color.
	Color *string `json:"color,omitempty"`
} // @name UpdateProject

type AddProjectMember struct {
	Email string `json:"email" valid:"required"`
	Role  string `json:"role" valid:"required" enums:"manager,editor,viewer"`
} // @name AddProjectMember

type TransferProjectOwner struct {
	UserID int64 `json:"userID" valid:"required"`
} // @name TransferProjectOwner

type UpdateProjectMember struct {
	Role string `json:"role" valid:"required" enums:"manager,editor,viewer"`
} // @name UpdateProjectMember
