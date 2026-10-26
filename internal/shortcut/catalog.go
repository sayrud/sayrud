package shortcut

import (
	"context"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/wuhan005/sayrud/internal/db"
)

// IsCustomID reports whether the ID refers to a custom shortcut.
func IsCustomID(id string) bool {
	return strings.HasPrefix(id, "fsc")
}

// Lookup returns the shortcut with the ID, it returns *Error "shortcut::not_found" if it does not exist.
func Lookup(ctx context.Context, id string) (*db.CustomFieldShortcut, error) {
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
	return custom, nil
}

// Catalog returns the enabled custom shortcuts.
func Catalog(ctx context.Context) ([]*db.CustomFieldShortcut, error) {
	customs, err := db.FieldShortcuts.List(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "list custom shortcuts")
	}
	list := make([]*db.CustomFieldShortcut, 0, len(customs))
	for _, custom := range customs {
		if custom.Enabled {
			list = append(list, custom)
		}
	}
	return list, nil
}
