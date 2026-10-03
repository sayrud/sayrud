package api

import (
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
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
// @Success 200 {array} dto.AdminFieldShortcut
// @Failure 403 {string} string "Not an admin"
// @Failure 500 {string} string "Internal server error"
// @ID listAdminFieldShortcuts
// @Router /admin/field-shortcuts [get]
func (adminRoute) ListFieldShortcuts(ctx context.Context) error {
	c := ctx.Request().Context()
	list, err := db.FieldShortcuts.List(c)
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
	return ctx.ApiSuccess(resp)
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

const (
	maxShortcutNameLength        = 64
	maxShortcutDescriptionLength = 500
	maxShortcutCodeSize          = 200 << 10
	maxShortcutFormItems         = 20
	maxShortcutDomains           = 50
	maxShortcutCredentials       = 20
	maxShortcutTimeoutSeconds    = 300
	defaultShortcutTimeout       = 30
)

var (
	shortcutKeyPattern  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,31}$`)
	headerNamePattern   = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
	errSecretKeyMissing = errors.New("secret key is not configured")
)

// buildCustomShortcut validates the form and returns the shortcut without the secrets, along with the credential values keyed by key.
func buildCustomShortcut(f form.SaveFieldShortcut) (*db.CustomFieldShortcut, map[string]string, error) {
	name := strings.TrimSpace(f.Name)
	switch {
	case name == "":
		return nil, nil, i18n.Errorf("field_shortcut::name_required")
	case utf8.RuneCountInString(name) > maxShortcutNameLength:
		return nil, nil, i18n.Errorf("field_shortcut::name_too_long", maxShortcutNameLength)
	case utf8.RuneCountInString(f.Description) > maxShortcutDescriptionLength:
		return nil, nil, i18n.Errorf("field_shortcut::description_too_long", maxShortcutDescriptionLength)
	}
	resultType := db.SLFieldType(f.ResultType)
	if !resultType.CanHostShortcut() {
		return nil, nil, i18n.Errorf("field_shortcut::invalid_result_type")
	}
	if strings.TrimSpace(f.Code) == "" {
		return nil, nil, i18n.Errorf("field_shortcut::code_required")
	}
	if len(f.Code) > maxShortcutCodeSize {
		return nil, nil, i18n.Errorf("field_shortcut::code_too_long")
	}
	if err := script.Compile(f.Code); err != nil {
		return nil, nil, &i18n.Error{Key: "field_shortcut::compile_error", Args: []interface{}{err.Error()}}
	}

	formItems, err := normalizeFormItems(f.FormItems)
	if err != nil {
		return nil, nil, err
	}

	if len(f.Domains) > maxShortcutDomains {
		return nil, nil, i18n.Errorf("field_shortcut::too_many_domains", maxShortcutDomains)
	}
	domains := make([]string, 0, len(f.Domains))
	for _, d := range f.Domains {
		normalized, ok := script.NormalizeDomain(d)
		if !ok {
			return nil, nil, i18n.Errorf("field_shortcut::invalid_domain", d)
		}
		domains = append(domains, normalized)
	}

	if len(f.Credentials) > maxShortcutCredentials {
		return nil, nil, i18n.Errorf("field_shortcut::too_many_credentials", maxShortcutCredentials)
	}
	credentials := make([]db.ShortcutCredential, 0, len(f.Credentials))
	values := map[string]string{}
	for _, c := range f.Credentials {
		key := strings.TrimSpace(c.Key)
		credentialType := db.ShortcutCredentialType(c.Type)
		credentialName := strings.TrimSpace(c.Name)
		switch {
		case !shortcutKeyPattern.MatchString(key):
			return nil, nil, i18n.Errorf("field_shortcut::invalid_credential_key", key)
		case lo.ContainsBy(credentials, func(other db.ShortcutCredential) bool { return other.Key == key }):
			return nil, nil, i18n.Errorf("field_shortcut::duplicate_credential_key", key)
		case !credentialType.Valid():
			return nil, nil, i18n.Errorf("field_shortcut::invalid_credential_type", key)
		case credentialType == db.ShortcutCredentialBearer:
			credentialName = ""
		case credentialName == "":
			return nil, nil, i18n.Errorf("field_shortcut::credential_name_required", key)
		case credentialType == db.ShortcutCredentialHeader && !headerNamePattern.MatchString(credentialName):
			return nil, nil, i18n.Errorf("field_shortcut::invalid_credential_name", key)
		}
		credentials = append(credentials, db.ShortcutCredential{Key: key, Type: credentialType, Name: credentialName})
		if c.Value != "" {
			values[key] = c.Value
		}
	}

	timeout := f.TimeoutSeconds
	if timeout == 0 {
		timeout = defaultShortcutTimeout
	}
	if timeout < 1 || timeout > maxShortcutTimeoutSeconds {
		return nil, nil, i18n.Errorf("field_shortcut::invalid_timeout", maxShortcutTimeoutSeconds)
	}

	return &db.CustomFieldShortcut{
		Name:           name,
		Description:    strings.TrimSpace(f.Description),
		ResultType:     resultType,
		Code:           f.Code,
		AIEnabled:      f.AIEnabled,
		FormItems:      datatypes.NewJSONType(formItems),
		Domains:        datatypes.NewJSONType(lo.Uniq(domains)),
		Credentials:    datatypes.NewJSONType(credentials),
		TimeoutSeconds: timeout,
		Enabled:        f.Enabled,
	}, values, nil
}

func normalizeFormItems(items []db.ShortcutFormItem) ([]db.ShortcutFormItem, error) {
	if len(items) > maxShortcutFormItems {
		return nil, i18n.Errorf("field_shortcut::too_many_form_items", maxShortcutFormItems)
	}
	result := make([]db.ShortcutFormItem, 0, len(items))
	for _, item := range items {
		item.Key = strings.TrimSpace(item.Key)
		item.Label = strings.TrimSpace(item.Label)
		item.Placeholder = strings.TrimSpace(item.Placeholder)
		switch {
		case !shortcutKeyPattern.MatchString(item.Key):
			return nil, i18n.Errorf("field_shortcut::invalid_form_key", item.Key)
		case lo.ContainsBy(result, func(other db.ShortcutFormItem) bool { return other.Key == item.Key }):
			return nil, i18n.Errorf("field_shortcut::duplicate_form_key", item.Key)
		case item.Label == "" || utf8.RuneCountInString(item.Label) > maxShortcutNameLength:
			return nil, i18n.Errorf("field_shortcut::invalid_form_label", item.Key)
		case !item.Component.Valid():
			return nil, i18n.Errorf("field_shortcut::invalid_component", item.Key)
		}

		if item.Component == db.ShortcutFormFieldSelect {
			for _, t := range item.FieldTypes {
				if !t.Check() || t == db.FormulaFieldType {
					return nil, i18n.Errorf("field_shortcut::invalid_field_types", item.Key)
				}
			}
			item.FieldTypes = lo.Uniq(item.FieldTypes)
			item.Default = ""
		} else {
			item.FieldTypes = nil
		}

		if item.Component == db.ShortcutFormSelect {
			values := lo.Map(item.Options, func(o db.ShortcutFormOption, _ int) string { return o.Value })
			if len(item.Options) == 0 || lo.Contains(values, "") || len(lo.Uniq(values)) != len(values) {
				return nil, i18n.Errorf("field_shortcut::invalid_options", item.Key)
			}
			item.Options = lo.Map(item.Options, func(o db.ShortcutFormOption, _ int) db.ShortcutFormOption {
				return db.ShortcutFormOption{Value: o.Value, Label: lo.Ternary(strings.TrimSpace(o.Label) == "", o.Value, strings.TrimSpace(o.Label))}
			})
			if item.Default != "" && !lo.Contains(values, item.Default) {
				return nil, i18n.Errorf("field_shortcut::invalid_options", item.Key)
			}
		} else {
			item.Options = nil
		}
		result = append(result, item)
	}
	return result, nil
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
			resp.Error = shortcutErr.Text(ctx.Tr)
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
				resp.Error = shortcutErr.Text(ctx.Tr)
			}
		}
		resp.CellValue = cellValue
	}
	return ctx.ApiSuccess(resp)
}
