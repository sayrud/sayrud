package collab

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

// OperationError rejects the whole changeset, Key is the message key shown to the user in the language of the connection.
type OperationError struct {
	Key  string
	Args []interface{}
}

func (e *OperationError) Error() string {
	return e.Key
}

func rejectf(key string, args ...interface{}) error {
	return &OperationError{Key: key, Args: args}
}

// applier applies the operations to a table within a transaction.
//
// The actions targeting the resources deleted concurrently are dropped instead of rejected, and the cell values which no longer
// match the field (e.g. the option was removed concurrently) are dropped, the affected data is reported in dirty for the clients to reload.
type applier struct {
	// ctx is the context of the request applying the operations.
	ctx context.Context
	// tx is the transaction the operations are applied in.
	tx *gorm.DB
	// table is the table the operations are applied to.
	table *db.SLTable

	// fields are the current fields of the table, reloaded after the field actions.
	fields []*db.SLField
	// views are the current views of the table.
	views []*db.SLView
	// dirty is the data changed by the server, which the clients should reload.
	dirty DirtyScope
	// change is the data changed by the operations, which the field shortcuts regenerate the dependent cells by.
	change Change

	// boolText returns the text of the checkbox values when converting them to text.
	boolText func(bool) string
	// shortcuts validates the field shortcuts, the shortcuts are rejected if it is nil.
	shortcuts ShortcutHooks
}

func newApplier(ctx context.Context, tx *gorm.DB, table *db.SLTable, boolText func(bool) string, shortcuts ShortcutHooks) (*applier, error) {
	a := &applier{ctx: ctx, tx: tx, table: table, boolText: boolText, shortcuts: shortcuts}
	if err := a.reloadFields(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *applier) reloadFields() error {
	fields, err := db.NewSLFieldsStore(a.tx).ListByTableID(a.ctx, a.table.ID)
	if err != nil {
		return errors.Wrap(err, "list fields")
	}
	a.fields = fields
	return nil
}

func (a *applier) reloadViews() error {
	views, err := db.NewSLViewsStore(a.tx).ListByTableID(a.ctx, a.table.ID)
	if err != nil {
		return errors.Wrap(err, "list views")
	}
	a.views = views
	return nil
}

func (a *applier) field(uid string) *db.SLField {
	field, _ := lo.Find(a.fields, func(f *db.SLField) bool { return f.UID == uid })
	return field
}

// apply applies the operations in order and returns the operations actually applied.
func (a *applier) apply(operations []Operation) ([]Operation, error) {
	applied := make([]Operation, 0, len(operations))
	for _, operation := range operations {
		actions, err := a.applyActions(operation.Actions)
		if err != nil {
			return nil, err
		}
		applied = append(applied, Operation{Command: operation.Command, Actions: actions})
	}
	return applied, nil
}

func (a *applier) applyActions(actions []Action) ([]Action, error) {
	applied := make([]Action, 0, len(actions))
	for i := 0; i < len(actions); {
		// Batch the consecutive record actions of the same kind, e.g. pasting many cells.
		j := i + 1
		kind := actions[i].Action
		if kind == ActionAddRecord || kind == ActionSetRecord {
			for j < len(actions) && actions[j].Action == kind {
				j++
			}
		}

		var kept []Action
		var err error
		switch kind {
		case ActionAddRecord:
			kept, err = a.addRecords(actions[i:j])
		case ActionSetRecord:
			kept, err = a.setRecords(actions[i:j])
		default:
			var action *Action
			action, err = a.applyAction(actions[i])
			if action != nil {
				kept = []Action{*action}
			}
		}
		if err != nil {
			if errors.Is(err, db.ErrOptionReference) {
				return nil, rejectf("field::invalid_options_reference")
			}
			return nil, err
		}
		applied = append(applied, kept...)
		i = j
	}
	return applied, nil
}

// applyAction applies a non-batched action, it returns nil if the action is dropped.
func (a *applier) applyAction(action Action) (*Action, error) {
	switch action.Action {
	case ActionDeleteRecords:
		if err := db.NewSLShortcutJobsStore(a.tx).DeleteByRecords(a.ctx, a.table.ID, action.RecordUIDs); err != nil {
			return nil, errors.Wrap(err, "delete shortcut jobs")
		}
		return &action, db.NewSLRecordsStore(a.tx).DeleteByUIDs(a.ctx, a.table.ID, action.RecordUIDs)
	case ActionAddField:
		return a.addField(action)
	case ActionSetField:
		return a.setField(action)
	case ActionSetFieldType:
		return a.setFieldType(action)
	case ActionSetFieldShortcut:
		return a.setFieldShortcut(action)
	case ActionMoveField:
		return a.moveField(action)
	case ActionDeleteField:
		return a.deleteField(action)
	case ActionAddView:
		return a.addView(action)
	case ActionSetView:
		return a.setView(action)
	case ActionMoveView:
		return a.moveView(action)
	case ActionDeleteView:
		return a.deleteView(action)
	default:
		return nil, rejectf("collab::unsupported_action", action.Action)
	}
}

func (a *applier) addRecords(actions []Action) ([]Action, error) {
	validator := routeutil.NewRecordValidator(a.fields)
	uids := make([]string, 0, len(actions))
	dataList := make([]json.RawMessage, 0, len(actions))
	for i, action := range actions {
		if !recordUIDPattern.MatchString(action.RecordUID) {
			return nil, rejectf("collab::invalid_record_uid")
		}

		data, dropped := validator.Normalize(action.Values)
		referenceChanged, err := validator.NormalizeReferencedOptions(a.ctx, a.tx, data)
		if err != nil {
			return nil, err
		}
		dropped = dropped || referenceChanged

		canonicalized, err := validator.ValidateAttachments(a.ctx, a.tx, a.table.ID, data)
		if err != nil {
			if errors.Is(err, routeutil.ErrInvalidAttachment) {
				return nil, rejectf("attachment::invalid_reference")
			}
			return nil, err
		}
		actions[i].Values = data
		if dropped || canonicalized {
			a.dirty.addRecords(action.RecordUID)
		}
		a.change.AddRecord(action.RecordUID, lo.Keys(data))
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return nil, errors.Wrap(err, "encode data")
		}
		uids = append(uids, action.RecordUID)
		dataList = append(dataList, jsonBytes)
	}

	if _, err := db.NewSLRecordsStore(a.tx).Import(a.ctx, a.table.ID, db.ImportSLRecordsOptions{
		Data: dataList,
		UIDs: uids,
	}); err != nil {
		if errors.Is(err, db.ErrSLRecordExists) {
			return nil, rejectf("collab::record_exists")
		}
		return nil, errors.Wrap(err, "import records")
	}
	return actions, nil
}

