package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/manifest"
	"github.com/wuhan005/sayrud/internal/shortcut"
	"github.com/wuhan005/sayrud/internal/shortcut/script"
	"github.com/wuhan005/sayrud/internal/sso"
)

// FieldShortcuter maps the custom shortcut of the shortcutUID path parameter as *db.CustomFieldShortcut, and responds 404 if not found.
func (adminRoute) FieldShortcuter(ctx context.Context) error {
	s, err := db.FieldShortcuts.GetByUID(ctx.Request().Context(), ctx.Param("shortcutUID"))
	if err != nil {
		if errors.Is(err, db.ErrFieldShortcutNotFound) {
			return ctx.ApiError(http.StatusNotFound, "field_shortcut::not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get field shortcut")
		return ctx.ApiServerError()
	}
	ctx.Map(s)
	return nil
}

func toAdminFieldShortcut(s *db.CustomFieldShortcut, counts map[string]int64) (*dto.AdminFieldShortcut, error) {
	valueKeys := map[string]bool{}
	if s.Secrets != "" {
		secrets, err := shortcut.DecodeSecrets(s.Secrets)
		if err != nil {
			return nil, errors.Wrap(err, "decode secrets")
		}
		for k := range secrets {
			valueKeys[k] = true
		}
	}
	return dto.ToAdminFieldShortcut(s, valueKeys, counts[s.UID]), nil
}

// ListFieldShortcuts
// @Summary List the custom field shortcuts
// @Description Requires the admin. The credential values are not returned.
// @Produce json
// @Param page query int false "Page number, starting from 1"
// @Param pageSize query int false "Page size, defaults to 20"
// @Param keyword query string false "Matches the shortcut name or description"
// @Success 200 {object} dto.ListAdminFieldShortcutsResp
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID listAdminFieldShortcuts
// @Router /admin/field-shortcuts [get]
func (adminRoute) ListFieldShortcuts(ctx context.Context) error {
	c := ctx.Request().Context()
	list, total, err := db.FieldShortcuts.ListAdmin(c, db.ListFieldShortcutsOptions{
		Pagination: dbutil.Pagination{Page: ctx.QueryInt("page"), PageSize: ctx.QueryInt("pageSize")},
		Keyword:    ctx.Query("keyword"),
	})
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to list field shortcuts")
		return ctx.ApiServerError()
	}
	counts, err := db.SLFields.CountByShortcutID(c)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to count field shortcut usages")
		return ctx.ApiServerError()
	}
	resp := make([]*dto.AdminFieldShortcut, 0, len(list))
	for _, s := range list {
		item, err := toAdminFieldShortcut(s, counts)
		if err != nil {
			// The secret key is changed, the values are unreadable and need to be entered again.
			logrus.WithContext(c).WithError(err).WithField("uid", s.UID).Warn("Failed to decode field shortcut secrets")
			item = dto.ToAdminFieldShortcut(s, nil, counts[s.UID])
		}
		resp = append(resp, item)
	}
	return ctx.ApiSuccess(dto.ListAdminFieldShortcutsResp{Shortcuts: resp, Total: total})
}

// GetFieldShortcut
// @Summary Get a custom field shortcut
// @Description Requires the admin. The credential values are not returned.
// @Produce json
// @Param shortcutUID path string true "Shortcut UID"
// @Success 200 {object} dto.AdminFieldShortcut
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Shortcut not found"
// @Failure 500 {string} string "Internal server error"
// @ID getAdminFieldShortcut
// @Router /admin/field-shortcuts/{shortcutUID} [get]
func (adminRoute) GetFieldShortcut(ctx context.Context, saved *db.CustomFieldShortcut) error {
	c := ctx.Request().Context()
	counts, err := db.SLFields.CountByShortcutID(c)
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to count field shortcut usages")
		return ctx.ApiServerError()
	}
	resp, err := toAdminFieldShortcut(saved, counts)
	if err != nil {
		// The secret key is changed, the values are unreadable and need to be entered again.
		logrus.WithContext(c).WithError(err).WithField("uid", saved.UID).Warn("Failed to decode field shortcut secrets")
		resp = dto.ToAdminFieldShortcut(saved, nil, counts[saved.UID])
	}
	return ctx.ApiSuccess(resp)
}

const maxShortcutCodeSize = 200 << 10

var errSecretKeyMissing = errors.New("secret key is not configured")

