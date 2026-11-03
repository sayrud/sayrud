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
	// AdminAccess reports whether the role is raised by the system admin, it is only returned when getting a single project.
	AdminAccess bool `json:"adminAccess,omitempty"`
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

// ResolvedMembers are the users and the workflows referenced by the member and system fields of the project.
type ResolvedMembers struct {
	// Users are the requested users who are or were members of the project or wrote its records, the others are omitted.
	Users []*ResolvedUser `json:"users"`
	// Workflows are the requested workflows of the project, including the deleted ones.
	Workflows []*WorkflowName `json:"workflows"`
} // @name ResolvedMembers

// ResolvedUser is a user referenced by the member and system fields.
type ResolvedUser struct {
	User *UserBrief `json:"user"`
	// Left reports whether the user was a collaborator and has been removed from the project.
	Left bool `json:"left"`
} // @name ResolvedUser

// WorkflowName is the name of a workflow writing the records.
type WorkflowName struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
} // @name WorkflowName