func (a *applier) setRecords(actions []Action) ([]Action, error) {
	recordsStore := db.NewSLRecordsStore(a.tx)
	records, err := recordsStore.ListByUIDs(a.ctx, a.table.ID, lo.Uniq(lo.Map(actions, func(action Action, _ int) string { return action.RecordUID })))
	if err != nil {
		return nil, errors.Wrap(err, "list records")
	}

	dataSets := make(map[string]map[string]interface{}, len(records))
	for _, record := range records {
		data := map[string]interface{}{}
		if err := json.Unmarshal(record.Data, &data); err != nil {
			return nil, errors.Wrapf(err, "decode record %q", record.UID)
		}
		dataSets[record.UID] = data
	}

	kept := make([]Action, 0, len(actions))
	for _, action := range actions {
		data, ok := dataSets[action.RecordUID]
		if !ok {
			// The record has been deleted concurrently, the deletion wins.
			continue
		}
		for fieldUID, value := range action.Values {
			if value == nil {
				delete(data, fieldUID)
			} else {
				data[fieldUID] = value
			}
		}
		a.change.AddRecord(action.RecordUID, lo.Keys(action.Values))
		kept = append(kept, action)
	}

	validator := routeutil.NewRecordValidator(a.fields)
	for _, record := range records {
		data, dropped := validator.Normalize(dataSets[record.UID])
		referenceChanged, err := validator.NormalizeReferencedOptions(a.ctx, a.tx, data)
		if err != nil {
			return nil, err
		}
		dropped = dropped || referenceChanged

		canonicalized, err := validator.ValidateAttachments(a.ctx, a.tx, a.table.ID, data)
		if err != nil {
			if errors.Is(err, routeutil.ErrInvalidAttachment) {
				return nil, rejectf("attachment::invalid_reference")
			}
			return nil, err
		}
		for i := range kept {
			if kept[i].RecordUID == record.UID {
				for uid := range kept[i].Values {
					if field := a.field(uid); field != nil && field.Type == db.AttachmentFieldType {
						kept[i].Values[uid] = data[uid]
					}
				}
			}
		}
		if dropped || canonicalized {
			a.dirty.addRecords(record.UID)
		}
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return nil, errors.Wrap(err, "encode data")
		}
		if err := recordsStore.Update(a.ctx, record.ID, jsonBytes); err != nil {
			return nil, errors.Wrap(err, "update record")
		}
	}
	return kept, nil
}