// buildCustomShortcut validates the form and returns the shortcut without the secrets, along with the credential values keyed by key.
// The invalid manifest errors are *i18n.Error.
func buildCustomShortcut(f form.SaveFieldShortcut) (*db.CustomFieldShortcut, map[string]string, error) {
	if err := f.Manifest.Normalize(); err != nil {
		return nil, nil, manifestError(err)
	}
	if strings.TrimSpace(f.Code) == "" {
		return nil, nil, i18n.Errorf("field_shortcut::code_required")
	}
	if len(f.Code) > maxShortcutCodeSize {
		return nil, nil, i18n.Errorf("field_shortcut::code_too_long")
	}
	language, err := pluginLanguage(f.Language)
	if err != nil {
		return nil, nil, err
	}
	if err := script.Compile(f.Code, string(language)); err != nil {
		return nil, nil, &i18n.Error{Key: "field_shortcut::compile_error", Args: []interface{}{err.Error()}}
	}

	m := f.Manifest
	values := map[string]string{}
	for _, c := range m.Credentials {
		if v := f.Secrets[c.Key]; v != "" {
			values[c.Key] = v
		}
	}
	return &db.CustomFieldShortcut{
		Name:           m.Name,
		Version:        m.Version,
		Description:    m.Description,
		ResultType:     db.SLFieldType(m.ResultType),
		Code:           f.Code,
		Language:       string(language),
		AIEnabled:      m.Permissions.AI,
		Sections:       datatypes.NewJSONType(m.Sections),
		Domains:        datatypes.NewJSONType(m.Permissions.Fetch),
		Credentials:    datatypes.NewJSONType(m.Credentials),
		TimeoutSeconds: m.Timeout,
		Enabled:        f.Enabled,
	}, values, nil
}

// manifestError shows the *manifest.Error like the invalid manifests of the workflow plugins.
func manifestError(err error) error {
	var manifestErr *manifest.Error
	if errors.As(err, &manifestErr) {
		return i18n.Errorf("workflow_plugin::invalid_manifest", manifestErr.Message)
	}
	return err
}

// sealSecrets merges the new credential values into the saved ones of the credentials still defined, and encrypts them.
func sealSecrets(saved string, credentials []db.ShortcutCredential, values map[string]string) (string, error) {
	merged := map[string]string{}
	if saved != "" {
		// The saved values are dropped if they can not be decrypted, e.g. the secret key changed.
		if secrets, err := shortcut.DecodeSecrets(saved); err == nil {
			merged = secrets
		}
	}
	for k, v := range values {
		merged[k] = v
	}
	merged = lo.PickBy(merged, func(k string, _ string) bool {
		return lo.ContainsBy(credentials, func(c db.ShortcutCredential) bool { return c.Key == k })
	})
	if len(merged) > 0 && !sso.SecretsReady() {
		return "", errSecretKeyMissing
	}
	return shortcut.EncodeSecrets(merged)
}

func saveFieldShortcutError(ctx context.Context, err error, action string) error {
	if errors.Is(err, errSecretKeyMissing) {
		return ctx.ApiError(http.StatusBadRequest, "field_shortcut::secret_key_required")
	}
	if msg, ok := i18n.Localize(ctx.Locale(), err); ok {
		return ctx.ApiErrorMessage(http.StatusBadRequest, msg)
	}
	logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to " + action + " field shortcut")
	return ctx.ApiServerError()
}

// CreateFieldShortcut
// @Summary Create a custom field shortcut
// @Description Requires the admin. The credential values are encrypted by auth.secret_key.
// @Accept json
// @Produce json
// @Param data body form.SaveFieldShortcut true "Shortcut"
// @Success 200 {object} dto.AdminFieldShortcut
// @Failure 400 {string} string "Invalid shortcut"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID createAdminFieldShortcut
// @Router /admin/field-shortcuts [post]
func (adminRoute) CreateFieldShortcut(ctx context.Context, f form.SaveFieldShortcut) error {
	c := ctx.Request().Context()
	s, values, err := buildCustomShortcut(f)
	if err != nil {
		return saveFieldShortcutError(ctx, err, "validate")
	}
	if s.Secrets, err = sealSecrets("", s.Credentials.Data(), values); err != nil {
		return saveFieldShortcutError(ctx, err, "seal")
	}
	if err := db.FieldShortcuts.Create(c, s); err != nil {
		return saveFieldShortcutError(ctx, err, "create")
	}
	resp, err := toAdminFieldShortcut(s, nil)
	if err != nil {
		return saveFieldShortcutError(ctx, err, "read")
	}
	return ctx.ApiSuccess(resp)
}

