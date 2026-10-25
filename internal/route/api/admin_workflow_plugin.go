package api

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/workflow"
	"github.com/wuhan005/sayrud/internal/workflow/sandbox"
)

// requestLang returns the language of the request, English without a locale.
func requestLang(ctx context.Context) string {
	if l := ctx.Locale(); l != nil {
		return l.Lang()
	}
	return i18n.LangEn
}

// WorkflowPluginer maps the installed plugin of the pluginID path parameter as *db.WorkflowPlugin, and responds 404 if not found.
func (adminRoute) WorkflowPluginer(ctx context.Context) error {
	p, err := db.WorkflowPlugins.GetByPluginID(ctx.Request().Context(), ctx.Param("pluginID"))
	if err != nil {
		if errors.Is(err, db.ErrWorkflowPluginNotFound) {
			return ctx.ApiError(http.StatusNotFound, "workflow_plugin::not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get workflow plugin")
		return ctx.ApiServerError()
	}
	ctx.Map(p)
	return nil
}

// toWorkflowPlugin keeps plugins with an invalid stored manifest listed so that they can be removed.
func toWorkflowPlugin(ctx context.Context, p *db.WorkflowPlugin) *dto.WorkflowPlugin {
	m, err := workflow.ParsePluginManifest(p.Manifest)
	if err != nil {
		m = &workflow.PluginManifest{ID: p.PluginID, Version: p.Version, Name: workflow.LocalText{"": p.Name}, Icon: "puzzle", Color: "#8f959e", Category: "integration", Flows: []string{}, Defaults: []byte("{}")}
	}
	return dto.ToWorkflowPlugin(p, m, requestLang(ctx))
}

// ListWorkflowPlugins
// @Summary List the workflow node plugins
// @Description Requires the admin. The built-in nodes are listed too, their texts are frontend i18n keys.
// @Produce json
// @Success 200 {object} dto.ListWorkflowPluginsResp
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID listAdminWorkflowPlugins
// @Router /admin/workflow-plugins [get]
func (adminRoute) ListWorkflowPlugins(ctx context.Context) error {
	list, err := db.WorkflowPlugins.List(ctx.Request().Context())
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list workflow plugins")
		return ctx.ApiServerError()
	}
	resp := dto.ListWorkflowPluginsResp{Plugins: make([]*dto.WorkflowPlugin, 0, len(list)), Builtins: workflow.Modules()}
	for _, p := range list {
		resp.Plugins = append(resp.Plugins, toWorkflowPlugin(ctx, p))
	}
	return ctx.ApiSuccess(resp)
}

// GetWorkflowPlugin
// @Summary Get a workflow node plugin with its source
// @Description Requires the admin.
// @Produce json
// @Param pluginID path string true "Plugin ID"
// @Success 200 {object} dto.WorkflowPlugin
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Plugin not found"
// @Failure 500 {string} string "Internal server error"
// @ID getAdminWorkflowPlugin
// @Router /admin/workflow-plugins/{pluginID} [get]
func (adminRoute) GetWorkflowPlugin(ctx context.Context, p *db.WorkflowPlugin) error {
	resp := toWorkflowPlugin(ctx, p)
	resp.Source = p.Source
	return ctx.ApiSuccess(resp)
}

// pluginLanguage returns the source language of the form, JavaScript by default.
func pluginLanguage(lang string) (sandbox.Language, error) {
	if lang == "" {
		return sandbox.JavaScript, nil
	}
	if l := sandbox.Language(lang); l.Valid() {
		return l, nil
	}
	return "", i18n.Errorf("workflow_plugin::invalid_language")
}

