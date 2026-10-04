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
	Icon       string    `json:"icon"`
	Color      string    `json:"color"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
} // @name SLTable

func ToTable(project *db.Project, table *db.SLTable) *Table {
	return &Table{
		UID:        table.UID,
		ProjectUID: project.UID,
		Name:       table.Name,
		Icon:       table.Icon,
		Color:      table.Color,
		CreatedAt:  table.CreatedAt,
		UpdatedAt:  table.UpdatedAt,
	}
}

// TableListItem is the table in the table list, with the number of its records.
type TableListItem struct {
	Table
	Count int64 `json:"count"`
} // @name TableListItem

// ListTablesResp is the response of the table list.
type ListTablesResp struct {
	Tables []*TableListItem `json:"tables"`
	Total  int64            `json:"total"`
} // @name ListTablesResp
