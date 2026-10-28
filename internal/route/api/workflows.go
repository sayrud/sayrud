package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/workflow"
)

var Workflow workflowRoute

type workflowRoute struct{}

// maxDraftSize limits the size of the draft JSON.
const maxDraftSize = 512 << 10

// Workflower injects the workflow of the workflowUID path parameter as *db.Workflow, the workflow must belong to the current project.
func (workflowRoute) Workflower(ctx context.Context, project *db.Project) error {
	wf, err := db.Workflows.GetByUID(ctx.Request().Context(), ctx.Param("workflowUID"))
	if err != nil {
		if errors.Is(err, db.ErrWorkflowNotFound) {
			return ctx.ApiError(http.StatusNotFound, "workflow::not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get workflow")
		return ctx.ApiServerError()
	}
	if wf.ProjectID != project.ID {
		return ctx.ApiError(http.StatusNotFound, "workflow::not_found")
	}
	ctx.Map(wf)
	return nil
}

func toWorkflow(ctx context.Context, engine *workflow.Engine, project *db.Project, wf *db.Workflow) (*dto.Workflow, error) {
	spec, err := workflow.ParseSpec(wf.Draft)
	if err != nil {
		spec, _ = workflow.ParseSpec(nil)
	}
	issues, err := workflow.Check(workflow.WithLanguage(ctx.Request().Context(), requestLang(ctx)), engine.DB(), project, spec)
	if err != nil {
		return nil, err
	}
	return &dto.Workflow{WorkflowListItem: *dto.ToWorkflowListItem(wf), Draft: spec, DraftVersion: wf.DraftVersion, Issues: issues}, nil
}

// ListWorkflows
// @Summary List workflows
// @Description List the workflows of the project ordered by sidebar position.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Success 200 {array} dto.WorkflowListItem
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID listWorkflows
// @Router /projects/{projectUID}/workflows [get]
func (workflowRoute) List(ctx context.Context, project *db.Project) error {
	list, err := db.Workflows.List(ctx.Request().Context(), project.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list workflows")
		return ctx.ApiServerError()
	}
	items := make([]*dto.WorkflowListItem, 0, len(list))
	for _, wf := range list {
		items = append(items, dto.ToWorkflowListItem(wf))
	}
	return ctx.ApiSuccess(items)
}

// uniquePath returns an API path not used by other workflows in the project.
func uniquePath(list []*db.Workflow, method, path string) string {
	used := map[string]bool{}
	for _, wf := range list {
		if spec, err := workflow.ParseSpec(wf.Draft); err == nil && spec.IsAPI() && spec.Trigger.API.Method == method {
			used[spec.Trigger.API.Path] = true
		}
		if wf.Method == method {
			used[wf.Path] = true
		}
	}
	candidate := path
	for n := 2; used[candidate]; n++ {
		candidate = path + "-" + strconv.Itoa(n)
	}
	return candidate
}

// CreateWorkflow
// @Summary Create a workflow
// @Description Create a workflow with the trigger, an empty trigger creates a blank workflow whose trigger is chosen later.
// @Description The API trigger starts from a blank template, or the draft which creates or queries the records of a table.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param data body form.CreateWorkflow true "Workflow to create"
// @Success 200 {object} dto.Workflow
// @Failure 400 {string} string "Invalid trigger, template or table"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID createWorkflow
// @Router /projects/{projectUID}/workflows [post]
func (workflowRoute) Create(ctx context.Context, engine *workflow.Engine, user *db.User, project *db.Project, f form.CreateWorkflow) error {
	rctx := ctx.Request().Context()
	triggerType := f.TriggerType
	if triggerType != "" && !contains(workflow.TriggerTypes, triggerType) {
		return ctx.ApiError(http.StatusBadRequest, "workflow::invalid_trigger")
	}

	// The table templates, record triggers and date triggers require a table in the project.
	var table *db.SLTable
	var fields []*db.SLField
	needsTable := triggerType != "" && triggerType != workflow.TriggerSchedule && (triggerType != workflow.TriggerAPI || f.Template == workflow.TemplateCreate || f.Template == workflow.TemplateQuery)
	if needsTable {
		var err error
		table, err = db.SLTables.GetByUID(rctx, f.TableUID)
		if err != nil || table.ProjectID != project.ID {
			if err == nil || errors.Is(err, db.ErrSLTableNotFound) {
				return ctx.ApiError(http.StatusBadRequest, "workflow::table_not_found")
			}
			logrus.WithContext(rctx).WithError(err).Error("Failed to get table")
			return ctx.ApiServerError()
		}
		if fields, err = db.SLFields.ListByTableID(rctx, table.ID); err != nil {
			logrus.WithContext(rctx).WithError(err).Error("Failed to list fields")
			return ctx.ApiServerError()
		}
	}

	var spec *workflow.Spec
	switch {
	case triggerType == "":
		spec, _ = workflow.ParseSpec(nil)
	case triggerType != workflow.TriggerAPI:
		spec = workflow.TriggerSpec(triggerType, table, fields, f.Timezone, dbutil.Now())
	case f.Template == "" || f.Template == workflow.TemplateBlank:
		spec = workflow.BlankSpec("/hello")
	case f.Template == workflow.TemplateCreate || f.Template == workflow.TemplateQuery:
		spec = workflow.TableSpec(f.Template, table, fields)
	default:
		return ctx.ApiError(http.StatusBadRequest, "workflow::invalid_template")
	}

	if f.Timezone != "" {
		if _, err := time.LoadLocation(f.Timezone); err == nil {
			spec.Timezone = f.Timezone
		}
	}

	if spec.IsAPI() {
		existing, err := db.Workflows.List(rctx, project.ID)
		if err != nil {
			logrus.WithContext(rctx).WithError(err).Error("Failed to list workflows")
			return ctx.ApiServerError()
		}
		spec.Trigger.API.Path = uniquePath(existing, spec.Trigger.API.Method, spec.Trigger.API.Path)
	}
	raw, _ := json.Marshal(spec)
	wf, err := db.Workflows.Create(rctx, db.CreateWorkflowOptions{
		ProjectID: project.ID, Name: strings.TrimSpace(f.Name), Description: strings.TrimSpace(f.Description), Draft: raw, UserID: user.ID,
	})
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to create workflow")
		return ctx.ApiServerError()
	}
	resp, err := toWorkflow(ctx, engine, project, wf)
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to check workflow")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(resp)
}

