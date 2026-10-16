package collab

import (
	"context"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"github.com/thanhpk/randstr"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/importer"
)

// importChunkSize keeps the number of the bind parameters of an insert statement far below the limit of PostgreSQL.
const importChunkSize = 1000

// ImportableFieldTypes are the field types the imported values can be converted to.
var ImportableFieldTypes = []db.SLFieldType{
	db.TextFieldType, db.NumberFieldType, db.SingleSelectFieldType, db.MultiSelectFieldType, db.DateTimeFieldType, db.CheckboxFieldType,
}

// ImportColumn maps a column of the imported rows to an existing field, or to a new field with the label and type if FieldUID is empty.
type ImportColumn struct {
	// Index is the zero-based index of the column in the rows.
	Index int
	// FieldUID is the existing target field.
	FieldUID string
	// Label is the title of the new field.
	Label string
	// Type is the type of the new field.
	Type db.SLFieldType
}

// ImportOptions are the options of Hub.Import.
type ImportOptions struct {
	// Rows are the data rows, excluding the header row.
	Rows [][]*importer.Cell
	// Columns are the imported columns and their target fields.
	Columns []ImportColumn
	// ViewName adds a grid view with the name if not empty, e.g. for the table created by the import.
	ViewName string
	// Location is the time zone of the dates without a time zone.
	Location *time.Location
	// Progress is called after each chunk of the records is inserted, the changes are not visible to other connections before committing.
	Progress func(done, total int)
}

// Import adds the new fields, the missing select options and the records of the rows in a single transaction.
// The field changes are broadcast as a changeset, while the records are announced by a dirty scope reloading all the records,
// so large imports never produce huge changesets. It returns the number of records added, or *OperationError if rejected.
func (h *Hub) Import(ctx context.Context, project *db.Project, table *db.SLTable, opts ImportOptions) (int, error) {
	if opts.Location == nil {
		opts.Location = time.Local
	}

	var changesets []*Changeset
	var schema []Operation
	var change Change
	queued := false
	added := 0
	if err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := dbutil.LockTable(ctx, tx, table.ID); err != nil {
			return err
		}
		a, err := newApplier(ctx, tx, table, (*Client)(nil).boolText, h.shortcuts)
		if err != nil {
			return err
		}

		actions, targets, err := importSchema(a.fields, opts)
		if err != nil {
			return err
		}
		applied, err := a.applyActions(actions)
		if err != nil {
			return err
		}
		if len(applied) > 0 {
			schema = []Operation{{
				Command: "ImportFields",
				Actions: applied,
			}}
			changeset, err := appendChangeset(ctx, tx, table, "", "", schema)
			if err != nil {
				return err
			}
			changesets = append(changesets, changeset)
		}

		fields := make([]*db.SLField, len(targets))
		for i, uid := range targets {
			if fields[i] = a.field(uid); fields[i] == nil {
				return rejectf("import::field_not_found")
			}
		}
		records := importRecords(opts, fields)
		for start := 0; start < len(records); start += importChunkSize {
			if err := ctx.Err(); err != nil {
				return err
			}
			chunk := records[start:min(start+importChunkSize, len(records))]
			if _, err := a.addRecords(chunk); err != nil {
				return err
			}
			added += len(chunk)
			if opts.Progress != nil {
				opts.Progress(added, len(records))
			}
		}

		dirty := a.dirty
		dirty.AllRecords = true
		dirty.Records = nil
		changeset, err := appendChangeset(ctx, tx, table, "", "", []Operation{{
			Command: "Refresh",
			Actions: []Action{{
				Action: ActionDirty,
				Dirty:  &dirty,
			}},
		}})
		if err != nil {
			return err
		}
		changesets = append(changesets, changeset)

		if queued, err = h.recordsChanged(ctx, tx, project, table, Origin{}, a.events); err != nil {
			return err
		}
		if a.hasShortcuts() {
			change = a.change
		}
		return nil
	}); err != nil {
		if errors.Is(err, db.ErrOptionReference) {
			return 0, rejectf("field::invalid_options_reference")
		}
		return 0, err
	}

	for _, changeset := range changesets {
		h.broadcastChangeset(project, changeset, nil)
	}
	h.NotifyChange(ctx, project, table, &change)
	// The records may be the source of the options referenced by other fields.
	h.notifyOptionReferences(ctx, project, table, append(schema, Operation{
		Actions: []Action{{
			Action: ActionDirty,
			Dirty: &DirtyScope{
				Fields:     true,
				AllRecords: true,
			},
		}},
	}))
	h.runsQueued(queued)
	return added, nil
}

