package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var ErrOptionReference = errors.New("invalid options reference")

type OptionCondition struct {
	FieldUID      string          `json:"fieldUID"`
	Operation     FilterOperation `json:"operation"`
	Value         string          `json:"value"`
	ValueFieldUID string          `json:"valueFieldUID,omitempty"`
}

type OptionReference struct {
	TableUID   string            `json:"tableUID"`
	FieldUID   string            `json:"fieldUID"`
	Match      string            `json:"match,omitempty"`
	Conditions []OptionCondition `json:"conditions"`
}

func ReadOptionReference(metadata map[string]interface{}) (*OptionReference, error) {
	raw := metadata["optionsReference"]
	if raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, ErrOptionReference
	}

	var ref OptionReference
	if json.Unmarshal(b, &ref) != nil || ref.TableUID == "" || ref.FieldUID == "" || len(ref.Conditions) > 20 {
		return nil, ErrOptionReference
	}
	if ref.Match != "" && ref.Match != "all" && ref.Match != "any" {
		return nil, ErrOptionReference
	}

	return &ref, nil
}

func selectOptions(metadata map[string]interface{}) []interface{} {
	options, _ := metadata["options"].([]interface{})
	return options
}

// PrepareSelectMetadata validates the reference and replaces editable copies with current source options.
// Matching names retain existing cell IDs when a manually configured field first enables referencing.
func PrepareSelectMetadata(ctx context.Context, tx *gorm.DB, tableID int64, uid string, typ SLFieldType, metadata map[string]interface{}) error {
	ref, err := ReadOptionReference(metadata)
	if err != nil || ref == nil {
		return err
	}
	if typ != SingleSelectFieldType && typ != MultiSelectFieldType {
		return ErrOptionReference
	}

	table, err := NewSLTablesStore(tx).GetByID(ctx, tableID)
	if err != nil {
		return err
	}
	sourceTable, source, err := optionSource(ctx, tx, table.ProjectID, ref)
	if err != nil {
		return err
	}
	fields, err := NewSLFieldsStore(tx).ListByTableID(ctx, sourceTable.ID)
	if err != nil {
		return err
	}
	targets, err := NewSLFieldsStore(tx).ListByTableID(ctx, tableID)
	if err != nil {
		return err
	}

	byUID := map[string]*SLField{}
	for _, f := range fields {
		byUID[f.UID] = f
	}

	targetByUID := map[string]*SLField{}
	for _, f := range targets {
		targetByUID[f.UID] = f
	}

	for _, c := range ref.Conditions {
		f := byUID[c.FieldUID]
		if f == nil || f.Type == FormulaFieldType || f.Type == AttachmentFieldType {
			return ErrOptionReference
		}

		value := c.Value
		if c.ValueFieldUID == "" && value == "" && c.Operation != FilterOperationEmpty && c.Operation != FilterOperationNotEmpty {
			return ErrOptionReference
		}

		if c.ValueFieldUID != "" {
			t := targetByUID[c.ValueFieldUID]
			if t == nil || t.UID == uid || t.Type == FormulaFieldType || t.Type == AttachmentFieldType {
				return ErrOptionReference
			}

			isSelect := func(typ SLFieldType) bool { return typ == SingleSelectFieldType || typ == MultiSelectFieldType }
			if f.Type != t.Type && !(isSelect(f.Type) && isSelect(t.Type)) && !(f.Type == TextFieldType && isSelect(t.Type)) && !(isSelect(f.Type) && t.Type == TextFieldType) {
				return ErrOptionReference
			}

			// Validate the operation without requiring an existing target record.
			switch f.Type {
			case NumberFieldType:
				value = "0"
			case DateTimeFieldType:
				value = "2026-01-01T00:00:00Z"
			case CheckboxFieldType:
				value = "true"
			default:
				value = "x"
			}
			if c.Operation == FilterOperationIn || c.Operation == FilterOperationNotIn {
				value = "[" + fmt.Sprintf("%q", value) + "]"
			}
		}

		if _, _, err := buildSLRecordFilter(f, QuerySLRecordsFilter{FieldUID: f.UID, Operation: c.Operation, Value: value}); err != nil {
			return ErrOptionReference
		}
	}

	// Walk both source and local condition dependencies, rejecting circular option references.
	seen := map[string]bool{}
	var visit func(string) error
	visit = func(next string) error {
		if next == uid {
			return ErrOptionReference
		}
		if seen[next] {
			return nil
		}

		seen[next] = true
		f, err := NewSLFieldsStore(tx).GetByUID(ctx, next)
		if err != nil {
			return err
		}

		md, _ := f.Metadata.Data().(map[string]interface{})
		r, err := ReadOptionReference(md)
		if err != nil {
			return err
		}
		if r == nil {
			return nil
		}

		if err := visit(r.FieldUID); err != nil {
			return err
		}
		for _, c := range r.Conditions {
			if c.ValueFieldUID != "" {
				if err := visit(c.ValueFieldUID); err != nil {
					return err
				}
			}
		}

		return nil
	}

	if err := visit(ref.FieldUID); err != nil {
		return err
	}
	for _, c := range ref.Conditions {
		if c.ValueFieldUID != "" {
			if err := visit(c.ValueFieldUID); err != nil {
				return err
			}
		}
	}

	md, _ := source.Metadata.Data().(map[string]interface{})
	metadata["options"] = referencedOptions(selectOptions(metadata), selectOptions(md))

	// Referenced fields do not have a default independent of the conditions.
	if typ == SingleSelectFieldType {
		metadata["default"] = ""
	} else {
		metadata["default"] = []interface{}{}
	}

	return nil
}

