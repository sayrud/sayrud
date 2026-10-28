package api

import (
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/form"
)

// NavigationPosition is the current position of a table or workflow in the project sidebar.
type NavigationPosition struct {
	UID      string `json:"uid"`
	Position int    `json:"position"`
} // @name NavigationPosition

// MoveNavigation
// @Summary Move a project sidebar item
// @Tags Projects
// @Description Moves a table or workflow before or after another item, using the latest order. Requires the editor role.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param data body form.MoveProjectNavigation true "Item and target"
// @Success 200 {array} NavigationPosition
// @Failure 400 {string} string "Invalid request body or item outside the project"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID moveProjectNavigation
// @Router /projects/{projectUID}/navigation/move [put]
func (projectRoute) MoveNavigation(ctx context.Context, hub *collab.Hub, project *db.Project, f form.MoveProjectNavigation) error {
	if f.UID == "" || f.TargetUID == "" || len(f.UID) > 64 || len(f.TargetUID) > 64 {
		return ctx.ApiError(http.StatusBadRequest, "common::invalid_body")
	}

	positions, err := db.Projects.MoveNavigation(ctx.Request().Context(), project.ID, f.UID, f.TargetUID, f.After)
	if err != nil {
		if errors.Is(err, db.ErrNavigationItemNotFound) {
			return ctx.ApiError(http.StatusBadRequest, "common::invalid_body")
		}

		if errors.Is(err, db.ErrProjectNotFound) {
			return ctx.ApiError(http.StatusNotFound, "project::not_found")
		}

		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to move project navigation item")
		return ctx.ApiServerError()
	}

	result := make([]NavigationPosition, len(positions))
	for i, item := range positions {
		result[i] = NavigationPosition{UID: item.UID, Position: item.Position}
	}

	hub.NotifyProject(project.UID, collab.MessageNavigationChanged)

	return ctx.ApiSuccess(result)
}