// GetWorkflow
// @Summary Get a workflow
// @Description Return the workflow with its draft and the issues to fix before publishing.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Success 200 {object} dto.Workflow
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or workflow not found"
// @Failure 500 {string} string "Internal server error"
// @ID getWorkflow
// @Router /projects/{projectUID}/workflows/{workflowUID} [get]
func (workflowRoute) Get(ctx context.Context, engine *workflow.Engine, project *db.Project, wf *db.Workflow) error {
	resp, err := toWorkflow(ctx, engine, project, wf)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to check workflow")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(resp)
}

// SaveWorkflowDraft
// @Summary Save the draft of a workflow
// @Description Save the draft based on draftVersion, it responds 409 if others saved the draft since. The settings unrelated to the trigger are dropped.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Param data body form.SaveWorkflowDraft true "Draft"
// @Success 200 {object} dto.SaveWorkflowDraftResp
// @Failure 400 {string} string "Draft too large"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or workflow not found"
// @Failure 409 {string} string "Draft saved by others"
// @Failure 500 {string} string "Internal server error"
// @ID saveWorkflowDraft
// @Router /projects/{projectUID}/workflows/{workflowUID} [put]
func (workflowRoute) SaveDraft(ctx context.Context, engine *workflow.Engine, user *db.User, project *db.Project, wf *db.Workflow, f form.SaveWorkflowDraft) error {
	rctx := ctx.Request().Context()
	spec := f.Draft
	spec.Normalize()
	raw, err := json.Marshal(spec)
	if err != nil || len(raw) > maxDraftSize {
		return ctx.ApiError(http.StatusBadRequest, "workflow::draft_too_large")
	}
	version, err := db.Workflows.UpdateDraft(rctx, wf.ID, f.DraftVersion, db.UpdateWorkflowDraftOptions{
		Name: strings.TrimSpace(f.Name), Description: strings.TrimSpace(f.Description), Draft: raw, UserID: user.ID,
	})
	if err != nil {
		if errors.Is(err, db.ErrWorkflowVersionConflict) {
			return ctx.ApiError(http.StatusConflict, "workflow::version_conflict")
		}
		logrus.WithContext(rctx).WithError(err).Error("Failed to save workflow draft")
		return ctx.ApiServerError()
	}
	issues, err := workflow.Check(workflow.WithLanguage(rctx, requestLang(ctx)), engine.DB(), project, &spec)
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to check workflow")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.SaveWorkflowDraftResp{DraftVersion: version, Issues: issues, UpdatedAt: dbutil.Now()})
}