func optionSource(ctx context.Context, tx *gorm.DB, projectID int64, ref *OptionReference) (*SLTable, *SLField, error) {
	table, err := NewSLTablesStore(tx).GetByUID(ctx, ref.TableUID)
	if err != nil {
		if errors.Is(err, ErrSLTableNotFound) {
			err = ErrOptionReference
		}
		return nil, nil, err
	}
	if table.ProjectID != projectID {
		return nil, nil, ErrOptionReference
	}

	field, err := NewSLFieldsStore(tx).GetByUID(ctx, ref.FieldUID)
	if err != nil {
		if errors.Is(err, ErrSLFieldNotFound) {
			err = ErrOptionReference
		}
		return nil, nil, err
	}
	if field.SLTableID != table.ID || (field.Type != SingleSelectFieldType && field.Type != MultiSelectFieldType) {
		return nil, nil, ErrOptionReference
	}

	return table, field, nil
}

func referencedOptions(current, source []interface{}) []interface{} {
	result := make([]interface{}, 0, len(source))
	bySource, byName := map[string]string{}, map[string]string{}
	for _, raw := range current {
		o, _ := raw.(map[string]interface{})
		id, _ := o["uid"].(string)
		sid, _ := o["sourceUID"].(string)
		name, _ := o["name"].(string)
		if sid != "" {
			bySource[sid] = id
		} else {
			byName[name] = id
		}
	}

	used := map[string]bool{}
	for _, raw := range source {
		s, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}

		sid, _ := s["uid"].(string)
		name, _ := s["name"].(string)
		if sid == "" || name == "" {
			continue
		}

		id := bySource[sid]
		if id == "" {
			id = byName[name]
		}
		if id == "" || used[id] {
			id = "opt" + randstr.String(6)
		}

		used[id] = true
		result = append(result, map[string]interface{}{"uid": id, "name": name, "color": s["color"], "sourceUID": sid})
	}

	return result
}

