package form

import (
	"encoding/json"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/shortcut"
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

// SaveFieldShortcut creates or updates a custom field shortcut, the manifest properties are the same as those exported by the source.
type SaveFieldShortcut struct {
	shortcut.Manifest
	// Code defines `async function execute(params, context)`, with context.ai.complete using the global model when permissions.ai is true.
	Code string `json:"code"`
	// Language of Code, javascript by default.
	Language string `json:"language,omitempty" enums:"javascript,typescript"`
	// Enabled reports whether the members can attach and execute the shortcut.
	Enabled bool `json:"enabled"`
	// Secrets are the new credential values keyed by credential key, the omitted or empty ones keep the saved values.
	Secrets map[string]string `json:"secrets,omitempty"`
	// Signature is the Sayrud Hub signature returned by the inspection, it must describe Code; without it the saved one is kept while it still does.
	Signature json.RawMessage `json:"signature,omitempty" swaggertype:"object"`
} // @name SaveFieldShortcut

// InspectFieldShortcut checks a single JavaScript or TypeScript module before installing it.
type InspectFieldShortcut struct {
	Source string `json:"source,omitempty"`
	// Language of Source, javascript by default; URL extensions take precedence.
	Language string `json:"language,omitempty" enums:"javascript,typescript"`
	// URL is downloaded when Source is empty, a source URL of Sayrud Hub is verified by its signature.
	URL string `json:"url,omitempty"`
	// HubID downloads the latest release of the shortcut from Sayrud Hub instead, it takes precedence over Source and URL.
	HubID string `json:"hubID,omitempty"`
} // @name InspectFieldShortcut

// InstallFieldShortcut installs the exact source reviewed by the admin.
type InstallFieldShortcut struct {
	Source string `json:"source"`
	// Language of Source, javascript by default.
	Language string `json:"language,omitempty" enums:"javascript,typescript"`
	// Signature is the Sayrud Hub signature returned by the inspection, it must describe Source.
	Signature json.RawMessage `json:"signature,omitempty" swaggertype:"object"`
} // @name InstallFieldShortcut

// TestFieldShortcut runs the unsaved custom shortcut with the parameters.
type TestFieldShortcut struct {
	// UID is the saved shortcut whose credential values are used if the values are not given.
	UID string `json:"uid,omitempty"`
	// Shortcut is the unsaved shortcut to run.
	Shortcut SaveFieldShortcut `json:"shortcut"`
	// Params are passed to execute as is, keyed by the form field key.
	Params map[string]interface{} `json:"params"`
} // @name TestFieldShortcut