// checkFieldLabel checks the label is not empty, reserved, or used by other fields.
func (a *applier) checkFieldLabel(label, fieldUID string) error {
	switch {
	case label == "":
		return rejectf("collab::field_title_required")
	case label == "_uid":
		return rejectf("collab::field_uid_reserved")
	case lo.ContainsBy(a.fields, func(f *db.SLField) bool { return f.Label == label && f.UID != fieldUID }):
		return rejectf("collab::field_label_exists", label)
	default:
		return nil
	}
}

func (a *applier) addField(action Action) (*Action, error) {
	if !fieldUIDPattern.MatchString(action.FieldUID) {
		return nil, rejectf("collab::invalid_field_uid")
	}
	if action.Field == nil || action.Field.Label == nil || action.Field.Type == nil {
		return nil, rejectf("collab::field_title_type_required")
	}
	label := strings.TrimSpace(*action.Field.Label)
	if err := a.checkFieldLabel(label, action.FieldUID); err != nil {
		return nil, err
	}
	metadata := action.Field.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	if err := db.PrepareSelectMetadata(a.ctx, a.tx, a.table.ID, action.FieldUID, db.SLFieldType(*action.Field.Type), metadata); err != nil {
		return nil, err
	}

	var shortcut *db.FieldShortcut
	if action.Field.Shortcut != nil {
		var err error
		shortcut, err = a.validateShortcut(&db.SLField{
			UID:      action.FieldUID,
			Label:    label,
			Type:     db.SLFieldType(*action.Field.Type),
			Metadata: datatypes.NewJSONType[db.SLFieldMetadata](metadata),
			Shortcut: action.Field.Shortcut,
		})
		if err != nil {
			return nil, err
		}
		action.Field.Shortcut = shortcut
		if !action.KeepValues {
			a.change.Shortcuts = append(a.change.Shortcuts, action.FieldUID)
		}
	}

	fieldsStore := db.NewSLFieldsStore(a.tx)
	field, err := fieldsStore.Create(a.ctx, db.CreateSLFieldOptions{
		UID:       action.FieldUID,
		SLTableID: a.table.ID,
		Label:     label,
		Type:      db.SLFieldType(*action.Field.Type),
		Metadata:  metadata,
		Position:  len(a.fields),
		Shortcut:  shortcut,
	})
	if err != nil {
		switch {
		case errors.Is(err, db.ErrUnexpectedType):
			return nil, rejectf("collab::invalid_field_type")
		case errors.Is(err, db.ErrSLFieldExists):
			return nil, rejectf("collab::field_exists")
		default:
			return nil, errors.Wrap(err, "create field")
		}
	}
	if action.Index != nil && *action.Index < len(a.fields) {
		if err := fieldsStore.Move(a.ctx, a.table.ID, field.ID, *action.Index); err != nil {
			return nil, errors.Wrap(err, "move field")
		}
	}
	action.Field.Label = &label
	return &action, a.reloadFields()
}

