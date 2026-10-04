package dto

import (
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// Project is the project returned by the management API.
type Project struct {
	UID       string    `json:"uid"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// Role is the permission of the signed-in user on the project.
	Role  string     `json:"role" enums:"owner,manager,editor,viewer"`
	Owner *UserBrief `json:"owner"`
} // @name Project

func ToProject(project *db.Project, role db.ProjectRole, owner *db.User) *Project {
	p := &Project{
		UID:       project.UID,
		Name:      project.Name,
		Icon:      project.Icon,
		Color:     project.Color,
		CreatedAt: project.CreatedAt,
		UpdatedAt: project.UpdatedAt,
		Role:      string(role),
	}
	if owner != nil {
		p.Owner = ToUserBrief(owner)
	}
	return p
}

// ProjectListItem is the project in the project list, with the number of its tables.
type ProjectListItem struct {
	Project
	TableCount int64 `json:"tableCount"`
} // @name ProjectListItem

// ListProjectsResp is the response of the project list.
type ListProjectsResp struct {
	Projects []*ProjectListItem `json:"projects"`
	Total    int64              `json:"total"`
} // @name ListProjectsResp

// ProjectMember is a user who can access the project, including the owner.
type ProjectMember struct {
	User *UserBrief `json:"user"`
	Role string     `json:"role" enums:"owner,manager,editor,viewer"`
} // @name ProjectMember
