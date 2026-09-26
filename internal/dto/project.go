package dto

import (
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// Project is the project returned by the management API.
type Project struct {
	UID        string    `json:"uid"`
	Name       string    `json:"name"`
	SchemaName string    `json:"schemaName"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
} // @name Project

func ToProject(project *db.Project) *Project {
	return &Project{
		UID:        project.UID,
		Name:       project.Name,
		SchemaName: project.SchemaName,
		CreatedAt:  project.CreatedAt,
		UpdatedAt:  project.UpdatedAt,
	}
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
