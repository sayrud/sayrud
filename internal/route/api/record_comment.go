package api

import (
	"net/http"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
)

const (
	defaultPageLimit = 20
	maxPageLimit     = 100
)

// pageLimit returns the limit query parameter clamped into [1, maxPageLimit], defaultPageLimit if absent.
func pageLimit(ctx context.Context) int {
	limit := ctx.QueryInt("limit")
	if limit <= 0 {
		return defaultPageLimit
	}
	return min(limit, maxPageLimit)
}

// Commenter maps the comment of the commentID path parameter as *db.SLRecordComment, it must belong to the current record.
func (schemalessRoute) Commenter(ctx context.Context, record *db.SLRecord) error {
	id, _ := strconv.ParseInt(ctx.Param("commentID"), 10, 64)
	comment, err := db.SLRecordComments.GetByID(ctx.Request().Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrSLRecordCommentNotFound) {
			return ctx.ApiError(http.StatusNotFound, "comment::not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get record comment")
		return ctx.ApiServerError()
	}
	if comment.SLRecordID != record.ID {
		return ctx.ApiError(http.StatusNotFound, "comment::not_found")
	}
	ctx.Map(comment)
	return nil
}

// commentErrorResponse returns the response of the comment validation errors, ok is false if the error is unexpected.
func commentErrorResponse(ctx context.Context, err error) (error, bool) {
	switch {
	case errors.Is(err, collab.ErrCommentEmpty):
		return ctx.ApiError(http.StatusBadRequest, "comment::body_required"), true
	case errors.Is(err, collab.ErrCommentTooLong):
		return ctx.ApiError(http.StatusBadRequest, "comment::body_too_long", collab.MaxCommentLength), true
	case errors.Is(err, collab.ErrInvalidMention):
		return ctx.ApiError(http.StatusBadRequest, "comment::invalid_mention"), true
	case errors.Is(err, collab.ErrCommentDeleted):
		return ctx.ApiError(http.StatusBadRequest, "comment::deleted"), true
	case errors.Is(err, db.ErrSLRecordCommentNotFound):
		return ctx.ApiError(http.StatusNotFound, "comment::not_found"), true
	case errors.Is(err, db.ErrSLRecordNotFound):
		return ctx.ApiError(http.StatusNotFound, "record::not_found"), true
	default:
		return nil, false
	}
}

// ListComments
// @Summary List the comments of a record
// @Tags Record comments
// @Description Return the root comments the newest first, each with all its live replies in creation order. A deleted root is kept as a placeholder while it has replies.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param recordUID path string true "Record UID"
// @Param cursor query int false "Return the root comments older than the cursor"
// @Param limit query int false "Maximum number of root comments, defaults to 20 and at most 100"
// @Success 200 {object} dto.ListRecordCommentsResp
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table or record not found"
// @Failure 500 {string} string "Internal server error"
// @ID listRecordComments
// @Router /projects/{projectUID}/tables/{tableUID}/records/{recordUID}/comments [get]
func (schemalessRoute) ListComments(ctx context.Context, record *db.SLRecord) error {
	c := ctx.Request().Context()
	limit := pageLimit(ctx)
	roots, err := db.SLRecordComments.ListRoots(c, record.ID, ctx.QueryInt64("cursor"), limit+1)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list record comments")
		return ctx.ApiServerError()
	}
	resp := dto.ListRecordCommentsResp{Comments: make([]*collab.Comment, 0, len(roots))}
	if len(roots) > limit {
		roots = roots[:limit]
		resp.NextCursor = roots[limit-1].ID
	}
	replies, err := db.SLRecordComments.ListReplies(c, lo.Map(roots, func(r *db.SLRecordComment, _ int) int64 { return r.ID }))
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list comment replies")
		return ctx.ApiServerError()
	}
	byRoot := lo.GroupBy(replies, func(r *db.SLRecordComment) int64 { return r.RootID })
	for _, root := range roots {
		item := collab.NewComment(record.UID, root)
		item.Replies = make([]*collab.Comment, 0, len(byRoot[root.ID]))
		for _, reply := range byRoot[root.ID] {
			item.Replies = append(item.Replies, collab.NewComment(record.UID, reply))
		}
		resp.Comments = append(resp.Comments, item)
	}
	return ctx.ApiSuccess(resp)
}

