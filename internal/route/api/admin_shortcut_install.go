package api

import (
	stdcontext "context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/workflow"
	"github.com/wuhan005/sayrud/internal/workflow/sandbox"
)

// inspectFieldShortcut reads metadata without calling execute or giving the module network or AI access.
func inspectFieldShortcut(ctx stdcontext.Context, source string, language sandbox.Language) (*db.CustomFieldShortcut, error) {
	if strings.TrimSpace(source) == "" {
		return nil, i18n.Errorf("field_shortcut::code_required")
	}
	if len(source) > maxShortcutCodeSize {
		return nil, i18n.Errorf("field_shortcut::code_too_long")
	}
	name := "shortcut.js"
	if language == sandbox.TypeScript {
		name = "shortcut.ts"
	}
	program, err := sandbox.Compile(ctx, name, source, language)
	if err != nil {
		return nil, i18n.Errorf("workflow_plugin::compile_error", truncate(err.Error(), 500))
	}

	var f form.SaveFieldShortcut
	if err := json.Unmarshal(program.Manifest(), &f); err != nil {
		return nil, i18n.Errorf("workflow_plugin::invalid_manifest", truncate(err.Error(), 500))
	}
	// The module supplies definitions only; secrets are configured after installation.
	for i := range f.Credentials {
		f.Credentials[i].Value = ""
	}
	f.Code = source
	f.Language = string(language)
	f.Enabled = true
	s, _, err := buildCustomShortcut(f)
	return s, err
}

// InspectFieldShortcut
// @Summary Check a field shortcut before installing
// @Description Requires the admin. Download the source if a URL is given, then validate its exported manifest and execute function without running execute.
// @Accept json
// @Produce json
// @Param data body form.InspectFieldShortcut true "Source or URL"
// @Success 200 {object} dto.AdminFieldShortcut
// @Failure 400 {string} string "Invalid shortcut"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID inspectAdminFieldShortcut
// @Router /admin/field-shortcuts/inspect [post]
func (adminRoute) InspectFieldShortcut(ctx context.Context, f form.InspectFieldShortcut) error {
	source := f.Source
	language, err := pluginLanguage(f.Language)
	if err != nil {
		return saveFieldShortcutError(ctx, err, "inspect")
	}
	if strings.TrimSpace(source) == "" && strings.TrimSpace(f.URL) != "" {
		source, language, err = workflow.DownloadPluginSource(ctx.Request().Context(), f.URL, language)
		if err != nil {
			var downloadErr *workflow.DownloadError
			switch {
			case errors.Is(err, workflow.ErrInvalidPluginURL):
				return ctx.ApiError(http.StatusBadRequest, "workflow_plugin::invalid_url")
			case errors.As(err, &downloadErr):
				return ctx.ApiError(http.StatusBadRequest, "workflow_plugin::download_failed", downloadErr.Message)
			}
			return saveFieldShortcutError(ctx, err, "download")
		}
	}

	s, err := inspectFieldShortcut(ctx.Request().Context(), source, language)
	if err != nil {
		return saveFieldShortcutError(ctx, err, "inspect")
	}
	return ctx.ApiSuccess(dto.ToAdminFieldShortcut(s, nil, 0))
}

// InstallFieldShortcut
// @Summary Install a field shortcut from a JavaScript or TypeScript module
// @Description Requires the admin. Revalidate the reviewed source and create a new shortcut. Credential values must be configured separately.
// @Accept json
// @Produce json
// @Param data body form.InstallFieldShortcut true "Source"
// @Success 200 {object} dto.AdminFieldShortcut
// @Failure 400 {string} string "Invalid shortcut"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID installAdminFieldShortcut
// @Router /admin/field-shortcuts/install [post]
func (adminRoute) InstallFieldShortcut(ctx context.Context, f form.InstallFieldShortcut) error {
	language, err := pluginLanguage(f.Language)
	if err != nil {
		return saveFieldShortcutError(ctx, err, "validate")
	}
	s, err := inspectFieldShortcut(ctx.Request().Context(), f.Source, language)
	if err != nil {
		return saveFieldShortcutError(ctx, err, "validate")
	}
	if err := db.FieldShortcuts.Create(ctx.Request().Context(), s); err != nil {
		return saveFieldShortcutError(ctx, err, "install")
	}
	return ctx.ApiSuccess(dto.ToAdminFieldShortcut(s, nil, 0))
}
