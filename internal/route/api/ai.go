// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"encoding/json"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/ai"
	"github.com/wuhan005/sayrud/internal/ai/hunyuan"
	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
)

var AI aiRoute

type aiRoute struct{}

// Advice
// @Summary Get table design advice from AI
// @Description Generate the advice of the action by AI from the conversation messages.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param data body form.AIAdvice true "Action and conversation messages"
// @Success 200 {object} dto.AIAdviceResp
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID aiAdvice
// @Router /projects/{projectUID}/ai/advice [post]
func (aiRoute) Advice(ctx context.Context, f form.AIAdvice) error {
	client := hunyuan.NewClient()

	messages := make([]*ai.Message, 0, len(f.Messages))
	for _, m := range f.Messages {
		messages = append(messages, &ai.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	resp, err := client.Advice(ctx.Request().Context(), f.Action, messages)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get advice")
		return ctx.ApiServerError()
	}

	return ctx.ApiSuccess(dto.AIAdviceResp{
		Raw:         resp.RawContent,
		Action:      resp.Action,
		ActionJSON:  resp.ActionJSON,
		Description: resp.Description,
	})
}

// Apply
// @Summary Apply the AI advice
// @Description Apply the actionJson returned by the advice API, e.g. create the advised tables.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param data body form.AIApply true "Action to apply"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid action or payload"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID aiApply
// @Router /projects/{projectUID}/ai/apply [post]
func (aiRoute) Apply(ctx context.Context, hub *collab.Hub, project *db.Project, tx dbutil.Transactor, f form.AIApply) error {
	switch f.Action {
	case ai.ActionTypeTables:
		var applyTables ai.ApplyTables
		if err := json.Unmarshal(f.ActionJson, &applyTables); err != nil {
			return ctx.ApiError(http.StatusBadRequest, "ai::invalid_json")
		}

		for _, table := range applyTables {
			if msg, ok := form.Validate(ctx.Locale(), table); !ok {
				return ctx.ApiErrorMessage(http.StatusBadRequest, msg)
			}
		}

		if err := tx.Transaction(func(tx *gorm.DB) error {
			slTablesStore := db.NewSLTablesStore(tx)

			for _, table := range applyTables {
				if _, err := slTablesStore.Create(ctx.Request().Context(), project.ID, db.CreateSLTableOptions{
					Name: table.TableLabel,
				}); err != nil {
					return errors.Wrap(err, "create")
				}
			}

			return nil
		}); err != nil {
			if errors.Is(err, db.ErrSLTableExists) {
				return ctx.ApiError(http.StatusBadRequest, "table::exists")
			}
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to apply tables")
			return ctx.ApiServerError()
		}

	default:
		return ctx.ApiError(http.StatusBadRequest, "ai::unknown_operation")
	}

	hub.NotifyProject(project.UID, collab.MessageTablesChanged)
	return ctx.Status(http.StatusNoContent)
}