// FilterReferencedOptions returns a copy with the options available for this record.
// Without conditions all source options are available, including options not yet used by any record.
func FilterReferencedOptions(ctx context.Context, tx *gorm.DB, field *SLField, fields []*SLField, data map[string]interface{}) (*SLField, error) {
	metadata, _ := field.Metadata.Data().(map[string]interface{})
	ref, err := ReadOptionReference(metadata)
	if err != nil || ref == nil {
		return field, err
	}

	owner, err := NewSLTablesStore(tx).GetByID(ctx, field.SLTableID)
	if err != nil {
		return nil, err
	}
	table, source, err := optionSource(ctx, tx, owner.ProjectID, ref)
	if err != nil {
		return nil, err
	}

	md := map[string]interface{}{}
	for k, v := range metadata {
		md[k] = v
	}

	allowed := map[string]bool{}
	if len(ref.Conditions) == 0 {
		sourceMD, _ := source.Metadata.Data().(map[string]interface{})
		for _, raw := range selectOptions(sourceMD) {
			o, _ := raw.(map[string]interface{})
			id, _ := o["uid"].(string)
			allowed[id] = true
		}
	} else {
		sourceFields, err := NewSLFieldsStore(tx).ListByTableID(ctx, table.ID)
		if err != nil {
			return nil, err
		}

		byUID := map[string]*SLField{}
		for _, f := range sourceFields {
			byUID[f.UID] = f
		}

		targets := map[string]*SLField{}
		for _, f := range fields {
			targets[f.UID] = f
		}

		q := tx.WithContext(ctx).Model(&SLRecord{}).Where("sl_table_id = ?", table.ID)
		predicates := make([]string, 0, len(ref.Conditions))
		filterArgs := []interface{}{}
		for _, c := range ref.Conditions {
			f := byUID[c.FieldUID]
			if f == nil {
				return nil, ErrOptionReference
			}

			value := c.Value
			op := c.Operation
			valid := true
			if c.ValueFieldUID != "" {
				t := targets[c.ValueFieldUID]
				if t == nil {
					return nil, ErrOptionReference
				}

				var ok bool
				if t.Type == MultiSelectFieldType {
					if op == FilterOperationEqual {
						op = FilterOperationIn
					} else if op == FilterOperationNotEqual {
						op = FilterOperationNotIn
					}
				}

				value, ok = conditionValue(f, t, data[c.ValueFieldUID], op)
				valid = ok
			}

			sql, args, err := buildSLRecordFilter(f, QuerySLRecordsFilter{FieldUID: f.UID, Operation: op, Value: value})
			if err != nil {
				return nil, ErrOptionReference
			}
			if !valid {
				sql, args = "FALSE", nil
			}
			predicates = append(predicates, "("+sql+")")
			filterArgs = append(filterArgs, args...)
		}

		// Group alternatives so OR never bypasses the source table or soft-delete filters.
		join := " AND "
		if ref.Match == "any" {
			join = " OR "
		}
		q = q.Where("("+strings.Join(predicates, join)+")", filterArgs...)

		var rows []struct{ Value datatypes.JSON }
		if err := q.Select("data -> ? AS value", source.UID).Scan(&rows).Error; err != nil {
			return nil, err
		}

		for _, r := range rows {
			var value interface{}
			if json.Unmarshal(r.Value, &value) != nil {
				continue
			}

			if s, ok := value.(string); ok {
				allowed[s] = true
			}
			if list, ok := value.([]interface{}); ok {
				for _, raw := range list {
					if s, ok := raw.(string); ok {
						allowed[s] = true
					}
				}
			}
		}
	}

	options := []interface{}{}
	for _, raw := range selectOptions(metadata) {
		o, _ := raw.(map[string]interface{})
		sid, _ := o["sourceUID"].(string)
		if allowed[sid] {
			options = append(options, raw)
		}
	}

	md["options"] = options
	copy := *field
	copy.Metadata = datatypes.NewJSONType[SLFieldMetadata](md)

	return &copy, nil
}

func conditionValue(source, target *SLField, raw interface{}, op FilterOperation) (string, bool) {
	values := []string{}
	add := func(raw interface{}) {
		if raw == nil {
			return
		}

		s := fmt.Sprint(raw)

		// Match option names because the source and target use different option IDs.
		if target.Type == SingleSelectFieldType || target.Type == MultiSelectFieldType {
			md, _ := target.Metadata.Data().(map[string]interface{})
			s = ""
			for _, o := range selectOptions(md) {
				o, _ := o.(map[string]interface{})
				if o["uid"] == raw {
					s, _ = o["name"].(string)
					break
				}
			}
		}

		if source.Type == SingleSelectFieldType || source.Type == MultiSelectFieldType {
			md, _ := source.Metadata.Data().(map[string]interface{})
			name := s
			s = ""
			for _, o := range selectOptions(md) {
				o, _ := o.(map[string]interface{})
				if o["name"] == name {
					s, _ = o["uid"].(string)
					break
				}
			}
		}

		if strings.TrimSpace(s) != "" {
			values = append(values, s)
		}
	}

	if target.Type == CheckboxFieldType && raw == nil {
		raw = false
	}
	if list, ok := raw.([]interface{}); ok {
		for _, v := range list {
			add(v)
		}
	} else {
		add(raw)
	}

	if len(values) == 0 {
		// Supply a syntactically valid unused value; the caller adds FALSE to the query.
		switch source.Type {
		case NumberFieldType:
			return "0", false
		case DateTimeFieldType:
			return "2026-01-01T00:00:00Z", false
		case CheckboxFieldType:
			return "false", false
		}
		if op == FilterOperationIn || op == FilterOperationNotIn {
			return `["__missing__"]`, false
		}
		return "__missing__", false
	}

	if op == FilterOperationIn || op == FilterOperationNotIn {
		b, _ := json.Marshal(values)
		return string(b), true
	}

	return values[0], true
}
