package db

import (
	"context"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/i18n"
)

var _ FieldShortcutsStore = (*fieldShortcuts)(nil)

// FieldShortcuts is the default instance of the FieldShortcutsStore.
var FieldShortcuts FieldShortcutsStore

// FieldShortcutsStore is the persistent interface for the custom field shortcuts published by the admins.
type FieldShortcutsStore interface {
	// List returns all the custom shortcuts in creation order.
	List(ctx context.Context) ([]*CustomFieldShortcut, error)
	// ListAdmin returns a page of matching shortcuts and the total number of matches in creation order.
	ListAdmin(ctx context.Context, opts ListFieldShortcutsOptions) ([]*CustomFieldShortcut, int64, error)
	// GetByUID returns the shortcut with the given UID, it returns ErrFieldShortcutNotFound if not found.
	GetByUID(ctx context.Context, uid string) (*CustomFieldShortcut, error)
	// Create creates the shortcut, a random UID is generated if empty.
	Create(ctx context.Context, s *CustomFieldShortcut) error
	// Update updates all the properties except UID, it returns ErrFieldShortcutNotFound if not found.
	Update(ctx context.Context, s *CustomFieldShortcut) error
	// Delete deletes the shortcut with the given UID.
	Delete(ctx context.Context, uid string) error
}

func NewFieldShortcutsStore(db *gorm.DB) FieldShortcutsStore {
	return &fieldShortcuts{db}
}

// ShortcutWidget is the control of a shortcut form field.
type ShortcutWidget string

const (
	// ShortcutWidgetFieldSelect picks a field of the table, the cell value of the record is passed when executing.
	ShortcutWidgetFieldSelect ShortcutWidget = "field_select"
	// ShortcutWidgetText is a text input, or a text area if the field is multiline.
	ShortcutWidgetText   ShortcutWidget = "text"
	ShortcutWidgetSelect ShortcutWidget = "select"
	// ShortcutWidgetFieldOptions edits the host select field's options and passes their names as a string array.
	ShortcutWidgetFieldOptions ShortcutWidget = "field_options"
	// ShortcutWidgetPrompt is a text referencing the fields by `{fieldUID}`, which are replaced by the cell texts when executing.
	ShortcutWidgetPrompt ShortcutWidget = "prompt"
)

// Valid reports whether the widget is supported.
func (w ShortcutWidget) Valid() bool {
	switch w {
	case ShortcutWidgetFieldSelect, ShortcutWidgetText, ShortcutWidgetSelect, ShortcutWidgetFieldOptions, ShortcutWidgetPrompt:
		return true
	}
	return false
}

// ShortcutOption is an option of the select field.
type ShortcutOption struct {
	// Value is the configured value of the option.
	Value string `json:"value"`
	// Label is the text of the option shown in the form, the value if empty.
	Label i18n.LocalText `json:"label,omitempty"`
} // @name ShortcutOption

// ShortcutField describes an input of a shortcut, the field editor renders the form by them.
type ShortcutField struct {
	// Key is the key of the configured value in FieldShortcut.Inputs and of the parameter passed to the shortcut.
	Key string `json:"key"`
	// Widget is the control to render the field.
	Widget ShortcutWidget `json:"widget" swaggertype:"string" enums:"field_select,text,select,field_options,prompt"`
	// Label is the name of the field shown in the form, the key if empty.
	Label i18n.LocalText `json:"label,omitempty"`
	// Help is the description shown below the control.
	Help i18n.LocalText `json:"help,omitempty"`
	// Placeholder is the hint of the empty input.
	Placeholder i18n.LocalText `json:"placeholder,omitempty"`
	// Required reports whether the field must be filled in before saving or executing.
	Required bool `json:"required,omitempty"`
	// Multiline shows a text area for text.
	Multiline bool `json:"multiline,omitempty"`
	// FieldTypes limits the fields of field_select, empty allows all the fields except formulas.
	FieldTypes []SLFieldType `json:"fieldTypes,omitempty" swaggertype:"array,string"`
	// Options are the choices of select.
	Options []ShortcutOption `json:"options,omitempty"`
	// Default is the initial value when configuring a new field.
	Default string `json:"default,omitempty"`
} // @name ShortcutField

