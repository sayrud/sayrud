package dto

import "github.com/wuhan005/sayrud/internal/db"

type LinkShare struct {
	Enabled         bool `json:"enabled"`
	IncludeChildren bool `json:"includeChildren"`
	PasswordEnabled bool `json:"passwordEnabled"`
	// Password is returned only by sharing settings restricted to project managers.
	Password string `json:"password,omitempty"`
	URL      string `json:"url"`
} // @name LinkShare

func ToLinkShare(project *db.Project, table *db.SLTable) LinkShare {
	share := LinkShare{
		Enabled:         table.ShareEnabled,
		IncludeChildren: table.ShareIncludeChildren,
		PasswordEnabled: table.SharePasswordHash != "",
	}
	if table.ShareEnabled {
		share.URL = "/base/" + project.UID + "/" + table.UID
	}

	return share
}

type ResolvedLinkShare struct {
	Token string `json:"token"`
} // @name ResolvedLinkShare

type SharedProject struct {
	Project         *Project         `json:"project"`
	Tables          []*TableListItem `json:"tables"`
	TableUID        string           `json:"tableUID"`
	IncludeChildren bool             `json:"includeChildren"`
} // @name SharedProject
