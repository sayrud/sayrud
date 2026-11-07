package collab

import (
	"context"

	"github.com/samber/lo"

	"github.com/wuhan005/sayrud/internal/db"
)

// Change summarizes the data changed by a changeset or the REST API, the field shortcuts regenerate the dependent cells by it.
type Change struct {
	// Records are the changed fields keyed by record UID, nil means any field of the record may have changed.
	Records map[string][]string
	// Fields are the fields whose values may have changed in all the records, e.g. converted to another type.
	Fields []string
	// Shortcuts are the fields whose shortcut is added or changed, all their cells should be regenerated.
	Shortcuts []string
	// RemovedShortcuts are the fields whose shortcut is removed, including the deleted fields.
	RemovedShortcuts []string
	// Written are the records written by users or workflows keyed by record UID, true for the created ones, so their system fields changed as well.
	// The writes of the server are excluded, otherwise a shortcut reading the update time would regenerate itself endlessly.
	Written map[string]bool
}

// IsEmpty reports whether nothing is changed.
func (c *Change) IsEmpty() bool {
	return len(c.Records) == 0 && len(c.Fields) == 0 && len(c.Shortcuts) == 0 && len(c.RemovedShortcuts) == 0
}

// AddRecord records the changed fields of the record, nil fields means any field may have changed.
func (c *Change) AddRecord(recordUID string, fieldUIDs []string) {
	if c.Records == nil {
		c.Records = map[string][]string{}
	}
	existing, ok := c.Records[recordUID]
	switch {
	case ok && existing == nil:
	case fieldUIDs == nil:
		c.Records[recordUID] = nil
	default:
		c.Records[recordUID] = lo.Uniq(append(existing, fieldUIDs...))
	}
}

// AddWritten records the record written by the writer, the writes of the server are ignored.
func (c *Change) AddWritten(recordUID string, created bool, writer db.SLRecordWriter) {
	if writer.IsZero() {
		return
	}
	if c.Written == nil {
		c.Written = map[string]bool{}
	}
	c.Written[recordUID] = c.Written[recordUID] || created
}

// ShortcutHooks connects the field shortcuts to the collaboration.
type ShortcutHooks interface {
	// ValidateShortcut checks field.Shortcut against the table fields and returns the normalized shortcut,
	// it returns *OperationError if the shortcut is invalid. fields may not contain the field when it is being added.
	ValidateShortcut(ctx context.Context, fields []*db.SLField, field *db.SLField) (*db.FieldShortcut, error)
	// ShortcutsChanged is called after the transaction commits and releases its row lock.
	ShortcutsChanged(ctx context.Context, project *db.Project, table *db.SLTable, change *Change)
}

// Reject returns the *OperationError rejecting the changeset with the message key.
func Reject(key string, args ...interface{}) error {
	return rejectf(key, args...)
}
