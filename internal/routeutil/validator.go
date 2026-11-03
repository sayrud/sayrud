// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routeutil

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
)

var (
	ErrFieldTypeMismatch = errors.New("field type mismatch")
	ErrFieldReadOnly     = errors.New("field is read-only")
	ErrInvalidMember     = errors.New("user is not a member of the project")
)

// maxUserID is the largest user ID the member values can hold, the IDs are JSON numbers which JavaScript reads as doubles.
const maxUserID = 1<<53 - 1

// Validate checks the cell values keyed by field UID against the table fields, and returns the data to be stored.
// Empty values, unchecked checkboxes and formula values are dropped, as they are not stored. previous is the stored data
// of the record being replaced, nil when creating; the members already in the previous value of a cell are kept after they left.
// It returns db.ErrSLFieldNotFound if any key is not a field of the table, ErrFieldTypeMismatch if any value mismatches its field type,
// ErrFieldReadOnly if any system field has a value, and ErrInvalidMember if any member value has a user out of the project.
func Validate(ctx context.Context, tx *gorm.DB, tableID int64, data map[string]interface{}, previous json.RawMessage) (json.RawMessage, error) {
	fields, err := db.NewSLFieldsStore(tx).ListByTableID(ctx, tableID)
	if err != nil {
		return nil, errors.Wrap(err, "list fields")
	}

	validator := NewRecordValidator(fields)
	result, err := validator.Validate(data)
	if err != nil {
		return nil, err
	}
	if len(validator.memberFields) > 0 {
		table, err := db.NewSLTablesStore(tx).GetByID(ctx, tableID)
		if err != nil {
			return nil, errors.Wrap(err, "get table")
		}
		before := map[string]interface{}{}
		if len(previous) > 0 {
			_ = json.Unmarshal(previous, &before)
		}
		if err := validator.ValidateMembers(ctx, tx, table.ProjectID, result, before); err != nil {
			return nil, err
		}
	}
	if _, err := validator.ValidateAttachments(ctx, tx, tableID, result); err != nil {
		return nil, err
	}
	if changed, err := validator.NormalizeReferencedOptions(ctx, tx, result); err != nil {
		return nil, err
	} else if changed {
		return nil, ErrFieldTypeMismatch
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "encode data")
	}
	return jsonBytes, nil
}

// NormalizeReferencedOptions applies the current record's cascading conditions, including when a parent value changes.
func (v *RecordValidator) NormalizeReferencedOptions(ctx context.Context, tx *gorm.DB, data map[string]interface{}) (bool, error) {
	changed := false
	for _, field := range v.optionFields {
		uid := field.UID
		value, exists := data[uid]
		if !exists {
			continue
		}

		filtered, err := db.FilterReferencedOptions(ctx, tx, field, lo.Values(v.fields), data)
		if err != nil {
			if errors.Is(err, db.ErrOptionReference) {
				continue
			}
			return false, err
		}
		if CheckValue(filtered, value) {
			continue
		}

		delete(data, uid)
		if list, ok := value.([]interface{}); ok && field.Type == db.MultiSelectFieldType {
			allowed := OptionUIDs(filtered)
			kept := lo.Filter(list, func(raw interface{}, _ int) bool {
				id, ok := raw.(string)
				return ok && lo.Contains(allowed, id)
			})
			if len(kept) > 0 {
				data[uid] = kept
			}
		}
		changed = true
	}

	return changed, nil
}

// RecordValidator checks the record data against the fields of a table.
type RecordValidator struct {
	fields       map[string]*db.SLField
	optionFields []*db.SLField
	memberFields []*db.SLField
	// members are the user IDs of the project, loaded by the first member value to check.
	members map[int64]bool
}

func NewRecordValidator(fields []*db.SLField) *RecordValidator {
	v := &RecordValidator{
		fields:       lo.KeyBy(fields, func(field *db.SLField) string { return field.UID }),
		memberFields: lo.Filter(fields, func(field *db.SLField, _ int) bool { return field.Type == db.MemberFieldType }),
	}

	// Normalize parent fields before their dependent option fields.
	visited := map[string]bool{}
	var visit func(string)
	visit = func(uid string) {
		if visited[uid] {
			return
		}

		visited[uid] = true
		f := v.fields[uid]
		if f == nil {
			return
		}

		md, _ := f.Metadata.Data().(map[string]interface{})
		ref, _ := db.ReadOptionReference(md)
		if ref == nil {
			return
		}

		for _, c := range ref.Conditions {
			if c.ValueFieldUID != "" {
				visit(c.ValueFieldUID)
			}
		}

		v.optionFields = append(v.optionFields, f)
	}

	for _, f := range fields {
		visit(f.UID)
	}

	return v
}

// Validate returns the data to be stored, see the Validate function for the rules.
func (v *RecordValidator) Validate(data map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(data))
	for uid, value := range data {
		field, ok := v.fields[uid]
		if !ok {
			return nil, errors.Wrapf(db.ErrSLFieldNotFound, "field %q", uid)
		}
		if field.Type.IsSystem() && !isEmptyValue(value) {
			return nil, errors.Wrapf(ErrFieldReadOnly, "field %q", field.Label)
		}
		if !IsStoredValue(field, value) {
			continue
		}
		if !CheckValue(field, value) {
			return nil, errors.Wrapf(ErrFieldTypeMismatch, "field %q", field.Label)
		}
		result[uid] = value
	}
	return result, nil
}

