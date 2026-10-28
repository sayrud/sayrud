package dto

import (
	"time"

	"github.com/wuhan005/sayrud/internal/db"
)

// Table is the schemaless table returned by the management API.
type Table struct {
	UID        string    `json:"uid"`
	ProjectUID string    `json:"projectUID"`
	Name       string    `json:"name"`
	Position   int       `json:"position"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
} // @name SLTable

func ToTable(project *db.Project, table *db.SLTable) *Table {
	return &Table{
		UID:        table.UID,
		ProjectUID: project.UID,
		Name:       table.Name,
		Position:   table.Position,
		CreatedAt:  table.CreatedAt,
		UpdatedAt:  table.UpdatedAt,
	}
}

// ListTablesResp is the response of the table list.
type ListTablesResp struct {
	Tables []*Table `json:"tables"`
	Total  int64    `json:"total"`
} // @name ListTablesResp
