package form

import (
	"encoding/json"

	"github.com/wuhan005/sayrud/internal/workflow"
)

type CreateWorkflow struct {
	Name        string `json:"name" valid:"required;maxlen:64"`
	Description string `json:"description" valid:"maxlen:500"`
	// TriggerType is the trigger type, an empty value creates a blank workflow without a trigger type chosen yet.
	TriggerType string `json:"triggerType" enums:",api,record_created,record_updated,record_matched,record_deleted,date_reached,schedule"`
	// Template is the template of the API trigger: blank, create or query, the latter two generate the default spec from the table of TableUID.
	Template string `json:"template" enums:"blank,create,query"`
	// TableUID is the table used by the API template, or the table watched by the record and date triggers.
	TableUID string `json:"tableUID,omitempty"`
	// Timezone is the creator's time zone, e.g. Asia/Shanghai, used by the formulas and the schedule trigger.
	Timezone string `json:"timezone,omitempty"`
} // @name CreateWorkflow

type SaveWorkflowDraft struct {
	Name        string `json:"name" valid:"required;maxlen:64"`
	Description string `json:"description" valid:"maxlen:500"`
	// DraftVersion is the draft version the edit is based on, 409 is returned if someone else has saved it.
	DraftVersion int64         `json:"draftVersion" valid:"required"`
	Draft        workflow.Spec `json:"draft"`
} // @name SaveWorkflowDraft

type PublishWorkflow struct {
	// DraftVersion is the draft version to publish, 409 is returned if the draft has changed.
	DraftVersion int64 `json:"draftVersion" valid:"required"`
} // @name PublishWorkflow

type RollbackWorkflow struct {
	// DraftVersion is the current draft version which the rollback overwrites, 409 is returned if the draft has changed.
	DraftVersion int64 `json:"draftVersion" valid:"required"`
} // @name RollbackWorkflow

type SetWorkflowEnabled struct {
	Enabled bool `json:"enabled"`
} // @name SetWorkflowEnabled

type DebugWorkflow struct {
	// Draft debugs the saved draft if empty.
	Draft *workflow.Spec `json:"draft,omitempty"`
	// Mode is dry_run (rolls back the writes, without running the async flows and external actions) or live (runs for real).
	Mode string `json:"mode" enums:"dry_run,live"`
	// RecordUID is the record used as the triggering record when debugging the record and date triggers, the latest record is used if empty.
	RecordUID string `json:"recordUID,omitempty"`
	// Query, Headers and Body are the request when debugging the API trigger.
	Query   map[string]string `json:"query,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	// Body is the JSON request body, empty means no request body is sent.
	Body json.RawMessage `json:"body,omitempty" swaggertype:"object"`
} // @name DebugWorkflow

type CreateWorkflowKey struct {
	Name string `json:"name" valid:"required;maxlen:64"`
} // @name CreateWorkflowKey

type SaveCredential struct {
	// Type is the credential type, only used on creation.
	Type string `json:"type"`
	Name string `json:"name" valid:"required;maxlen:64"`
	// Data holds the non-secret fields of the credential type DSL.
	Data map[string]interface{} `json:"data"`
	// Secrets holds the secret fields, empty ones keep the saved values; the fields in ClearSecrets are cleared.
	Secrets      map[string]string `json:"secrets,omitempty"`
	ClearSecrets []string          `json:"clearSecrets,omitempty"`
} // @name SaveCredential

type TestCredential struct {
	Type string `json:"type" valid:"required"`
	// CredentialUID provides the saved secrets for the fields not given.
	CredentialUID string                 `json:"credentialUID,omitempty"`
	Data          map[string]interface{} `json:"data"`
	Secrets       map[string]string      `json:"secrets,omitempty"`
	ClearSecrets  []string               `json:"clearSecrets,omitempty"`
	// Input holds the values of test.fields in the credential type DSL, e.g. the recipient of the test email.
	Input map[string]string `json:"input"`
} // @name TestCredential