// DeleteWorkflow
// @Summary Delete a workflow
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or workflow not found"
// @Failure 500 {string} string "Internal server error"
// @ID deleteWorkflow
// @Router /projects/{projectUID}/workflows/{workflowUID} [delete]
func (workflowRoute) Delete(ctx context.Context, wf *db.Workflow) error {
	if err := db.Workflows.DeleteByID(ctx.Request().Context(), wf.ID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete workflow")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// SetWorkflowEnabled
// @Summary Enable or disable a workflow
// @Description A disabled workflow ignores its triggers. The schedule restarts from now when it is enabled again, the missed times are skipped.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Param data body form.SetWorkflowEnabled true "Enabled"
// @Success 204 "No Content"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or workflow not found"
// @Failure 500 {string} string "Internal server error"
// @ID setWorkflowEnabled
// @Router /projects/{projectUID}/workflows/{workflowUID}/enabled [put]
func (workflowRoute) SetEnabled(ctx context.Context, engine *workflow.Engine, wf *db.Workflow, f form.SetWorkflowEnabled) error {
	rctx := ctx.Request().Context()
	var schedule *db.WorkflowSchedule
	if f.Enabled && !wf.Enabled && (wf.TriggerType == workflow.TriggerSchedule || wf.TriggerType == workflow.TriggerDateReached) {
		spec, err := engine.PublishedSpec(rctx, wf.PublishedRevisionID)
		if err != nil {
			logrus.WithContext(rctx).WithError(err).Error("Failed to load workflow revision")
			return ctx.ApiServerError()
		}
		state := workflow.ScheduleState(spec, wf, dbutil.Now())
		schedule = &state
	}
	if err := db.Workflows.SetEnabled(rctx, wf.ID, f.Enabled, schedule); err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to set workflow enabled")
		return ctx.ApiServerError()
	}
	engine.NotifySchedule()
	return ctx.Status(http.StatusNoContent)
}

// PublishWorkflow
// @Summary Publish a workflow
// @Description Freeze the draft as a new immutable version which handles the triggers. Nothing is published if issues are returned.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Param data body form.PublishWorkflow true "Draft version"
// @Success 200 {object} dto.PublishWorkflowResp
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or workflow not found"
// @Failure 409 {string} string "Draft changed or route conflict"
// @Failure 500 {string} string "Internal server error"
// @ID publishWorkflow
// @Router /projects/{projectUID}/workflows/{workflowUID}/publish [post]
func (workflowRoute) Publish(ctx context.Context, engine *workflow.Engine, user *db.User, project *db.Project, wf *db.Workflow, f form.PublishWorkflow) error {
	rctx := ctx.Request().Context()
	if f.DraftVersion != wf.DraftVersion {
		return ctx.ApiError(http.StatusConflict, "workflow::version_conflict")
	}
	spec, err := workflow.ParseSpec(wf.Draft)
	if err != nil {
		return ctx.ApiError(http.StatusBadRequest, "workflow::invalid_draft")
	}
	issues, err := workflow.Check(workflow.WithLanguage(rctx, requestLang(ctx)), engine.DB(), project, spec)
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to check workflow")
		return ctx.ApiServerError()
	}
	if len(issues) > 0 {
		return ctx.ApiSuccess(dto.PublishWorkflowResp{Issues: issues})
	}
	return publishSpec(ctx, engine, user, project, wf, spec, f.DraftVersion, 0)
}