func (a *applier) setField(action Action) (*Action, error) {
	field := a.field(action.FieldUID)
	if field == nil || action.Field == nil {
		return nil, nil
	}
	fieldsStore := db.NewSLFieldsStore(a.tx)

	if action.Field.Label != nil {
		label := strings.TrimSpace(*action.Field.Label)
		if err := a.checkFieldLabel(label, field.UID); err != nil {
			return nil, err
		}
		if err := fieldsStore.SetLabel(a.ctx, field.ID, label); err != nil {
			return nil, errors.Wrap(err, "set label")
		}
		action.Field.Label = &label
	}

	if action.Field.Metadata != nil {
		if err := fieldsStore.SetMetadata(a.ctx, field.ID, action.Field.Metadata); err != nil {
			return nil, errors.Wrap(err, "set metadata")
		}
	}
	if err := a.reloadFields(); err != nil {
		return nil, err
	}

	// The values of the removed options are no longer valid.
	if action.Field.Metadata != nil && (field.Type == db.SingleSelectFieldType || field.Type == db.MultiSelectFieldType) {
		if err := a.normalizeFieldValues(a.field(field.UID)); err != nil {
			return nil, err
		}
		a.change.Fields = append(a.change.Fields, field.UID)
		// Category changes affect this shortcut too; color and ordering edits do not need another execution.
		oldMD, _ := field.Metadata.Data().(map[string]interface{})
		newMD, _ := a.field(field.UID).Metadata.Data().(map[string]interface{})
		if field.Shortcut != nil && field.Shortcut.AutoUpdate && (!reflect.DeepEqual(optionNames(field), optionNames(a.field(field.UID))) || !reflect.DeepEqual(oldMD["optionsReference"], newMD["optionsReference"])) {
			a.change.Shortcuts = append(a.change.Shortcuts, field.UID)
		}
	}
	return &action, nil
}

// validateShortcut returns the normalized shortcut of the field, which is not saved yet.
func (a *applier) validateShortcut(field *db.SLField) (*db.FieldShortcut, error) {
	if a.shortcuts == nil {
		return nil, rejectf("collab::shortcut_unavailable")
	}
	fields := lo.Filter(a.fields, func(f *db.SLField, _ int) bool { return f.UID != field.UID })
	return a.shortcuts.ValidateShortcut(a.ctx, append(fields, field), field)
}

func (a *applier) setFieldShortcut(action Action) (*Action, error) {
	field := a.field(action.FieldUID)
	if field == nil {
		return nil, nil
	}

	var shortcut *db.FieldShortcut
	if action.Shortcut != nil {
		draft := *field
		draft.Shortcut = action.Shortcut
		var err error
		if shortcut, err = a.validateShortcut(&draft); err != nil {
			return nil, err
		}

		// Only toggling the auto update keeps the generated values.
		sameConfig := field.Shortcut != nil && field.Shortcut.ID == shortcut.ID && reflect.DeepEqual(field.Shortcut.Inputs, shortcut.Inputs)
		if !sameConfig && !action.KeepValues {
			a.change.Shortcuts = append(a.change.Shortcuts, field.UID)
		}
	} else {
		if err := db.NewSLShortcutJobsStore(a.tx).DeleteByField(a.ctx, a.table.ID, field.UID); err != nil {
			return nil, errors.Wrap(err, "delete shortcut jobs")
		}
		a.change.RemovedShortcuts = append(a.change.RemovedShortcuts, field.UID)
	}

	if err := db.NewSLFieldsStore(a.tx).SetShortcut(a.ctx, field.ID, shortcut); err != nil {
		return nil, errors.Wrap(err, "set shortcut")
	}
	action.Shortcut = shortcut
	return &action, a.reloadFields()
}

// normalizeFieldValues drops the values of the field which no longer match the field, and reports the changed records as dirty.
func (a *applier) normalizeFieldValues(field *db.SLField) error {
	return a.updateFieldValues(field.UID, func(record *db.SLRecord, value interface{}) (interface{}, bool) {
		if routeutil.CheckValue(field, value) {
			return value, false
		}
		if values, ok := value.([]interface{}); ok && field.Type == db.MultiSelectFieldType {
			options := routeutil.OptionUIDs(field)
			kept := lo.Filter(values, func(v interface{}, _ int) bool {
				s, ok := v.(string)
				return ok && lo.Contains(options, s)
			})
			if len(kept) > 0 {
				return kept, true
			}
		}
		return nil, true
	})
}

// updateFieldValues updates the value of the field in all the records, update returns the new value and whether it changes.
func (a *applier) updateFieldValues(fieldUID string, update func(record *db.SLRecord, value interface{}) (interface{}, bool)) error {
	recordsStore := db.NewSLRecordsStore(a.tx)
	records, err := recordsStore.ListAll(a.ctx, a.table.ID)
	if err != nil {
		return errors.Wrap(err, "list records")
	}

	for _, record := range records {
		data := map[string]interface{}{}
		if err := json.Unmarshal(record.Data, &data); err != nil {
			return errors.Wrapf(err, "decode record %q", record.UID)
		}
		value, ok := data[fieldUID]
		next, changed := update(record, value)
		if !changed {
			continue
		}
		if next == nil {
			if !ok {
				continue
			}
			delete(data, fieldUID)
		} else {
			data[fieldUID] = next
		}

		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return errors.Wrap(err, "encode data")
		}
		if err := recordsStore.Update(a.ctx, record.ID, jsonBytes); err != nil {
			return errors.Wrap(err, "update record")
		}
		a.dirty.addRecords(record.UID)
	}
	return nil
}