// DisplayLabel is the label of the field, or its key if the label is empty.
func (f ShortcutField) DisplayLabel() i18n.LocalText {
	if strings.TrimSpace(f.Label.In("en")) == "" {
		return i18n.LocalText{"": f.Key}
	}
	return f.Label
}

// ShortcutSection is a group of the form fields of a shortcut.
type ShortcutSection struct {
	Title       i18n.LocalText  `json:"title,omitempty"`
	Description i18n.LocalText  `json:"description,omitempty"`
	Fields      []ShortcutField `json:"fields"`
} // @name ShortcutSection

// ShortcutPermissions are what a shortcut is allowed to reach.
type ShortcutPermissions struct {
	// Fetch are the hosts context.fetch can request, including their subdomains; * allows any public host.
	Fetch []string `json:"fetch"`
	// AI allows context.ai.complete to use the globally configured model.
	AI bool `json:"ai"`
} // @name ShortcutPermissions

// ShortcutCredentialType is how a credential is put into the request.
type ShortcutCredentialType string

const (
	// ShortcutCredentialBearer sets the `Authorization: Bearer <value>` header.
	ShortcutCredentialBearer ShortcutCredentialType = "bearer"
	// ShortcutCredentialHeader sets the header of the name.
	ShortcutCredentialHeader ShortcutCredentialType = "header"
	// ShortcutCredentialQuery sets the query parameter of the name.
	ShortcutCredentialQuery ShortcutCredentialType = "query"
)

// Valid reports whether the credential type is supported.
func (t ShortcutCredentialType) Valid() bool {
	switch t {
	case ShortcutCredentialBearer, ShortcutCredentialHeader, ShortcutCredentialQuery:
		return true
	}
	return false
}

// ShortcutCredential is a credential the script refers to by key when fetching, its value is stored encrypted in the secrets.
type ShortcutCredential struct {
	// Key is the identifier the script passes to context.fetch to use the credential.
	Key string `json:"key"`
	// Label is the display name of the credential, the key if empty.
	Label i18n.LocalText `json:"label,omitempty"`
	// Type is how the credential is put into the request.
	Type ShortcutCredentialType `json:"type" swaggertype:"string" enums:"bearer,header,query"`
	// Name is the header or query parameter name, it is ignored by bearer.
	Name string `json:"name,omitempty"`
} // @name ShortcutCredential

// CustomFieldShortcut is a field shortcut implemented by a JavaScript function, which is executed on the server.
type CustomFieldShortcut struct {
	// Model contains the primary key and the creation, update and deletion times.
	dbutil.Model

	// UID is "fsc" followed by 7 random characters, the fields refer to the shortcut by it.
	UID string `gorm:"type:varchar(16);not null;uniqueIndex:idx_field_shortcuts_uid, where:deleted_at IS NULL"`
	// Name is shown in the field editor.
	Name i18n.LocalText `gorm:"type:jsonb;not null"`
	// Version is the manifest version, defaulting to 1.0.0 for existing shortcuts.
	Version string `gorm:"type:varchar(32);not null;default:1.0.0"`
	// Description tells the members what the shortcut does, empty if not set.
	Description i18n.LocalText `gorm:"type:jsonb;not null"`
	// ResultType is the type of the fields the shortcut can be attached to.
	ResultType SLFieldType `gorm:"type:varchar(32);not null"`
	// Code defines `async function execute(params, context)` returning the cell value, with context.ai.complete using the global model when AIEnabled.
	Code string `gorm:"type:text;not null"`
	// Language is javascript for existing shortcuts, or typescript for typed modules.
	Language string `gorm:"type:varchar(16);not null;default:javascript"`
	// AIEnabled allows the script to call the globally configured AI model.
	AIEnabled bool `gorm:"not null;default:false"`
	// Sections are the form fields configured in the field editor and passed to execute as the params.
	Sections datatypes.JSONType[[]ShortcutSection] `gorm:"type:jsonb;not null"`
	// Domains are the hosts the script can fetch, including their subdomains.
	Domains datatypes.JSONType[[]string] `gorm:"type:jsonb;not null"`
	// Credentials are the definitions of the credentials without their values.
	Credentials datatypes.JSONType[[]ShortcutCredential] `gorm:"type:jsonb;not null"`
	// Secrets are the credential values keyed by credential key, encrypted by sso.Seal.
	Secrets string `gorm:"type:text;not null;default:''"`
	// TimeoutSeconds is the time limit of an execution.
	TimeoutSeconds int `gorm:"not null;default:30"`
	// Enabled reports whether the members can attach and execute the shortcut.
	Enabled bool `gorm:"not null;default:false"`
}