// publishSpec publishes the checked spec as a new version, restoredFrom is the version being rolled back to or 0.
func publishSpec(ctx context.Context, engine *workflow.Engine, user *db.User, project *db.Project, wf *db.Workflow, spec *workflow.Spec, draftVersion int64, restoredFrom int) error {
	rctx := ctx.Request().Context()
	opts, err := workflow.PublishOptions(rctx, engine.DB(), spec, draftVersion, user.ID, dbutil.Now())
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to prepare workflow publishing")
		return ctx.ApiServerError()
	}
	opts.RestoredFrom = restoredFrom
	if _, err := db.Workflows.Publish(rctx, wf.ID, opts); err != nil {
		switch {
		case errors.Is(err, db.ErrWorkflowVersionConflict):
			return ctx.ApiError(http.StatusConflict, "workflow::version_conflict")
		case errors.Is(err, db.ErrWorkflowRouteConflict):
			return ctx.ApiError(http.StatusConflict, "workflow::route_conflict", opts.Method, opts.Path)
		}
		logrus.WithContext(rctx).WithError(err).Error("Failed to publish workflow")
		return ctx.ApiServerError()
	}
	engine.NotifySchedule()

	published, err := db.Workflows.GetByUID(rctx, wf.UID)
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to get workflow")
		return ctx.ApiServerError()
	}
	resp, err := toWorkflow(ctx, engine, project, published)
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to check workflow")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.PublishWorkflowResp{Issues: []workflow.Issue{}, Workflow: resp})
}

// RollbackWorkflow
// @Summary Roll back a workflow to a published version
// @Description Republish the spec of the version as a new version, the draft is overwritten by it.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Param version path int true "Version to roll back to"
// @Param data body form.RollbackWorkflow true "Draft version"
// @Success 200 {object} dto.PublishWorkflowResp
// @Failure 400 {string} string "The version references deleted tables, fields or credentials"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, workflow or version not found"
// @Failure 409 {string} string "Draft changed or route conflict"
// @Failure 500 {string} string "Internal server error"
// @ID rollbackWorkflow
// @Router /projects/{projectUID}/workflows/{workflowUID}/revisions/{version}/rollback [post]
func (workflowRoute) Rollback(ctx context.Context, engine *workflow.Engine, user *db.User, project *db.Project, wf *db.Workflow, f form.RollbackWorkflow) error {
	rctx := ctx.Request().Context()
	if f.DraftVersion != wf.DraftVersion {
		return ctx.ApiError(http.StatusConflict, "workflow::version_conflict")
	}
	revision, err := db.WorkflowRevisions.GetByVersion(rctx, wf.ID, int(ctx.ParamInt64("version")))
	if err != nil {
		if errors.Is(err, db.ErrWorkflowRevisionNotFound) {
			return ctx.ApiError(http.StatusNotFound, "workflow::revision_not_found")
		}
		logrus.WithContext(rctx).WithError(err).Error("Failed to get workflow revision")
		return ctx.ApiServerError()
	}
	spec, err := workflow.ParseSpec(revision.Spec)
	if err != nil {
		return ctx.ApiError(http.StatusBadRequest, "workflow::invalid_draft")
	}
	// The tables, fields or credentials referenced by the version may have been deleted since.
	issues, err := workflow.Check(workflow.WithLanguage(rctx, requestLang(ctx)), engine.DB(), project, spec)
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to check workflow")
		return ctx.ApiServerError()
	}
	if len(issues) > 0 {
		return ctx.ApiError(http.StatusBadRequest, "workflow::revision_invalid")
	}
	return publishSpec(ctx, engine, user, project, wf, spec, f.DraftVersion, revision.Version)
}

