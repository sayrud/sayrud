package shortcut

import (
	"context"
	"strings"

	"github.com/pkg/errors"

	"github.com/wuhan005/sayrud/internal/db"
)

// Kind is how a shortcut is executed.
type Kind string

const (
	KindAI     Kind = "ai"
	KindScript Kind = "script"
)

// Definition describes a shortcut, the built-in ones are localized by Localize before showing.
type Definition struct {
	ID          string
	Kind        Kind
	Name        string
	Description string
	ResultTypes []db.SLFieldType
	FormItems   []db.ShortcutFormItem

	builtin *builtin
	custom  *db.CustomFieldShortcut
}

// Available reports whether the shortcut can be executed now, aiConfigured is the result of AIConfigured.
func (d *Definition) Available(aiConfigured bool) bool {
	switch d.Kind {
	case KindAI:
		return aiConfigured
	case KindScript:
		return d.custom != nil && d.custom.Enabled
	}
	return false
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

// Localize returns the copy with the message keys translated, the custom shortcuts are written by the admins and kept as is.
func (d *Definition) Localize(tr Translator) *Definition {
	if d.builtin == nil {
		return d
	}
	copied := *d
	copied.Name = tr(d.Name)
	copied.Description = tr(d.Description)
	copied.FormItems = make([]db.ShortcutFormItem, 0, len(d.FormItems))
	for _, item := range d.FormItems {
		item.Label = tr(item.Label)
		if item.Placeholder != "" {
			item.Placeholder = tr(item.Placeholder)
		}
		options := make([]db.ShortcutFormOption, 0, len(item.Options))
		for _, o := range item.Options {
			options = append(options, db.ShortcutFormOption{Value: o.Value, Label: tr(o.Label)})
		}
		item.Options = options
		copied.FormItems = append(copied.FormItems, item)
	}
	return &copied
}

// IsCustomID reports whether the ID refers to a custom shortcut.
func IsCustomID(id string) bool {
	return strings.HasPrefix(id, "fsc")
}

// Lookup returns the definition of the shortcut ID, it returns *Error "shortcut::not_found" if it does not exist.
func Lookup(ctx context.Context, id string) (*Definition, error) {
	if b, ok := builtins[id]; ok {
		return b.definition(), nil
	}
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

// Catalog returns the built-in shortcuts followed by the enabled custom shortcuts.
func Catalog(ctx context.Context) ([]*Definition, error) {
	list := make([]*Definition, 0, len(builtinOrder))
	for _, id := range builtinOrder {
		list = append(list, builtins[id].definition())
	}
	customs, err := db.FieldShortcuts.List(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "list custom shortcuts")
	}
	for _, custom := range customs {
		if custom.Enabled {
			list = append(list, CustomDefinition(custom))
		}
	}
	return list, nil
}
