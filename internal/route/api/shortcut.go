package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/shortcut"
)

var Shortcut shortcutRoute

type shortcutRoute struct{}

func toManifest(def *shortcut.Definition, aiConfigured bool) *dto.FieldShortcutManifest {
	return &dto.FieldShortcutManifest{
		ID:          def.ID,
		Kind:        string(def.Kind),
		AIEnabled:   def.AIEnabled,
		Name:        def.Name,
		Description: def.Description,
		ResultTypes: lo.Map(def.ResultTypes, func(t db.SLFieldType, _ int) string { return string(t) }),
		FormItems:   lo.Ternary(def.FormItems == nil, []db.ShortcutFormItem{}, def.FormItems),
		Available:   def.Available(aiConfigured),
	}
}

// shortcutErrorResponse responds the *shortcut.Error with 400, ok is false if err is not one.
func shortcutErrorResponse(ctx context.Context, err error) (error, bool) {
	var shortcutErr *shortcut.Error
	if !errors.As(err, &shortcutErr) {
		return nil, false
	}
	return ctx.ApiErrorMessage(http.StatusBadRequest, shortcutErr.Text(ctx.Tr)), true
}

// ListFieldShortcuts
// @Summary List field shortcuts
// @Description List the enabled custom shortcuts, with the forms to configure them.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Success 200 {array} dto.FieldShortcutManifest
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID listFieldShortcuts
// @Router /projects/{projectUID}/field-shortcuts [get]
func (shortcutRoute) List(ctx context.Context) error {
	defs, err := shortcut.Catalog(ctx.Request().Context())
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list field shortcuts")
		return ctx.ApiServerError()
	}

	aiConfigured := false
	if lo.ContainsBy(defs, func(def *shortcut.Definition) bool { return def.AIEnabled }) {
		settings, err := db.Settings.GetAI(ctx.Request().Context())
		if err != nil {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get AI settings")
			return ctx.ApiServerError()
		}
		aiConfigured = settings.Configured()
	}

	return ctx.ApiSuccess(lo.Map(defs, func(def *shortcut.Definition, _ int) *dto.FieldShortcutManifest {
		return toManifest(def, aiConfigured)
	}))
}

const maxPreviewRecords = 3

// Preview
// @Summary Preview a field shortcut
// @Description Execute the unsaved shortcut config of the field on at most 3 records, the results are not written back.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param data body form.PreviewFieldShortcut true "Field and shortcut config"
// @Success 200 {array} dto.ShortcutPreview
// @Failure 400 {string} string "Invalid config"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID previewFieldShortcut
// @Router /projects/{projectUID}/tables/{tableUID}/shortcuts/preview [post]
func (shortcutRoute) Preview(ctx context.Context, engine *shortcut.Engine, project *db.Project, table *db.SLTable, f form.PreviewFieldShortcut) error {
	c := ctx.Request().Context()
	if len(f.RecordUIDs) > maxPreviewRecords {
		return ctx.ApiError(http.StatusBadRequest, "shortcut::too_many_preview_records", maxPreviewRecords)
	}
	fields, err := db.SLFields.ListByTableID(c, table.ID)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list fields")
		return ctx.ApiServerError()
	}
	records, err := db.SLRecords.ListByUIDs(c, table.ID, f.RecordUIDs)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list records")
		return ctx.ApiServerError()
	}

	metadata := f.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	shortcutConfig := f.Shortcut
	draft := &db.SLField{
		UID:       f.FieldUID,
		SLTableID: table.ID,
		Type:      db.SLFieldType(f.Type),
		Metadata:  datatypes.NewJSONType[db.SLFieldMetadata](metadata),
		Shortcut:  &shortcutConfig,
	}
	if existing, ok := lo.Find(fields, func(field *db.SLField) bool { return field.UID == f.FieldUID }); ok {
		draft.Label = existing.Label
	}
	fields = append(lo.Filter(fields, func(field *db.SLField, _ int) bool { return field.UID != f.FieldUID }), draft)

	results, err := engine.Preview(c, project, table, fields, draft, records, ctx.Tr)
	if err != nil {
		if resp, ok := shortcutErrorResponse(ctx, err); ok {
			return resp
		}
		logrus.WithContext(c).WithError(err).Error("Failed to preview field shortcut")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(results)
}

// Run
// @Summary Generate the cells of a field shortcut
// @Description Queue the cells of the field to generate by its shortcut in the background, the progress is sent by the SHORTCUT_JOBS WebSocket messages.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param fieldUID path string true "Field UID"
// @Param data body form.RunFieldShortcut true "Cells to generate"
// @Success 200 {object} dto.RunFieldShortcutResp
// @Failure 400 {string} string "The field has no shortcut or the scope is invalid"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table or field not found"
// @Failure 500 {string} string "Internal server error"
// @ID runFieldShortcut
// @Router /projects/{projectUID}/tables/{tableUID}/fields/{fieldUID}/shortcut/run [post]
func (shortcutRoute) Run(ctx context.Context, engine *shortcut.Engine, project *db.Project, table *db.SLTable, field *db.SLField, f form.RunFieldShortcut) error {
	if field.Shortcut == nil {
		return ctx.ApiError(http.StatusBadRequest, "shortcut::field_without_shortcut")
	}
	scope := shortcut.RunScope(f.Scope)
	if scope != shortcut.RunScopeAll && scope != shortcut.RunScopeEmpty && scope != shortcut.RunScopeRecords {
		return ctx.ApiError(http.StatusBadRequest, "shortcut::invalid_scope")
	}
	if scope == shortcut.RunScopeRecords && len(f.RecordUIDs) > 10000 {
		return ctx.ApiError(http.StatusBadRequest, "record::too_many_records")
	}

	count, err := engine.Run(ctx.Request().Context(), project, table, field, scope, f.RecordUIDs)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to run field shortcut")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(dto.RunFieldShortcutResp{Count: count})
}

// ListJobs
// @Summary List the shortcut jobs
// @Description List the cells of the table being generated by the field shortcuts or failed to.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Success 200 {array} dto.ShortcutJob
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID listShortcutJobs
// @Router /projects/{projectUID}/tables/{tableUID}/shortcut-jobs [get]
func (shortcutRoute) ListJobs(ctx context.Context, engine *shortcut.Engine, table *db.SLTable) error {
	jobs, err := engine.ListJobs(ctx.Request().Context(), table, ctx.Tr)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list shortcut jobs")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(jobs)
}
