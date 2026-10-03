package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ FieldShortcutsStore = (*fieldShortcuts)(nil)

// FieldShortcuts is the default instance of the FieldShortcutsStore.
var FieldShortcuts FieldShortcutsStore

// FieldShortcutsStore is the persistent interface for the custom field shortcuts published by the admins.
type FieldShortcutsStore interface {
	// List returns all the custom shortcuts in creation order.
	List(ctx context.Context) ([]*CustomFieldShortcut, error)
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

// ShortcutFormComponent is the control of a shortcut form item.
type ShortcutFormComponent string

const (
	// ShortcutFormFieldSelect picks a field of the table, the cell value of the record is passed when executing.
	ShortcutFormFieldSelect ShortcutFormComponent = "field_select"
	ShortcutFormInput       ShortcutFormComponent = "input"
	ShortcutFormTextarea    ShortcutFormComponent = "textarea"
	ShortcutFormSelect      ShortcutFormComponent = "select"
	// ShortcutFormPrompt is a text referencing the fields by `{fieldUID}`, which are replaced by the cell texts when executing.
	ShortcutFormPrompt ShortcutFormComponent = "prompt"
)

// Valid reports whether the component is supported.
func (c ShortcutFormComponent) Valid() bool {
	switch c {
	case ShortcutFormFieldSelect, ShortcutFormInput, ShortcutFormTextarea, ShortcutFormSelect, ShortcutFormPrompt:
		return true
	}
	return false
}

// ShortcutFormOption is an option of the select form item.
type ShortcutFormOption struct {
	// Value is the configured value of the option.
	Value string `json:"value"`
	// Label is the text of the option shown in the form.
	Label string `json:"label"`
} // @name ShortcutFormOption

// ShortcutFormItem describes an input of a shortcut, the field editor renders the form by them.
type ShortcutFormItem struct {
	// Key is the key of the configured value in FieldShortcut.Inputs and of the parameter passed to the shortcut.
	Key string `json:"key"`
	// Label is the name of the item shown in the form.
	Label string `json:"label"`
	// Component is the control to render the item.
	Component ShortcutFormComponent `json:"component" swaggertype:"string" enums:"field_select,input,textarea,select,prompt"`
	// Required reports whether the item must be filled in before saving or executing.
	Required bool `json:"required,omitempty"`
	// Placeholder is the hint of the empty input.
	Placeholder string `json:"placeholder,omitempty"`
	// FieldTypes limits the fields of field_select, empty allows all the fields except formulas.
	FieldTypes []SLFieldType `json:"fieldTypes,omitempty" swaggertype:"array,string"`
	// Options are the choices of select.
	Options []ShortcutFormOption `json:"options,omitempty"`
	// Default is the initial value when configuring a new field.
	Default string `json:"default,omitempty"`
} // @name ShortcutFormItem

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
	// Type is how the credential is put into the request.
	Type ShortcutCredentialType `json:"type"`
	// Name is the header or query parameter name, it is ignored by bearer.
	Name string `json:"name,omitempty"`
} // @name ShortcutCredential

// CustomFieldShortcut is a field shortcut implemented by a JavaScript function, which is executed on the server.
type CustomFieldShortcut struct {
	// ID is the primary key.
	ID int64 `gorm:"primarykey"`
	// UID is "fsc" followed by 7 random characters, the fields refer to the shortcut by it.
	UID string `gorm:"type:varchar(16);not null;uniqueIndex:idx_field_shortcuts_uid"`
	// Name is shown in the field editor.
	Name string `gorm:"type:varchar(64);not null"`
	// Description tells the members what the shortcut does, empty if not set.
	Description string `gorm:"type:text;not null;default:''"`
	// ResultType is the type of the fields the shortcut can be attached to.
	ResultType SLFieldType `gorm:"type:varchar(32);not null"`
	// Code defines `async function execute(params, context)` returning the cell value, with context.ai.complete using the global model when AIEnabled.
	Code string `gorm:"type:text;not null"`
	// AIEnabled allows the script to call the globally configured AI model.
	AIEnabled bool `gorm:"not null;default:false"`
	// FormItems are the inputs configured in the field editor and passed to execute as the params.
	FormItems datatypes.JSONType[[]ShortcutFormItem] `gorm:"type:jsonb;not null"`
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
	// CreatedAt is the time the shortcut was created.
	CreatedAt time.Time
	// UpdatedAt is the time the shortcut was last updated.
	UpdatedAt time.Time
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
		"description":     s.Description,
		"result_type":     s.ResultType,
		"code":            s.Code,
		"ai_enabled":      s.AIEnabled,
		"form_items":      s.FormItems,
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