// DebugWorkflow
// @Summary Debug a workflow
// @Description Run the draft. The API trigger runs with the request; the other triggers run with the record (the latest one if not specified) or the current time.
// @Description dry_run rolls back the writes and skips the asynchronous flows and external actions, live runs them for real and is recorded as a debug run.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Param data body form.DebugWorkflow true "Request"
// @Success 200 {object} dto.DebugWorkflowResp
// @Failure 400 {string} string "Invalid request body or no record to debug with"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or workflow not found"
// @Failure 500 {string} string "Internal server error"
// @ID debugWorkflow
// @Router /projects/{projectUID}/workflows/{workflowUID}/debug [post]
func (workflowRoute) Debug(ctx context.Context, engine *workflow.Engine, user *db.User, project *db.Project, wf *db.Workflow, f form.DebugWorkflow) error {
	rctx := ctx.Request().Context()
	spec := f.Draft
	if spec == nil {
		var err error
		if spec, err = workflow.ParseSpec(wf.Draft); err != nil {
			return ctx.ApiError(http.StatusBadRequest, "workflow::invalid_draft")
		}
	} else {
		spec.Normalize()
	}
	issues, err := workflow.Check(workflow.WithLanguage(rctx, requestLang(ctx)), engine.DB(), project, spec)
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to check workflow")
		return ctx.ApiServerError()
	}

	if !spec.IsAPI() {
		trigger, err := engine.SampleTrigger(rctx, project, spec, f.RecordUID)
		if err != nil {
			if errors.Is(err, workflow.ErrNoSampleRecord) {
				return ctx.ApiError(http.StatusBadRequest, "workflow::no_sample_record")
			}
			logrus.WithContext(rctx).WithError(err).Error("Failed to prepare workflow debug")
			return ctx.ApiServerError()
		}
		result, err := engine.Debug(rctx, workflow.DebugInput{
			Project: project, Workflow: wf, Spec: spec, Trigger: trigger, UserID: user.ID, DryRun: f.Mode != "live",
		})
		if err != nil {
			logrus.WithContext(rctx).WithError(err).Error("Failed to debug workflow")
			return ctx.ApiServerError()
		}
		return ctx.ApiSuccess(dto.DebugWorkflowResp{RunResult: *result, Issues: issues})
	}

	path, ok := workflow.NormalizePath(spec.Trigger.API.Path)
	if !ok {
		path = spec.Trigger.API.Path
	}
	query := url.Values{}
	for k, v := range f.Query {
		query.Set(k, v)
	}
	header := http.Header{}
	for k, v := range f.Headers {
		header.Set(k, v)
	}
	raw := workflow.RawInput{Query: query, Header: header}
	body := []byte(strings.TrimSpace(string(f.Body)))
	if len(body) > 0 && string(body) != "null" {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(body, &obj); err != nil || obj == nil {
			return ctx.ApiError(http.StatusBadRequest, "workflow::invalid_debug_body")
		}
		raw.Body = obj
	} else {
		body = nil
	}

	result, err := engine.Run(rctx, workflow.RunInput{
		Project: project, Workflow: wf, Spec: spec, Raw: raw, Trigger: db.WorkflowRunTriggerDebug, UserID: user.ID,
		Snapshot: workflow.NewSnapshot(spec.Trigger.API.Method, path, ctx.IP(), query, header, body),
		DryRun:   f.Mode != "live",
	})
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to debug workflow")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.DebugWorkflowResp{RunResult: *result, Issues: issues})
}

// ListWorkflowRevisions
// @Summary List the published versions of a workflow
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Success 200 {array} dto.WorkflowRevision
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or workflow not found"
// @Failure 500 {string} string "Internal server error"
// @ID listWorkflowRevisions
// @Router /projects/{projectUID}/workflows/{workflowUID}/revisions [get]
func (workflowRoute) ListRevisions(ctx context.Context, wf *db.Workflow) error {
	revisions, err := db.WorkflowRevisions.ListByWorkflowID(ctx.Request().Context(), wf.ID, 50)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list workflow revisions")
		return ctx.ApiServerError()
	}
	userIDs := make([]int64, 0, len(revisions))
	for _, r := range revisions {
		userIDs = append(userIDs, r.CreatedBy)
	}
	users, err := usersByID(ctx, userIDs)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list users")
		return ctx.ApiServerError()
	}
	out := make([]*dto.WorkflowRevision, 0, len(revisions))
	for _, r := range revisions {
		item := &dto.WorkflowRevision{Version: r.Version, TriggerType: r.TriggerType, Method: r.Method, Path: r.Path, RestoredFrom: r.RestoredFrom, CreatedAt: r.CreatedAt}
		if u, ok := users[r.CreatedBy]; ok {
			item.CreatedBy = dto.ToUserBrief(u)
		}
		out = append(out, item)
	}
	return ctx.ApiSuccess(out)
}

