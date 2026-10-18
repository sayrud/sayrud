package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/sso"
	"github.com/wuhan005/sayrud/internal/workflow"
)

var Credential credentialRoute

type credentialRoute struct{}

// Credentialer injects the credential of the credentialUID path parameter as *db.Credential, the credential must belong to the current project.
func (credentialRoute) Credentialer(ctx context.Context, project *db.Project) error {
	c, err := db.Credentials.GetByUID(ctx.Request().Context(), ctx.Param("credentialUID"))
	if err != nil {
		if errors.Is(err, db.ErrCredentialNotFound) {
			return ctx.ApiError(http.StatusNotFound, "credential::not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get credential")
		return ctx.ApiServerError()
	}
	if c.ProjectID != project.ID {
		return ctx.ApiError(http.StatusNotFound, "credential::not_found")
	}
	ctx.Map(c)
	return nil
}

// secretKeys returns the names of the saved secret fields.
func secretKeys(secrets map[string]string) []string {
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func toCredential(c *db.Credential) *dto.Credential {
	secrets, _ := workflow.OpenSecrets(c.Sealed)
	return dto.ToCredential(c, secretKeys(secrets))
}

// ListCredentials
// @Summary List the credentials of a project
// @Description The values of the secret fields are not returned. Filter by type with the type query.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param type query string false "Credential type"
// @Success 200 {array} dto.Credential
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID listCredentials
// @Router /projects/{projectUID}/credentials [get]
func (credentialRoute) List(ctx context.Context, project *db.Project) error {
	list, err := db.Credentials.List(ctx.Request().Context(), project.ID, ctx.Query("type"))
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list credentials")
		return ctx.ApiServerError()
	}
	out := make([]*dto.Credential, 0, len(list))
	for _, c := range list {
		out = append(out, toCredential(c))
	}
	return ctx.ApiSuccess(out)
}

// GetCredential
// @Summary Get a project credential
// @Description Returns the non-secret fields and the names of saved secret fields. Secret values are never returned.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param credentialUID path string true "Credential UID"
// @Success 200 {object} dto.Credential
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or credential not found"
// @Failure 500 {string} string "Internal server error"
// @ID getCredential
// @Router /projects/{projectUID}/credentials/{credentialUID} [get]
func (credentialRoute) Get(ctx context.Context, c *db.Credential) error {
	return ctx.ApiSuccess(toCredential(c))
}

// prepareCredential validates the submitted credential and seals its secrets; current are the saved secrets, empty on creation. Validation failures are *i18n.Error.
func prepareCredential(def *workflow.CredentialTypeDef, f form.SaveCredential, current map[string]string) ([]byte, string, error) {
	data, secrets, err := workflow.PrepareCredential(def, workflow.CredentialInput{Data: f.Data, Secrets: f.Secrets, ClearSecrets: f.ClearSecrets}, current)
	if err != nil {
		return nil, "", err
	}
	changed := false
	for _, v := range f.Secrets {
		changed = changed || v != ""
	}
	if changed && !sso.SecretsReady() {
		return nil, "", i18n.Errorf("credential::secret_key_required")
	}
	sealed, err := workflow.SealSecrets(secrets)
	if err != nil {
		return nil, "", errors.Wrap(err, "seal credential secrets")
	}
	raw, _ := json.Marshal(data)
	return raw, sealed, nil
}

// respondCredentialError responds 400 for validation errors, and logs other errors and responds 500.
func respondCredentialError(ctx context.Context, err error) error {
	if isLocalized(err) {
		return ctx.ApiErrorFrom(http.StatusBadRequest, err)
	}
	logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to prepare credential")
	return ctx.ApiServerError()
}

// CreateCredential
// @Summary Create a credential in a project
// @Description The fields are defined by the DSL of the credential type, the secret fields are encrypted by auth.secret_key.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param data body form.SaveCredential true "Credential"
// @Success 200 {object} dto.Credential
// @Failure 400 {string} string "Invalid credential"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 500 {string} string "Internal server error"
// @ID createCredential
// @Router /projects/{projectUID}/credentials [post]
func (credentialRoute) Create(ctx context.Context, user *db.User, project *db.Project, f form.SaveCredential) error {
	workflow.SyncPlugins(ctx.Request().Context(), false)
	def := workflow.CredentialType(f.Type)
	if def == nil {
		return ctx.ApiError(http.StatusBadRequest, "credential::invalid_type")
	}
	data, sealed, err := prepareCredential(def, f, nil)
	if err != nil {
		return respondCredentialError(ctx, err)
	}
	c, err := db.Credentials.Create(ctx.Request().Context(), db.CreateCredentialOptions{
		ProjectID: project.ID, Type: def.Type, Name: strings.TrimSpace(f.Name), Data: data, Sealed: sealed, UserID: user.ID,
	})
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create credential")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(toCredential(c))
}

// UpdateCredential
// @Summary Update a credential
// @Description Takes effect immediately for the workflows that reference it. The empty secret fields keep the saved values.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param credentialUID path string true "Credential UID"
// @Param data body form.SaveCredential true "Credential"
// @Success 200 {object} dto.Credential
// @Failure 400 {string} string "Invalid credential or the saved secrets can not be decrypted"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or credential not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateCredential
// @Router /projects/{projectUID}/credentials/{credentialUID} [put]
func (credentialRoute) Update(ctx context.Context, user *db.User, c *db.Credential, f form.SaveCredential) error {
	workflow.SyncPlugins(ctx.Request().Context(), false)
	def := workflow.CredentialType(c.Type)
	if def == nil {
		return ctx.ApiError(http.StatusBadRequest, "credential::invalid_type")
	}
	current, err := workflow.OpenSecrets(c.Sealed)
	if err != nil {
		return ctx.ApiError(http.StatusBadRequest, "credential::secret_key_required")
	}
	data, sealed, err := prepareCredential(def, f, current)
	if err != nil {
		return respondCredentialError(ctx, err)
	}
	name := strings.TrimSpace(f.Name)
	if err := db.Credentials.Update(ctx.Request().Context(), c.ID, db.UpdateCredentialOptions{Name: name, Data: data, Sealed: sealed, UserID: user.ID}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update credential")
		return ctx.ApiServerError()
	}
	c.Name, c.Data, c.Sealed = name, data, sealed
	return ctx.ApiSuccess(toCredential(c))
}

// DeleteCredential
// @Summary Delete a credential
// @Description A credential referenced by a published workflow can not be deleted.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param credentialUID path string true "Credential UID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or credential not found"
// @Failure 409 {string} string "Used by a published workflow"
// @Failure 500 {string} string "Internal server error"
// @ID deleteCredential
// @Router /projects/{projectUID}/credentials/{credentialUID} [delete]
func (credentialRoute) Delete(ctx context.Context, project *db.Project, c *db.Credential) error {
	if err := checkWorkflowReference(ctx, db.Workflows, project, c.UID, "credential::used_by_workflow"); err != nil {
		if isLocalized(err) {
			return ctx.ApiErrorFrom(http.StatusConflict, err)
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to check credential reference")
		return ctx.ApiServerError()
	}
	if err := db.Credentials.DeleteByID(ctx.Request().Context(), c.ID); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete credential")
		return ctx.ApiServerError()
	}
	return ctx.Status(http.StatusNoContent)
}

// TestCredential
// @Summary Test a credential
// @Description Run the test action of the credential type (e.g. send a test email) with the unsaved values; the saved secrets of credentialUID are used for the fields not given.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param data body form.TestCredential true "Credential and the test input"
// @Success 204 "No Content"
// @Failure 400 {string} string "Invalid credential or the test failed"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or credential not found"
// @Failure 500 {string} string "Internal server error"
// @ID testCredential
// @Router /projects/{projectUID}/credentials/test [post]
func (credentialRoute) Test(ctx context.Context, project *db.Project, f form.TestCredential) error {
	workflow.SyncPlugins(ctx.Request().Context(), false)
	c := ctx.Request().Context()
	def := workflow.CredentialType(f.Type)
	if def == nil {
		return ctx.ApiError(http.StatusBadRequest, "credential::invalid_type")
	}
	current := map[string]string{}
	if f.CredentialUID != "" {
		saved, err := db.Credentials.GetByUID(c, f.CredentialUID)
		if err != nil || saved.ProjectID != project.ID || saved.Type != def.Type {
			if err == nil || errors.Is(err, db.ErrCredentialNotFound) {
				return ctx.ApiError(http.StatusNotFound, "credential::not_found")
			}
			logrus.WithContext(c).WithError(err).Error("Failed to get credential")
			return ctx.ApiServerError()
		}
		if current, err = workflow.OpenSecrets(saved.Sealed); err != nil {
			return ctx.ApiError(http.StatusBadRequest, "credential::secret_key_required")
		}
	}
	data, secrets, err := workflow.PrepareCredential(def, workflow.CredentialInput{Data: f.Data, Secrets: f.Secrets, ClearSecrets: f.ClearSecrets}, current)
	if err != nil {
		return ctx.ApiErrorFrom(http.StatusBadRequest, err)
	}
	raw, _ := json.Marshal(data)
	err = workflow.TestCredential(c, def, workflow.CredentialValues(raw, secrets), f.Input, workflow.CredentialTestMessage{
		Subject: ctx.Tr("credential::test_subject"), Content: ctx.Tr("credential::test_content"),
	})
	if err != nil {
		if isLocalized(err) {
			return ctx.ApiErrorFrom(http.StatusBadRequest, err)
		}
		msg := err.Error()
		var actionErr *workflow.ActionError
		if errors.As(err, &actionErr) {
			msg = actionErr.Message
		}
		if utf8.RuneCountInString(msg) > 300 {
			msg = string([]rune(msg)[:300])
		}
		return ctx.ApiError(http.StatusBadRequest, "credential::test_failed", msg)
	}
	return ctx.Status(http.StatusNoContent)
}

// ListWorkflowModules
// @Summary List the step modules and credential types
// @Description Return the JSON DSL definitions the editor renders the nodes and the configuration forms with. The texts of the installed plugins are in the language of the request.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Success 200 {object} dto.WorkflowModules
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @ID listWorkflowModules
// @Router /projects/{projectUID}/workflow-modules [get]
func (workflowRoute) Modules(ctx context.Context) error {
	workflow.SyncPlugins(ctx.Request().Context(), false)
	lang := requestLang(ctx)
	return ctx.ApiSuccess(dto.WorkflowModules{Modules: workflow.ModulesIn(lang), CredentialTypes: workflow.CredentialTypesIn(lang), SecretsReady: sso.SecretsReady()})
}