// Fields returns the form fields of all the sections.
func (s *CustomFieldShortcut) Fields() []ShortcutField {
	var fields []ShortcutField
	for _, section := range s.Sections.Data() {
		fields = append(fields, section.Fields...)
	}
	return fields
}

// TableName keeps the table name short, as there is no built-in shortcut table.
func (*CustomFieldShortcut) TableName() string {
	return "field_shortcuts"
}

// NewCustomFieldShortcutUID returns a random UID of the custom shortcut.
func NewCustomFieldShortcutUID() string {
	return "fsc" + randstr.String(7)
}

type fieldShortcuts struct {
	*gorm.DB
}

var ErrFieldShortcutNotFound = errors.New("field shortcut does not exist")

func (db *fieldShortcuts) List(ctx context.Context) ([]*CustomFieldShortcut, error) {
	var list []*CustomFieldShortcut
	if err := db.WithContext(ctx).Order("id ASC").Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return list, nil
}

// ListFieldShortcutsOptions are the options of listing shortcuts in the admin console.
type ListFieldShortcutsOptions struct {
	dbutil.Pagination
	// Keyword matches a translation of the name or description case-insensitively.
	Keyword string
}

func (db *fieldShortcuts) ListAdmin(ctx context.Context, opts ListFieldShortcutsOptions) ([]*CustomFieldShortcut, int64, error) {
	q := db.WithContext(ctx).Model(&CustomFieldShortcut{})
	if k := strings.TrimSpace(opts.Keyword); k != "" {
		pattern := "%" + dbutil.EscapeLike(strings.ToLower(k)) + "%"
		q = q.Where("EXISTS (SELECT 1 FROM jsonb_each_text(name) WHERE LOWER(value) LIKE ?) OR EXISTS (SELECT 1 FROM jsonb_each_text(description) WHERE LOWER(value) LIKE ?)", pattern, pattern)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}
	limit, offset := opts.LimitOffset()
	var list []*CustomFieldShortcut
	if err := q.Order("id ASC").Limit(limit).Offset(offset).Find(&list).Error; err != nil {
		return nil, 0, errors.Wrap(err, "find")
	}
	return list, total, nil
}

func (db *fieldShortcuts) GetByUID(ctx context.Context, uid string) (*CustomFieldShortcut, error) {
	var s CustomFieldShortcut
	if err := db.WithContext(ctx).Where("uid = ?", uid).First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFieldShortcutNotFound
		}
		return nil, errors.Wrap(err, "get")
	}
	return &s, nil
}

func (db *fieldShortcuts) Create(ctx context.Context, s *CustomFieldShortcut) error {
	if s.UID == "" {
		s.UID = NewCustomFieldShortcutUID()
	}
	if err := db.WithContext(ctx).Create(s).Error; err != nil {
		return errors.Wrap(err, "create")
	}
	return nil
}

func (db *fieldShortcuts) Update(ctx context.Context, s *CustomFieldShortcut) error {
	result := db.WithContext(ctx).Model(&CustomFieldShortcut{}).Where("uid = ?", s.UID).Updates(map[string]interface{}{
		"name":            s.Name,
		"version":         s.Version,
		"description":     s.Description,
		"result_type":     s.ResultType,
		"code":            s.Code,
		"language":        s.Language,
		"ai_enabled":      s.AIEnabled,
		"sections":        s.Sections,
		"domains":         s.Domains,
		"credentials":     s.Credentials,
		"secrets":         s.Secrets,
		"timeout_seconds": s.TimeoutSeconds,
		"enabled":         s.Enabled,
		"updated_at":      dbutil.Now(),
	})
	if result.Error != nil {
		return errors.Wrap(result.Error, "update")
	}
	if result.RowsAffected == 0 {
		return ErrFieldShortcutNotFound
	}
	return nil
}

func (db *fieldShortcuts) Delete(ctx context.Context, uid string) error {
	if err := db.WithContext(ctx).Where("uid = ?", uid).Delete(&CustomFieldShortcut{}).Error; err != nil {
		return errors.Wrap(err, "delete")
	}
	return nil
}