// ListWorkflowRunsResp is the list of runs.
type ListWorkflowRunsResp struct {
	Runs []*dto.WorkflowRun `json:"runs"`
	// NextCursor is passed as cursor to get the earlier runs, 0 means there are no more.
	NextCursor int64 `json:"nextCursor"`
} // @name ListWorkflowRunsResp

// ListWorkflowRuns
// @Summary List the runs of a workflow
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Param status query string false "Filter by the comma-separated statuses: queued, running, succeeded, failed, rejected"
// @Param minDuration query int false "Return the finished runs taking at least the milliseconds"
// @Param maxDuration query int false "Return the finished runs taking less than the milliseconds"
// @Param from query string false "Return the runs created at or after the RFC 3339 time"
// @Param to query string false "Return the runs created before the RFC 3339 time"
// @Param cursor query int false "Return the runs older than the cursor"
// @Success 200 {object} ListWorkflowRunsResp
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or workflow not found"
// @Failure 500 {string} string "Internal server error"
// @ID listWorkflowRuns
// @Router /projects/{projectUID}/workflows/{workflowUID}/runs [get]
func (workflowRoute) ListRuns(ctx context.Context, wf *db.Workflow) error {
	rctx := ctx.Request().Context()
	const limit = 30
	opts := db.ListWorkflowRunsOptions{
		MinDurationMs: max(int64(ctx.QueryInt64("minDuration")), 0),
		MaxDurationMs: max(int64(ctx.QueryInt64("maxDuration")), 0),
		BeforeID:      int64(ctx.QueryInt64("cursor")),
		Limit:         limit,
	}
	for _, s := range strings.Split(ctx.Query("status"), ",") {
		switch status := db.WorkflowRunStatus(strings.TrimSpace(s)); status {
		case db.WorkflowRunQueued, db.WorkflowRunRunning, db.WorkflowRunSucceeded, db.WorkflowRunFailed, db.WorkflowRunRejected:
			opts.Statuses = append(opts.Statuses, status)
		}
	}
	// Malformed times are ignored.
	if t, err := time.Parse(time.RFC3339, ctx.Query("from")); err == nil {
		opts.CreatedFrom = t
	}
	if t, err := time.Parse(time.RFC3339, ctx.Query("to")); err == nil {
		opts.CreatedTo = t
	}
	runs, err := db.WorkflowRuns.List(rctx, wf.ID, opts)
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to list workflow runs")
		return ctx.ApiServerError()
	}
	ids := make([]int64, 0, len(runs))
	for _, r := range runs {
		ids = append(ids, r.ID)
	}
	jobs, err := db.WorkflowJobs.ListByRunIDs(rctx, ids)
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to list workflow jobs")
		return ctx.ApiServerError()
	}
	resp := ListWorkflowRunsResp{Runs: make([]*dto.WorkflowRun, 0, len(runs))}
	for _, r := range runs {
		resp.Runs = append(resp.Runs, dto.ToWorkflowRun(r, jobs, false))
	}
	if len(runs) == limit {
		resp.NextCursor = runs[len(runs)-1].ID
	}
	return ctx.ApiSuccess(resp)
}

// GetWorkflowRun
// @Summary Get a run of a workflow
// @Description Return the request or the trigger data, the response, the step trace and the background jobs of the run.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Param runUID path string true "Run UID"
// @Success 200 {object} dto.WorkflowRun
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, workflow or run not found"
// @Failure 500 {string} string "Internal server error"
// @ID getWorkflowRun
// @Router /projects/{projectUID}/workflows/{workflowUID}/runs/{runUID} [get]
func (workflowRoute) GetRun(ctx context.Context, wf *db.Workflow) error {
	rctx := ctx.Request().Context()
	run, err := db.WorkflowRuns.GetByUID(rctx, wf.ID, ctx.Param("runUID"))
	if err != nil {
		if errors.Is(err, db.ErrWorkflowRunNotFound) {
			return ctx.ApiError(http.StatusNotFound, "workflow::run_not_found")
		}
		logrus.WithContext(rctx).WithError(err).Error("Failed to get workflow run")
		return ctx.ApiServerError()
	}
	jobs, err := db.WorkflowJobs.ListByRunIDs(rctx, []int64{run.ID})
	if err != nil {
		logrus.WithContext(rctx).WithError(err).Error("Failed to list workflow jobs")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.ToWorkflowRun(run, jobs, true))
}