// CreateComment
// @Summary Comment on a record
// @Tags Record comments
// @Description Post a root comment or a reply, the replies of a reply belong to the same root. Any member of the project may comment, including the viewers and the bots.
// @Description Mention a member with <@userID>, the online members mentioned are told over WebSocket.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param recordUID path string true "Record UID"
// @Param data body form.CreateRecordComment true "Comment"
// @Success 200 {object} collab.Comment
// @Failure 400 {string} string "Empty or too long body, unsupported kind, deleted comment replied to or user mentioned out of the project"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table, record or comment replied to not found"
// @Failure 500 {string} string "Internal server error"
// @ID createRecordComment
// @Router /projects/{projectUID}/tables/{tableUID}/records/{recordUID}/comments [post]
func (schemalessRoute) CreateComment(ctx context.Context, hub *collab.Hub, user *db.User, project *db.Project, table *db.SLTable, record *db.SLRecord, f form.CreateRecordComment) error {
	if f.Kind != "" && f.Kind != string(db.SLRecordCommentKindComment) {
		return ctx.ApiError(http.StatusBadRequest, "comment::unsupported_kind")
	}
	comment, err := hub.CreateComment(ctx.Request().Context(), project, table, record, collab.Origin{UserID: user.ID}, collab.CommentOptions{
		Body:      f.Body,
		ReplyToID: f.ReplyToID,
	})
	if err != nil {
		if resp, ok := commentErrorResponse(ctx, err); ok {
			return resp
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create record comment")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(comment)
}

// GetComment
// @Summary Get a comment of a record
// @Tags Record comments
// @Description Return the full comment, e.g. a comment truncated in the timeline; a deleted comment has an empty body.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param recordUID path string true "Record UID"
// @Param commentID path int true "Comment ID"
// @Success 200 {object} collab.Comment
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table, record or comment not found"
// @Failure 500 {string} string "Internal server error"
// @ID getRecordComment
// @Router /projects/{projectUID}/tables/{tableUID}/records/{recordUID}/comments/{commentID} [get]
func (schemalessRoute) GetComment(ctx context.Context, record *db.SLRecord, comment *db.SLRecordComment) error {
	return ctx.ApiSuccess(collab.NewComment(record.UID, comment))
}

// UpdateComment
// @Summary Edit a comment
// @Tags Record comments
// @Description Replace the body of the user's own comment, the mentions are parsed again and only the newly mentioned members are told.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param recordUID path string true "Record UID"
// @Param commentID path int true "Comment ID"
// @Param data body form.UpdateRecordComment true "Comment"
// @Success 200 {object} collab.Comment
// @Failure 400 {string} string "Empty or too long body, deleted comment or user mentioned out of the project"
// @Failure 403 {string} string "Not the author"
// @Failure 404 {string} string "Project, table, record or comment not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateRecordComment
// @Router /projects/{projectUID}/tables/{tableUID}/records/{recordUID}/comments/{commentID} [put]
func (schemalessRoute) UpdateComment(ctx context.Context, hub *collab.Hub, user *db.User, project *db.Project, table *db.SLTable, record *db.SLRecord, comment *db.SLRecordComment, f form.UpdateRecordComment) error {
	if comment.AuthorWorkflowID != 0 || comment.AuthorID != user.ID {
		return ctx.ApiError(http.StatusForbidden, "comment::edit_own_only")
	}
	updated, err := hub.UpdateComment(ctx.Request().Context(), project, table, record, comment.ID, f.Body)
	if err != nil {
		if resp, ok := commentErrorResponse(ctx, err); ok {
			return resp
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update record comment")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(updated)
}

// DeleteComment
// @Summary Delete a comment
// @Tags Record comments
// @Description Delete a comment of the user, or any comment for the managers. A deleted root comment with replies stays as a placeholder.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param recordUID path string true "Record UID"
// @Param commentID path int true "Comment ID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Neither the author nor a manager"
// @Failure 404 {string} string "Project, table, record or comment not found"
// @Failure 500 {string} string "Internal server error"
// @ID deleteRecordComment
// @Router /projects/{projectUID}/tables/{tableUID}/records/{recordUID}/comments/{commentID} [delete]
func (schemalessRoute) DeleteComment(ctx context.Context, hub *collab.Hub, user *db.User, role db.ProjectRole, project *db.Project, table *db.SLTable, record *db.SLRecord, comment *db.SLRecordComment) error {
	own := comment.AuthorWorkflowID == 0 && comment.AuthorID == user.ID
	if !own && !role.AtLeast(db.ProjectRoleManager) {
		return ctx.ApiError(http.StatusForbidden, "comment::delete_denied")
	}
	if comment.DeletedAt == nil {
		if err := hub.DeleteComment(ctx.Request().Context(), project, table, record, comment.ID); err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete record comment")
			return ctx.ApiServerError()
		}
	}
	return ctx.Status(http.StatusNoContent)
}
