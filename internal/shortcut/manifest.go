package shortcut

import (
	"context"
	"strings"

	"github.com/pkg/errors"

	"github.com/wuhan005/sayrud/internal/db"
)

// Kind is how a shortcut is executed.
type Kind string

const KindScript Kind = "script"

// Definition describes a custom field shortcut.
type Definition struct {
	ID          string
	Kind        Kind
	Name        string
	Description string
	ResultTypes []db.SLFieldType
	FormItems   []db.ShortcutFormItem

	custom *db.CustomFieldShortcut
}

// Available reports whether the shortcut can be executed now.
func (d *Definition) Available() bool {
	return d.custom != nil && d.custom.Enabled
}

// SupportsType reports whether the shortcut can be attached to a field of the type.
func (d *Definition) SupportsType(t db.SLFieldType) bool {
	for _, rt := range d.ResultTypes {
		if rt == t {
			return true
		}
	}
	return false
}

// IsCustomID reports whether the ID refers to a custom shortcut.
func IsCustomID(id string) bool {
	return strings.HasPrefix(id, "fsc")
}

// Lookup returns the definition of the shortcut ID, it returns *Error "shortcut::not_found" if it does not exist.
func Lookup(ctx context.Context, id string) (*Definition, error) {
	if !IsCustomID(id) {
		return nil, newError("shortcut::not_found")
	}
	custom, err := db.FieldShortcuts.GetByUID(ctx, id)
	if err != nil {
		if errors.Is(err, db.ErrFieldShortcutNotFound) {
			return nil, newError("shortcut::not_found")
		}
		return nil, errors.Wrap(err, "get custom shortcut")
	}
	return CustomDefinition(custom), nil
}

// CustomDefinition returns the definition of the custom shortcut.
func CustomDefinition(custom *db.CustomFieldShortcut) *Definition {
	return &Definition{
		ID:          custom.UID,
		Kind:        KindScript,
		Name:        custom.Name,
		Description: custom.Description,
		ResultTypes: []db.SLFieldType{custom.ResultType},
		FormItems:   custom.FormItems.Data(),
		custom:      custom,
	}
}

// Catalog returns the enabled custom shortcuts.
func Catalog(ctx context.Context) ([]*Definition, error) {
	customs, err := db.FieldShortcuts.List(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "list custom shortcuts")
	}
	list := make([]*Definition, 0, len(customs))
	for _, custom := range customs {
		if custom.Enabled {
			list = append(list, CustomDefinition(custom))
		}
	}
	return list, nil
}