// RetryWorkflowJob
// @Summary Retry a failed background job
// @Description Run the failed job again, the steps which succeeded are not run again.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Param jobID path int true "Job ID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, workflow or failed job not found"
// @Failure 500 {string} string "Internal server error"
// @ID retryWorkflowJob
// @Router /projects/{projectUID}/workflows/{workflowUID}/jobs/{jobID}/retry [post]
func (workflowRoute) RetryJob(ctx context.Context, engine *workflow.Engine, wf *db.Workflow) error {
	ok, err := engine.Requeue(ctx.Request().Context(), wf.ID, ctx.ParamInt64("jobID"))
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to retry workflow job")
		return ctx.ApiServerError()
	}
	if !ok {
		return ctx.ApiError(http.StatusNotFound, "workflow::job_not_found")
	}
	return ctx.Status(http.StatusNoContent)
}

// ListWorkflowKeys
// @Summary List the API keys of a workflow
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Success 200 {array} dto.WorkflowKey
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or workflow not found"
// @Failure 500 {string} string "Internal server error"
// @ID listWorkflowKeys
// @Router /projects/{projectUID}/workflows/{workflowUID}/keys [get]
func (workflowRoute) ListKeys(ctx context.Context, wf *db.Workflow) error {
	keys, err := db.WorkflowKeys.List(ctx.Request().Context(), wf.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list workflow keys")
		return ctx.ApiServerError()
	}
	out := make([]*dto.WorkflowKey, 0, len(keys))
	for _, k := range keys {
		out = append(out, dto.ToWorkflowKey(k))
	}
	return ctx.ApiSuccess(out)
}

// CreateWorkflowKey
// @Summary Create an API key of a workflow
// @Description The secret is only returned once.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Param data body form.CreateWorkflowKey true "Key"
// @Success 200 {object} dto.CreateWorkflowKeyResp
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or workflow not found"
// @Failure 500 {string} string "Internal server error"
// @ID createWorkflowKey
// @Router /projects/{projectUID}/workflows/{workflowUID}/keys [post]
func (workflowRoute) CreateKey(ctx context.Context, user *db.User, wf *db.Workflow, f form.CreateWorkflowKey) error {
	key, secret, err := db.WorkflowKeys.Create(ctx.Request().Context(), wf.ID, strings.TrimSpace(f.Name), user.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create workflow key")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.CreateWorkflowKeyResp{Key: dto.ToWorkflowKey(key), Secret: secret})
}

// DeleteWorkflowKey
// @Summary Delete an API key of a workflow
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param workflowUID path string true "Workflow UID"
// @Param keyID path int true "Key ID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or workflow not found"
// @Failure 500 {string} string "Internal server error"
// @ID deleteWorkflowKey
// @Router /projects/{projectUID}/workflows/{workflowUID}/keys/{keyID} [delete]
func (workflowRoute) DeleteKey(ctx context.Context, wf *db.Workflow) error {
	if err := db.WorkflowKeys.Delete(ctx.Request().Context(), wf.ID, ctx.ParamInt64("keyID")); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete workflow key")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// checkWorkflowReference returns a localized error if the identifier is referenced by a published workflow, the argument of key is the workflow name.
func checkWorkflowReference(ctx context.Context, store db.WorkflowsStore, project *db.Project, uid, key string) error {
	wf, err := store.FindReference(ctx.Request().Context(), project.ID, uid)
	if err != nil {
		if errors.Is(err, db.ErrWorkflowNotFound) {
			return nil
		}
		return errors.Wrap(err, "find workflow reference")
	}
	return i18n.Errorf(key, wf.Name)
}

// isLocalized reports whether the error chain contains an *i18n.Error to be translated.
func isLocalized(err error) bool {
	var e *i18n.Error
	return errors.As(err, &e)
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
