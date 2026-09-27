package dto

import (
	"encoding/json"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/db"
)

// SLView is the table view returned by the management API.
type SLView struct {
	UID      string `json:"uid"`
	TableUID string `json:"tableUID"`
	Name     string `json:"name"`
	Type     string `json:"type" enums:"grid,kanban,gallery,form"`
	// Config is maintained by the frontend, e.g. filter, sort, group, hidden fields and field widths.
	Config   map[string]interface{} `json:"config"`
	Position int                    `json:"position"`
} // @name SLView

func ToView(table *db.SLTable, view *db.SLView) *SLView {
	config := map[string]interface{}{}
	_ = json.Unmarshal(view.Config, &config)

	return &SLView{
		UID:      view.UID,
		TableUID: table.UID,
		Name:     view.Name,
		Type:     string(view.Type),
		Config:   config,
		Position: view.Position,
	}
}

func ToViews(table *db.SLTable, views []*db.SLView) []*SLView {
	result := make([]*SLView, 0, len(views))
	for _, view := range views {
		result = append(result, ToView(table, view))
	}
	return result
}

// TableSnapshot is all the data of a table at a revision, the client applies the changesets after the revision to catch up.
type TableSnapshot struct {
	Rev     int64     `json:"rev"`
	Table   *Table    `json:"table"`
	Fields  []*Field  `json:"fields"`
	Views   []*SLView `json:"views"`
	Records []*Record `json:"records"`
} // @name TableSnapshot

// ListChangesetsResp is the response of the missing changesets.
type ListChangesetsResp struct {
	Changesets []*collab.Changeset `json:"changesets"`
	// HasMore reports there are more changesets after the last one, the client should fetch again from its revision.
	HasMore bool `json:"hasMore"`
} // @name ListChangesetsResp