// ReadOnlyField returns the first system field given a non-empty value, or nil if there is none.
func (v *RecordValidator) ReadOnlyField(data map[string]interface{}) *db.SLField {
	for uid, value := range data {
		if field, ok := v.fields[uid]; ok && field.Type.IsSystem() && !isEmptyValue(value) {
			return field
		}
	}
	return nil
}

// ValidateMembers checks every user of the member values is the owner or a collaborator of the project, except the ones already in previous of the same cell.
// It returns ErrInvalidMember wrapping the field label if not.
func (v *RecordValidator) ValidateMembers(ctx context.Context, tx *gorm.DB, projectID int64, data, previous map[string]interface{}) error {
	for _, field := range v.memberFields {
		ids := MemberIDs(data[field.UID])
		kept := lo.SliceToMap(MemberIDs(previous[field.UID]), func(id int64) (int64, bool) { return id, true })
		for _, id := range ids {
			if kept[id] {
				continue
			}
			if v.members == nil {
				userIDs, err := db.NewProjectMembersStore(tx).UserIDs(ctx, projectID)
				if err != nil {
					return errors.Wrap(err, "list members")
				}
				v.members = lo.SliceToMap(userIDs, func(id int64) (int64, bool) { return id, true })
			}
			if !v.members[id] {
				return errors.Wrapf(ErrInvalidMember, "field %q", field.Label)
			}
		}
	}
	return nil
}

// MemberIDs returns the user IDs of a member value, the invalid items are skipped.
func MemberIDs(value interface{}) []int64 {
	values, _ := value.([]interface{})
	ids := make([]int64, 0, len(values))
	for _, item := range values {
		if id, ok := memberID(item); ok {
			ids = append(ids, id)
		}
	}
	return ids
}

func memberID(value interface{}) (int64, bool) {
	v, ok := value.(float64)
	if !ok || v < 1 || v > maxUserID || v != math.Trunc(v) {
		return 0, false
	}
	return int64(v), true
}

// Normalize is the lenient version of Validate, it drops the unknown fields and the mismatched values instead of returning an error.
// dropped reports whether any non-empty value is dropped.
func (v *RecordValidator) Normalize(data map[string]interface{}) (result map[string]interface{}, dropped bool) {
	result = make(map[string]interface{}, len(data))
	for uid, value := range data {
		field, ok := v.fields[uid]
		if !ok {
			dropped = dropped || !isEmptyValue(value)
			continue
		}
		if !IsStoredValue(field, value) {
			continue
		}
		if !CheckValue(field, value) {
			dropped = true
			continue
		}
		result[uid] = value
	}
	return result, dropped
}

// IsStoredValue reports whether the value needs to be stored: empty values, unchecked checkboxes, formula and system values are not stored.
func IsStoredValue(field *db.SLField, value interface{}) bool {
	if field.Type == db.FormulaFieldType || field.Type.IsSystem() || isEmptyValue(value) {
		return false
	}
	return field.Type != db.CheckboxFieldType || value != false
}

func isEmptyValue(value interface{}) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []interface{}:
		return len(v) == 0
	default:
		return false
	}
}

// CheckValue reports whether the non-empty value matches the field type.
func CheckValue(field *db.SLField, value interface{}) bool {
	switch field.Type {
	case db.TextFieldType:
		_, ok := value.(string)
		return ok
	case db.NumberFieldType:
		v, ok := value.(float64)
		return ok && !math.IsNaN(v) && !math.IsInf(v, 0)
	case db.CheckboxFieldType:
		_, ok := value.(bool)
		return ok
	case db.DateTimeFieldType:
		v, ok := value.(string)
		if !ok {
			return false
		}
		_, err := time.Parse(time.RFC3339, v)
		return err == nil
	case db.SingleSelectFieldType:
		v, ok := value.(string)
		return ok && lo.Contains(OptionUIDs(field), v)
	case db.MultiSelectFieldType:
		values, ok := value.([]interface{})
		if !ok {
			return false
		}
		options := OptionUIDs(field)
		for _, value := range values {
			v, ok := value.(string)
			if !ok || !lo.Contains(options, v) {
				return false
			}
		}
		return true
	case db.AttachmentFieldType:
		return checkAttachmentValue(value)
	case db.MemberFieldType:
		values, ok := value.([]interface{})
		if !ok || (len(values) > 1 && !db.MemberMultiple(field)) {
			return false
		}
		seen := make(map[int64]bool, len(values))
		for _, item := range values {
			id, ok := memberID(item)
			if !ok || seen[id] {
				return false
			}
			seen[id] = true
		}
		return true
	default:
		return false
	}
}

// OptionUIDs returns the option UIDs of the select field, which are stored in metadata as `{"options": [{"uid": "..."}]}`.
func OptionUIDs(field *db.SLField) []string {
	metadata, _ := field.Metadata.Data().(map[string]interface{})
	options, _ := metadata["options"].([]interface{})

	uids := make([]string, 0, len(options))
	for _, option := range options {
		option, _ := option.(map[string]interface{})
		if uid, ok := option["uid"].(string); ok {
			uids = append(uids, uid)
		}
	}
	return uids
}