func (a *applier) setFieldType(action Action) (*Action, error) {
	field := a.field(action.FieldUID)
	if field == nil || action.Field == nil || action.Field.Type == nil {
		return nil, nil
	}
	newType := db.SLFieldType(*action.Field.Type)
	if !newType.Check() {
		return nil, rejectf("collab::invalid_field_type")
	}
	metadata := action.Field.Metadata
	if metadata == nil {
		metadata = map[string]interface{}{}
	}

	records, err := db.NewSLRecordsStore(a.tx).ListAll(a.ctx, a.table.ID)
	if err != nil {
		return nil, errors.Wrap(err, "list records")
	}
	conversion, err := convertField(field, newType, metadata, records, a.boolText)
	if err != nil {
		return nil, errors.Wrap(err, "convert field")
	}

	if err := db.NewSLFieldsStore(a.tx).SetType(a.ctx, field.ID, newType, conversion.metadata); err != nil {
		return nil, errors.Wrap(err, "set type")
	}
	if err := a.reloadFields(); err != nil {
		return nil, err
	}
	if err := a.updateFieldValues(field.UID, func(record *db.SLRecord, _ interface{}) (interface{}, bool) {
		value, ok := conversion.values[record.UID]
		return value, ok
	}); err != nil {
		return nil, err
	}

	// The metadata may be changed by the conversion, e.g. the options generated from the text values.
	a.dirty.Fields = true
	a.change.Fields = append(a.change.Fields, field.UID)

	// The shortcut may not support the new type, it is removed then and the client sets a new one in the same changeset if needed.
	// Otherwise the converted values are regenerated in the new type.
	if converted := a.field(field.UID); converted.Shortcut != nil {
		if _, err := a.validateShortcut(converted); err != nil {
			var operationErr *OperationError
			if !errors.As(err, &operationErr) {
				return nil, err
			}
			if err := a.removeShortcut(converted); err != nil {
				return nil, err
			}
		} else {
			a.change.Shortcuts = append(a.change.Shortcuts, field.UID)
		}
	}
	return &action, nil
}

// hasShortcuts reports whether any field of the table has a shortcut.
func (a *applier) hasShortcuts() bool {
	return lo.ContainsBy(a.fields, func(f *db.SLField) bool { return f.Shortcut != nil })
}

func (a *applier) removeShortcut(field *db.SLField) error {
	if err := db.NewSLFieldsStore(a.tx).SetShortcut(a.ctx, field.ID, nil); err != nil {
		return errors.Wrap(err, "remove shortcut")
	}
	if err := db.NewSLShortcutJobsStore(a.tx).DeleteByField(a.ctx, a.table.ID, field.UID); err != nil {
		return errors.Wrap(err, "delete shortcut jobs")
	}
	a.change.RemovedShortcuts = append(a.change.RemovedShortcuts, field.UID)
	return a.reloadFields()
}

func (a *applier) moveField(action Action) (*Action, error) {
	field := a.field(action.FieldUID)
	if field == nil || action.Index == nil {
		return nil, nil
	}
	if err := db.NewSLFieldsStore(a.tx).Move(a.ctx, a.table.ID, field.ID, *action.Index); err != nil {
		return nil, errors.Wrap(err, "move field")
	}
	return &action, a.reloadFields()
}