// UpdateFieldShortcut
// @Summary Update a custom field shortcut
// @Description Requires the admin. The empty credential values keep the saved ones.
// @Accept json
// @Produce json
// @Param shortcutUID path string true "Shortcut UID"
// @Param data body form.SaveFieldShortcut true "Shortcut"
// @Success 200 {object} dto.AdminFieldShortcut
// @Failure 400 {string} string "Invalid shortcut"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Shortcut not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateAdminFieldShortcut
// @Router /admin/field-shortcuts/{shortcutUID} [put]
func (adminRoute) UpdateFieldShortcut(ctx context.Context, saved *db.CustomFieldShortcut, f form.SaveFieldShortcut) error {
	c := ctx.Request().Context()
	s, values, err := buildCustomShortcut(f)
	if err != nil {
		return saveFieldShortcutError(ctx, err, "validate")
	}
	if s.Secrets, err = sealSecrets(saved.Secrets, s.Credentials.Data(), values); err != nil {
		return saveFieldShortcutError(ctx, err, "seal")
	}
	s.ID, s.UID, s.CreatedAt = saved.ID, saved.UID, saved.CreatedAt
	if err := db.FieldShortcuts.Update(c, s); err != nil {
		return saveFieldShortcutError(ctx, err, "update")
	}
	updated, err := db.FieldShortcuts.GetByUID(c, saved.UID)
	if err != nil {
		return saveFieldShortcutError(ctx, err, "get")
	}
	counts, err := db.SLFields.CountByShortcutID(c)
	if err != nil {
		return saveFieldShortcutError(ctx, err, "count")
	}
	resp, err := toAdminFieldShortcut(updated, counts)
	if err != nil {
		return saveFieldShortcutError(ctx, err, "read")
	}
	return ctx.ApiSuccess(resp)
}

// DeleteFieldShortcut
// @Summary Delete a custom field shortcut
// @Description Requires the admin. The fields using it keep their values, and fail to generate new ones.
// @Produce json
// @Param shortcutUID path string true "Shortcut UID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Not an admin"
// @Failure 404 {string} string "Shortcut not found"
// @Failure 500 {string} string "Internal server error"
// @ID deleteAdminFieldShortcut
// @Router /admin/field-shortcuts/{shortcutUID} [delete]
func (adminRoute) DeleteFieldShortcut(ctx context.Context, saved *db.CustomFieldShortcut) error {
	if err := db.FieldShortcuts.Delete(ctx.Request().Context(), saved.UID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete field shortcut")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// TestFieldShortcut
// @Summary Test a custom field shortcut
// @Description Requires the admin. Run the unsaved code with the parameters, and return the result and the logs.
// @Accept json
// @Produce json
// @Param data body form.TestFieldShortcut true "Shortcut and parameters"
// @Success 200 {object} dto.TestFieldShortcutResp
// @Failure 400 {string} string "Invalid shortcut"
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID testAdminFieldShortcut
// @Router /admin/field-shortcuts/test [post]
func (adminRoute) TestFieldShortcut(ctx context.Context, f form.TestFieldShortcut) error {
	c := ctx.Request().Context()
	s, values, err := buildCustomShortcut(f.Shortcut)
	if err != nil {
		return saveFieldShortcutError(ctx, err, "validate")
	}
	secrets := map[string]string{}
	if f.UID != "" {
		if saved, err := db.FieldShortcuts.GetByUID(c, f.UID); err == nil && saved.Secrets != "" {
			if decoded, err := shortcut.DecodeSecrets(saved.Secrets); err == nil {
				secrets = decoded
			}
		}
	}
	for k, v := range values {
		secrets[k] = v
	}

	start := time.Now()
	result, err := shortcut.RunCustom(c, s, secrets, f.Params, shortcut.Env{ProjectUID: "prjtest", TableUID: "tbltest", FieldUID: "fldtest", RecordUID: "rectest"})
	resp := dto.TestFieldShortcutResp{Logs: []string{}, DurationMs: time.Since(start).Milliseconds()}
	if result != nil && result.Logs != nil {
		resp.Logs = result.Logs
	}
	if err != nil {
		var shortcutErr *shortcut.Error
		if errors.As(err, &shortcutErr) {
			resp.Error = shortcutErr.Text(ctx.Tr, ctx.Locale().Lang())
		} else {
			resp.Error = err.Error()
		}
		return ctx.ApiSuccess(resp)
	}
	resp.Value = result.Value

	// The select fields have no options to match here.
	if s.ResultType != db.SingleSelectFieldType && s.ResultType != db.MultiSelectFieldType {
		cellValue, err := shortcut.CellValue(&db.SLField{Type: s.ResultType, Metadata: datatypes.NewJSONType[db.SLFieldMetadata](map[string]interface{}{})}, result.Value)
		if err != nil {
			var shortcutErr *shortcut.Error
			if errors.As(err, &shortcutErr) {
				resp.Error = shortcutErr.Text(ctx.Tr, ctx.Locale().Lang())
			}
		}
		resp.CellValue = cellValue
	}
	return ctx.ApiSuccess(resp)
}