// importSchema returns the actions adding the new fields, the missing options and the view, and the target field UIDs of the columns.
func importSchema(fields []*db.SLField, opts ImportOptions) ([]Action, []string, error) {
	var actions []Action
	targets := make([]string, 0, len(opts.Columns))
	for _, c := range opts.Columns {
		cells := importer.Column(opts.Rows, c.Index)
		if c.FieldUID == "" {
			if !lo.Contains(ImportableFieldTypes, c.Type) {
				return nil, nil, rejectf("collab::invalid_field_type")
			}
			uid := "fld" + randstr.String(7)
			label := strings.TrimSpace(c.Label)
			typ := string(c.Type)
			actions = append(actions, Action{
				Action:   ActionAddField,
				FieldUID: uid,
				Field: &FieldAttrs{
					Label:    &label,
					Type:     &typ,
					Metadata: importMetadata(c.Type, cells),
				},
			})
			targets = append(targets, uid)
			continue
		}

		field, ok := lo.Find(fields, func(f *db.SLField) bool { return f.UID == c.FieldUID })
		if !ok || !lo.Contains(ImportableFieldTypes, field.Type) || lo.Contains(targets, c.FieldUID) {
			return nil, nil, rejectf("import::field_not_found")
		}
		metadata, _ := field.Metadata.Data().(map[string]interface{})
		isSelect := field.Type == db.SingleSelectFieldType || field.Type == db.MultiSelectFieldType
		// Fields with referenced options only accept the existing options, the other values are dropped.
		if isSelect && metadata["optionsReference"] == nil {
			next := appendOptions(metadata, field.Type, cellTexts(cells))
			if len(next["options"].([]interface{})) > len(optionNames(field)) {
				actions = append(actions, Action{
					Action:   ActionSetField,
					FieldUID: field.UID,
					Field:    &FieldAttrs{Metadata: next},
				})
			}
		}
		targets = append(targets, field.UID)
	}

	if opts.ViewName != "" {
		name := opts.ViewName
		typ := "grid"
		actions = append(actions, Action{
			Action:  ActionAddView,
			ViewUID: "viw" + randstr.String(10),
			View: &ViewAttrs{
				Name: &name,
				Type: &typ,
				Config: map[string]interface{}{
					"filter":       []interface{}{},
					"conjunction":  "and",
					"sort":         []interface{}{},
					"group":        []interface{}{},
					"hiddenFields": []interface{}{},
					"fieldWidths":  map[string]interface{}{},
					"rowHeight":    "short",
					"frozenCount":  1,
					"summary":      map[string]interface{}{},
				},
			},
		})
	}
	return actions, targets, nil
}

// cellTexts returns the texts of the non-empty cells.
func cellTexts(cells []*importer.Cell) []string {
	texts := make([]string, 0, len(cells))
	for _, c := range cells {
		if c != nil {
			texts = append(texts, c.Text)
		}
	}
	return texts
}

// importMetadata returns the metadata of a new field holding the cells, the select options are created from the values.
func importMetadata(fieldType db.SLFieldType, cells []*importer.Cell) map[string]interface{} {
	switch fieldType {
	case db.NumberFieldType:
		return map[string]interface{}{
			"format":  importer.NumberFormat(cells),
			"default": nil,
		}
	case db.DateTimeFieldType:
		format, withTime := importer.DateFormat(cells)
		return map[string]interface{}{
			"format":    format,
			"with_time": withTime,
			"default":   "",
		}
	case db.CheckboxFieldType:
		return map[string]interface{}{}
	case db.SingleSelectFieldType:
		metadata := map[string]interface{}{
			"options": []interface{}{},
			"default": "",
		}
		return appendOptions(metadata, fieldType, cellTexts(cells))
	case db.MultiSelectFieldType:
		metadata := map[string]interface{}{
			"options": []interface{}{},
			"default": []interface{}{},
		}
		return appendOptions(metadata, fieldType, cellTexts(cells))
	default:
		return map[string]interface{}{"default": ""}
	}
}

// importRecords converts the rows to record.add actions, the rows empty in all the mapped columns are skipped.
func importRecords(opts ImportOptions, fields []*db.SLField) []Action {
	actions := make([]Action, 0, len(opts.Rows))
	for _, row := range opts.Rows {
		values := map[string]interface{}{}
		for i, c := range opts.Columns {
			if c.Index >= len(row) {
				continue
			}
			if v := importValue(fields[i], row[c.Index], opts.Location); v != nil {
				values[fields[i].UID] = v
			}
		}
		if len(values) > 0 {
			actions = append(actions, Action{
				Action:    ActionAddRecord,
				RecordUID: "rec" + randstr.String(11),
				Values:    values,
			})
		}
	}
	return actions
}

// importValue converts the cell to the value of the field, the typed spreadsheet values are used before parsing the text.
// It returns nil for the empty values, including unchecked checkboxes.
func importValue(field *db.SLField, cell *importer.Cell, loc *time.Location) interface{} {
	if cell == nil {
		return nil
	}
	var v interface{}
	switch field.Type {
	case db.TextFieldType:
		v = cell.Text
	case db.NumberFieldType:
		switch cell.Kind {
		case importer.KindNumber:
			v = cell.Number
		case importer.KindBool:
			v = lo.Ternary(cell.Bool, 1.0, 0.0)
		default:
			// Unlike typing into a percent field, "15" is imported as is and only "15%" means 0.15.
			if n, ok := parseNumber(cell.Text, ""); ok {
				v = n
			}
		}
	case db.DateTimeFieldType:
		if cell.Kind == importer.KindDate {
			t := cell.Time
			v = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc).UTC().Format(storedTimeLayout)
		} else if s, ok := parseDateIn(cell.Text, loc); ok {
			v = s
		}
	case db.CheckboxFieldType:
		switch cell.Kind {
		case importer.KindBool:
			v = cell.Bool
		case importer.KindNumber:
			v = cell.Number != 0
		default:
			v = lo.Contains(importer.CheckboxTrueTexts, strings.ToLower(cell.Text))
		}
		if v == false {
			return nil
		}
	default:
		v = textToValue(field, cell.Text)
		if uids, ok := v.([]interface{}); ok && len(uids) == 0 {
			return nil
		}
	}
	return v
}