func (a *applier) deleteField(action Action) (*Action, error) {
	field := a.field(action.FieldUID)
	if field == nil {
		return nil, nil
	}
	if a.fields[0].UID == field.UID {
		return nil, rejectf("collab::primary_field_undeletable")
	}
	if err := db.NewSLFieldsStore(a.tx).DeleteByID(a.ctx, field.ID); err != nil {
		return nil, errors.Wrap(err, "delete field")
	}
	if err := db.NewSLRecordsStore(a.tx).RemoveFieldData(a.ctx, a.table.ID, field.UID); err != nil {
		return nil, errors.Wrap(err, "remove field data")
	}
	if err := db.NewSLShortcutJobsStore(a.tx).DeleteByField(a.ctx, a.table.ID, field.UID); err != nil {
		return nil, errors.Wrap(err, "delete shortcut jobs")
	}
	if field.Shortcut != nil {
		a.change.RemovedShortcuts = append(a.change.RemovedShortcuts, field.UID)
	}
	return &action, a.reloadFields()
}

func (a *applier) view(uid string) (*db.SLView, error) {
	if a.views == nil {
		if err := a.reloadViews(); err != nil {
			return nil, err
		}
	}
	view, _ := lo.Find(a.views, func(v *db.SLView) bool { return v.UID == uid })
	return view, nil
}

func (a *applier) addView(action Action) (*Action, error) {
	if !viewUIDPattern.MatchString(action.ViewUID) {
		return nil, rejectf("collab::invalid_view_uid")
	}
	if action.View == nil || action.View.Name == nil || action.View.Type == nil {
		return nil, rejectf("collab::view_name_type_required")
	}
	name := strings.TrimSpace(*action.View.Name)
	if name == "" {
		return nil, rejectf("collab::view_name_required")
	}
	config, err := json.Marshal(lo.Ternary(action.View.Config == nil, map[string]interface{}{}, action.View.Config))
	if err != nil {
		return nil, errors.Wrap(err, "encode config")
	}
	if err := a.reloadViews(); err != nil {
		return nil, err
	}

	viewsStore := db.NewSLViewsStore(a.tx)
	view, err := viewsStore.Create(a.ctx, db.CreateSLViewOptions{
		UID:       action.ViewUID,
		SLTableID: a.table.ID,
		Name:      name,
		Type:      db.SLViewType(*action.View.Type),
		Config:    config,
		Position:  len(a.views),
	})
	if err != nil {
		switch {
		case errors.Is(err, db.ErrUnexpectedSLView):
			return nil, rejectf("collab::invalid_view_type")
		case errors.Is(err, db.ErrSLViewExists):
			return nil, rejectf("collab::view_exists")
		default:
			return nil, errors.Wrap(err, "create view")
		}
	}
	if action.Index != nil && *action.Index < len(a.views) {
		if err := viewsStore.Move(a.ctx, a.table.ID, view.ID, *action.Index); err != nil {
			return nil, errors.Wrap(err, "move view")
		}
	}
	action.View.Name = &name
	return &action, a.reloadViews()
}

func (a *applier) setView(action Action) (*Action, error) {
	view, err := a.view(action.ViewUID)
	if err != nil || view == nil || action.View == nil {
		return nil, err
	}

	options := db.UpdateSLViewOptions{}
	if action.View.Name != nil {
		name := strings.TrimSpace(*action.View.Name)
		if name == "" {
			return nil, rejectf("collab::view_name_required")
		}
		action.View.Name = &name
		options.Name = &name
	}
	if action.View.Config != nil {
		options.Config, err = json.Marshal(action.View.Config)
		if err != nil {
			return nil, errors.Wrap(err, "encode config")
		}
	}
	if err := db.NewSLViewsStore(a.tx).Update(a.ctx, view.ID, options); err != nil {
		return nil, errors.Wrap(err, "update view")
	}
	return &action, nil
}

func (a *applier) moveView(action Action) (*Action, error) {
	view, err := a.view(action.ViewUID)
	if err != nil || view == nil || action.Index == nil {
		return nil, err
	}
	if err := db.NewSLViewsStore(a.tx).Move(a.ctx, a.table.ID, view.ID, *action.Index); err != nil {
		return nil, errors.Wrap(err, "move view")
	}
	return &action, a.reloadViews()
}

func (a *applier) deleteView(action Action) (*Action, error) {
	view, err := a.view(action.ViewUID)
	if err != nil || view == nil {
		return nil, err
	}
	if len(a.views) <= 1 {
		return nil, rejectf("collab::keep_one_view")
	}
	if err := db.NewSLViewsStore(a.tx).DeleteByID(a.ctx, view.ID); err != nil {
		return nil, errors.Wrap(err, "delete view")
	}
	return &action, a.reloadViews()
}