// inspectPlugin compiles the source and validates its manifest, errors to show are *i18n.Error.
func inspectPlugin(ctx context.Context, source string, lang sandbox.Language) (*sandbox.Program, *workflow.PluginManifest, error) {
	if strings.TrimSpace(source) == "" {
		return nil, nil, i18n.Errorf("workflow_plugin::source_required")
	}
	if len(source) > sandbox.MaxSourceSize {
		return nil, nil, i18n.Errorf("workflow_plugin::source_too_large", sandbox.MaxSourceSize>>10)
	}
	program, m, err := workflow.InspectPlugin(ctx.Request().Context(), source, lang)
	if err != nil {
		var manifestErr *workflow.ManifestError
		if errors.As(err, &manifestErr) {
			return nil, nil, i18n.Errorf("workflow_plugin::invalid_manifest", manifestErr.Message)
		}
		return nil, nil, i18n.Errorf("workflow_plugin::compile_error", truncate(err.Error(), 500))
	}
	return program, m, nil
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

func respondPluginError(ctx context.Context, err error, action string) error {
	if isLocalized(err) {
		return ctx.ApiErrorFrom(http.StatusBadRequest, err)
	}
	logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to " + action + " workflow plugin")
	return ctx.ApiServerError()
}

// installedPlugin returns the installed plugin of the id, nil if it is not installed.
func installedPlugin(ctx context.Context, id string) (*dto.WorkflowPlugin, error) {
	saved, err := db.WorkflowPlugins.GetByPluginID(ctx.Request().Context(), id)
	if errors.Is(err, db.ErrWorkflowPluginNotFound) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return toWorkflowPlugin(ctx, saved), nil
}

// InspectWorkflowPlugin
// @Summary Check a workflow node plugin before installing
// @Description Requires the admin. Compile the source, or download it from the URL first, and validate the manifest without installing.
// @Accept json
// @Produce json
// @Param data body form.InspectWorkflowPlugin true "Source or URL"
// @Success 200 {object} dto.InspectWorkflowPluginResp
// @Failure 400 {string} string "Invalid plugin"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID inspectAdminWorkflowPlugin
// @Router /admin/workflow-plugins/inspect [post]
func (adminRoute) InspectWorkflowPlugin(ctx context.Context, f form.InspectWorkflowPlugin) error {
	source := f.Source
	lang, err := pluginLanguage(f.Language)
	if err != nil {
		return respondPluginError(ctx, err, "inspect")
	}
	if strings.TrimSpace(source) == "" && strings.TrimSpace(f.URL) != "" {
		source, lang, err = workflow.DownloadPluginSource(ctx.Request().Context(), f.URL, lang)
		if err != nil {
			var downloadErr *workflow.DownloadError
			switch {
			case errors.Is(err, workflow.ErrInvalidPluginURL):
				return ctx.ApiError(http.StatusBadRequest, "workflow_plugin::invalid_url")
			case errors.As(err, &downloadErr):
				return ctx.ApiError(http.StatusBadRequest, "workflow_plugin::download_failed", downloadErr.Message)
			}
			return respondPluginError(ctx, err, "download")
		}
	}
	program, m, err := inspectPlugin(ctx, source, lang)
	if err != nil {
		return respondPluginError(ctx, err, "inspect")
	}
	installed, err := installedPlugin(ctx, m.ID)
	if err != nil {
		return respondPluginError(ctx, err, "get")
	}
	preview := &db.WorkflowPlugin{PluginID: m.ID, Version: m.Version, Language: string(lang), SourceURL: strings.TrimSpace(f.URL), Checksum: workflow.PluginChecksum(source), Enabled: true}
	if installed != nil {
		preview.Enabled = installed.Enabled
	}
	return ctx.ApiSuccess(dto.InspectWorkflowPluginResp{
		Plugin: dto.ToWorkflowPlugin(preview, m, requestLang(ctx)), Installed: installed,
		Source: source, Language: string(lang), HasPreview: program.Has(sandbox.HookPreview),
	})
}

// InstallWorkflowPlugin
// @Summary Install or upgrade a workflow node plugin
// @Description Requires the admin. A plugin with the same manifest id is replaced and keeps its enabled state; the workflows use the new version right away.
// @Accept json
// @Produce json
// @Param data body form.InstallWorkflowPlugin true "Source"
// @Success 200 {object} dto.WorkflowPlugin
// @Failure 400 {string} string "Invalid plugin"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID installAdminWorkflowPlugin
// @Router /admin/workflow-plugins [post]
func (adminRoute) InstallWorkflowPlugin(ctx context.Context, f form.InstallWorkflowPlugin) error {
	lang, err := pluginLanguage(f.Language)
	if err != nil {
		return respondPluginError(ctx, err, "install")
	}
	program, m, err := inspectPlugin(ctx, f.Source, lang)
	if err != nil {
		return respondPluginError(ctx, err, "install")
	}
	p := &db.WorkflowPlugin{
		PluginID: m.ID, Name: m.Name.In(i18n.LangEn), Version: m.Version, Language: string(lang), Source: f.Source,
		SourceURL: strings.TrimSpace(f.SourceURL), Checksum: workflow.PluginChecksum(f.Source), Manifest: datatypes.JSON(program.Manifest()), Enabled: true,
	}
	if err := db.WorkflowPlugins.Save(ctx.Request().Context(), p); err != nil {
		return respondPluginError(ctx, err, "save")
	}
	workflow.SyncPlugins(ctx.Request().Context(), true)
	return ctx.ApiSuccess(toWorkflowPlugin(ctx, p))
}

// SetWorkflowPluginEnabled
// @Summary Enable or disable a workflow node plugin
// @Description Requires the admin. A disabled node can not be added, published or executed, its steps are kept.
// @Accept json
// @Produce json
// @Param pluginID path string true "Plugin ID"
// @Param data body form.SetWorkflowPluginEnabled true "Enabled"
// @Success 200 {object} dto.WorkflowPlugin
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Plugin not found"
// @Failure 500 {string} string "Internal server error"
// @ID setAdminWorkflowPluginEnabled
// @Router /admin/workflow-plugins/{pluginID}/enabled [put]
func (adminRoute) SetWorkflowPluginEnabled(ctx context.Context, p *db.WorkflowPlugin, f form.SetWorkflowPluginEnabled) error {
	if err := db.WorkflowPlugins.SetEnabled(ctx.Request().Context(), p.PluginID, f.Enabled); err != nil {
		if errors.Is(err, db.ErrWorkflowPluginNotFound) {
			return ctx.ApiError(http.StatusNotFound, "workflow_plugin::not_found")
		}
		return respondPluginError(ctx, err, "update")
	}
	workflow.SyncPlugins(ctx.Request().Context(), true)
	p.Enabled = f.Enabled
	return ctx.ApiSuccess(toWorkflowPlugin(ctx, p))
}

// UninstallWorkflowPlugin
// @Summary Uninstall a workflow node plugin
// @Description Requires the admin. The workflows using the node keep their steps, which fail until the plugin is installed again.
// @Produce json
// @Param pluginID path string true "Plugin ID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Plugin not found"
// @Failure 500 {string} string "Internal server error"
// @ID uninstallAdminWorkflowPlugin
// @Router /admin/workflow-plugins/{pluginID} [delete]
func (adminRoute) UninstallWorkflowPlugin(ctx context.Context, p *db.WorkflowPlugin) error {
	if err := db.WorkflowPlugins.Delete(ctx.Request().Context(), p.PluginID); err != nil {
		return respondPluginError(ctx, err, "delete")
	}
	workflow.SyncPlugins(ctx.Request().Context(), true)
	return ctx.Status(http.StatusNoContent)
}

// TestWorkflowPlugin
// @Summary Test a workflow node plugin
// @Description Requires the admin. Run execute of the unsaved source or of the installed plugin with the config as is, and return the result and the logs. Requests are sent for real.
// @Accept json
// @Produce json
// @Param data body form.TestWorkflowPlugin true "Plugin and config"
// @Success 200 {object} dto.TestWorkflowPluginResp
// @Failure 400 {string} string "Invalid plugin"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Plugin not found"
// @Failure 500 {string} string "Internal server error"
// @ID testAdminWorkflowPlugin
// @Router /admin/workflow-plugins/test [post]
func (adminRoute) TestWorkflowPlugin(ctx context.Context, f form.TestWorkflowPlugin) error {
	source, lang := f.Source, f.Language
	if strings.TrimSpace(source) == "" {
		saved, err := db.WorkflowPlugins.GetByPluginID(ctx.Request().Context(), f.PluginID)
		if err != nil {
			if errors.Is(err, db.ErrWorkflowPluginNotFound) {
				return ctx.ApiError(http.StatusNotFound, "workflow_plugin::not_found")
			}
			return respondPluginError(ctx, err, "get")
		}
		source, lang = saved.Source, saved.Language
	}
	language, err := pluginLanguage(lang)
	if err != nil {
		return respondPluginError(ctx, err, "test")
	}
	program, m, err := inspectPlugin(ctx, source, language)
	if err != nil {
		return respondPluginError(ctx, err, "test")
	}

	start := time.Now()
	result, err := workflow.TestPlugin(ctx.Request().Context(), program, m, f.Config)
	resp := dto.TestWorkflowPluginResp{Logs: []string{}, DurationMs: time.Since(start).Milliseconds()}
	if result != nil {
		if result.Logs != nil {
			resp.Logs = result.Logs
		}
		resp.Fetches = result.Fetches
	}
	if err != nil {
		var actionErr *workflow.ActionError
		if !errors.As(err, &actionErr) {
			logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to test workflow plugin")
			return ctx.ApiServerError()
		}
		resp.Error = actionErr.Message
		return ctx.ApiSuccess(resp)
	}
	resp.Value = result.Value
	return ctx.ApiSuccess(resp)
}
