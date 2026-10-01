package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
)

var Collab collabRoute

type collabRoute struct{}

// Serve
// @Summary Collaborate on the project over WebSocket
// @Description Upgrade to WebSocket to submit changesets and receive the changes of the subscribed tables and the online members.
// @Description Messages are JSON `{type, reqId, data}`. Client: SUBSCRIBE, UNSUBSCRIBE, USER_CHANGES, PRESENCE, PING.
// @Description Server: HELLO, SUBSCRIBED, ACCEPT_COMMIT, REJECT_COMMIT, NEW_CHANGES, MEMBERS, TABLES_CHANGED, PROJECT_CHANGED, PERMISSION_CHANGED, ERROR, PONG.
// @Description The changesets of the viewers are rejected.
// @Param projectUID path string true "Project UID"
// @Success 101 "Switching Protocols"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @ID collaborate
// @Router /projects/{projectUID}/ws [get]
func (collabRoute) Serve(ctx context.Context, hub *collab.Hub, project *db.Project, user *db.User, role db.ProjectRole) error {
	if err := hub.Serve(ctx.ResponseWriter(), ctx.Request().Request, project, collab.Identity{
		UserID:   user.ID,
		MemberID: dto.MemberID(user.ID),
		Name:     user.UserName,
		Color:    dto.UserColor(user.ID),
		CanEdit:  role.AtLeast(db.ProjectRoleEditor),
	}, ctx.Locale()); err != nil {
		// The upgrader has written the error response.
		logrus.WithContext(ctx.Request().Context()).WithError(err).Warn("Failed to serve WebSocket")
	}
	return nil
}

// GetSnapshot
// @Summary Get the table snapshot
// @Description Return the fields, views and all the records of the table consistently at the latest revision.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Success 200 {object} dto.TableSnapshot
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID getTableSnapshot
// @Router /projects/{projectUID}/tables/{tableUID}/snapshot [get]
func (collabRoute) GetSnapshot(ctx context.Context, tx dbutil.Transactor, project *db.Project, table *db.SLTable) error {
	var snapshot dto.TableSnapshot
	if err := tx.Transaction(func(tx *gorm.DB) error {
		latest, err := db.NewSLTablesStore(tx).GetByID(ctx.Request().Context(), table.ID)
		if err != nil {
			return errors.Wrap(err, "get table")
		}
		fields, err := db.NewSLFieldsStore(tx).ListByTableID(ctx.Request().Context(), table.ID)
		if err != nil {
			return errors.Wrap(err, "list fields")
		}
		views, err := db.NewSLViewsStore(tx).ListByTableID(ctx.Request().Context(), table.ID)
		if err != nil {
			return errors.Wrap(err, "list views")
		}
		records, err := db.NewSLRecordsStore(tx).ListAll(ctx.Request().Context(), table.ID)
		if err != nil {
			return errors.Wrap(err, "list records")
		}

		snapshot = dto.TableSnapshot{
			Rev:     latest.Rev,
			Table:   dto.ToTable(project, latest),
			Fields:  dto.ToFields(table, fields),
			Views:   dto.ToViews(table, views),
			Records: dto.ToRecords(table, records),
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get table snapshot")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(snapshot)
}

const maxChangesetsPerPage = 200

// ListChangesets
// @Summary List the missing changesets
// @Description Return the changesets whose revision is greater than since in revision order, for the client to catch up after missing broadcasts.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param since query int true "The latest revision the client has applied"
// @Success 200 {object} dto.ListChangesetsResp
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID listChangesets
// @Router /projects/{projectUID}/tables/{tableUID}/changesets [get]
func (collabRoute) ListChangesets(ctx context.Context, table *db.SLTable) error {
	changesets, err := db.SLChangesets.ListSince(ctx.Request().Context(), table.ID, int64(ctx.QueryInt("since")), maxChangesetsPerPage+1)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list changesets")
		return ctx.ApiServerError()
	}

	resp := dto.ListChangesetsResp{
		Changesets: make([]*collab.Changeset, 0, len(changesets)),
		HasMore:    len(changesets) > maxChangesetsPerPage,
	}
	for _, changeset := range changesets[:min(len(changesets), maxChangesetsPerPage)] {
		var operations []collab.Operation
		if err := json.Unmarshal(changeset.Operations, &operations); err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to decode changeset operations")
			return ctx.ApiServerError()
		}
		resp.Changesets = append(resp.Changesets, &collab.Changeset{
			TableUID:   table.UID,
			Rev:        changeset.Rev,
			Signature:  changeset.Signature,
			ClientID:   changeset.ClientID,
			Operations: operations,
		})
	}
	return ctx.ApiSuccess(resp)
}

// ListViews
// @Summary List views
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Success 200 {array} dto.SLView
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID listViews
// @Router /projects/{projectUID}/tables/{tableUID}/views [get]
func (collabRoute) ListViews(ctx context.Context, table *db.SLTable) error {
	views, err := db.SLViews.ListByTableID(ctx.Request().Context(), table.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list views")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToViews(table, views))
}

// FetchRecords
// @Summary Fetch records by UID
// @Description Return the records with the given UIDs in creation order, the deleted ones are omitted. It is used to reload the dirty records.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param data body form.FetchRecords true "Record UIDs"
// @Success 200 {array} dto.Record
// @Failure 400 {string} string "Invalid request body"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID fetchRecords
// @Router /projects/{projectUID}/tables/{tableUID}/records/fetch [post]
func (collabRoute) FetchRecords(ctx context.Context, table *db.SLTable, f form.FetchRecords) error {
	if len(f.UIDs) > 1000 {
		return ctx.ApiError(http.StatusBadRequest, "record::too_many_records")
	}
	records, err := db.SLRecords.ListByUIDs(ctx.Request().Context(), table.ID, f.UIDs)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to fetch records")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToRecords(table, records))
}

// notifyDirty tells the collaborators to reload the data changed by the REST API, the failure is only logged.
func notifyDirty(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable, scope collab.DirtyScope) {
	if err := hub.NotifyDirty(ctx.Request().Context(), project, table, scope); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).WithField("tableUID", table.UID).Error("Failed to notify dirty data")
	}
}
