package form

import (
	"github.com/wuhan005/sayrud/internal/db"
)

// RunFieldShortcut queues the cells of the field to generate by its shortcut.
type RunFieldShortcut struct {
	// Scope is all the records, the records whose cell is empty, or the given records.
	Scope string `json:"scope" enums:"all,empty,records"`
	// RecordUIDs are the records to generate if Scope is records.
	RecordUIDs []string `json:"recordUIDs,omitempty"`
} // @name RunFieldShortcut

// PreviewFieldShortcut executes the unsaved shortcut config on some records without writing back.
type PreviewFieldShortcut struct {
	// FieldUID is the UID of the field being edited or created.
	FieldUID string `json:"fieldUID"`
	// Type is the unsaved type of the field.
	Type string `json:"type" enums:"text,single_select,multi_select,datetime,number,checkbox"`
	// Metadata is the unsaved metadata of the field, e.g. the options of the select fields.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// Shortcut is the unsaved shortcut config of the field.
	Shortcut db.FieldShortcut `json:"shortcut"`
	// RecordUIDs are the records to preview, at most 3.
	RecordUIDs []string `json:"recordUIDs"`
} // @name PreviewFieldShortcut

// SaveFieldShortcut creates or updates a custom field shortcut.
type SaveFieldShortcut struct {
	// Name is shown in the field editor, at most 64 characters.
	Name string `json:"name"`
	// Description tells the members what the shortcut does, at most 500 characters.
	Description string `json:"description"`
	// ResultType is the type of the fields the shortcut can be attached to.
	ResultType string `json:"resultType" enums:"text,single_select,multi_select,datetime,number,checkbox"`
	// Code defines `async function execute(params, context)`, with context.ai.complete using the global model when AIEnabled.
	Code string `json:"code"`
	// AIEnabled allows the script to call the globally configured AI model.
	AIEnabled bool `json:"aiEnabled"`
	// FormItems are the inputs configured in the field editor and passed to execute as the params.
	FormItems []db.ShortcutFormItem `json:"formItems"`
	// Domains are the hosts the script can fetch, including their subdomains.
	Domains []string `json:"domains"`
	// Credentials are the credentials the script can use when fetching.
	Credentials []SaveShortcutCredential `json:"credentials"`
	// TimeoutSeconds is the time limit of an execution between 1 and 300, 0 defaults to 30.
	TimeoutSeconds int `json:"timeoutSeconds"`
	// Enabled reports whether the members can attach and execute the shortcut.
	Enabled bool `json:"enabled"`
} // @name SaveFieldShortcut

// SaveShortcutCredential is a credential of a custom shortcut.
type SaveShortcutCredential struct {
	// Key is the identifier the script passes to context.fetch to use the credential.
	Key string `json:"key"`
	// Type is how the credential is put into the request.
	Type string `json:"type" enums:"bearer,header,query"`
	// Name is the header or query parameter name, it is ignored by bearer.
	Name string `json:"name,omitempty"`
	// Value replaces the saved value, empty keeps the saved one.
	Value string `json:"value,omitempty"`
} // @name SaveShortcutCredential

// TestFieldShortcut runs the unsaved custom shortcut with the parameters.
type TestFieldShortcut struct {
	// UID is the saved shortcut whose credential values are used if the values are not given.
	UID string `json:"uid,omitempty"`
	// Shortcut is the unsaved shortcut to run.
	Shortcut SaveFieldShortcut `json:"shortcut"`
	// Params are passed to execute as is, keyed by the form item key.
	Params map[string]interface{} `json:"params"`
} // @name TestFieldShortcut
