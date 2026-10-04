package shortcut

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/samber/lo"

	"github.com/wuhan005/sayrud/internal/db"
)

const maxInputLength = 10000

var (
	fieldUIDPattern = regexp.MustCompile(`^fld[A-Za-z0-9]{7}$`)
	fieldRefPattern = regexp.MustCompile(`\{(fld[A-Za-z0-9]{7})\}`)
)

// Validate checks the shortcut of the field against the custom shortcut and the table fields, and returns the normalized shortcut,
// which only keeps the inputs of the form items. fields are the fields of the table, the field itself may or may not be in it.
// It returns *Error if invalid.
func Validate(custom *db.CustomFieldShortcut, fields []*db.SLField, field *db.SLField, shortcut *db.FieldShortcut) (*db.FieldShortcut, error) {
	if !field.Type.CanHostShortcut() || field.Type != custom.ResultType {
		return nil, newError("shortcut::unsupported_type")
	}

	byUID := lo.KeyBy(fields, func(f *db.SLField) string { return f.UID })
	normalized := &db.FieldShortcut{ID: custom.UID, Inputs: map[string]interface{}{}, AutoUpdate: shortcut.AutoUpdate}
	for _, item := range custom.FormItems.Data() {
		raw, _ := shortcut.Inputs[item.Key].(string)
		value := strings.TrimSpace(raw)
		if value == "" {
			if item.Required {
				return nil, newError("shortcut::input_required", item.Label)
			}
			continue
		}
		if utf8.RuneCountInString(value) > maxInputLength {
			return nil, newError("shortcut::input_too_long", item.Label)
		}

		switch item.Component {
		case db.ShortcutFormFieldSelect:
			ref, ok := byUID[value]
			if !ok || ref.UID == field.UID || !inputFieldAllowed(item, ref.Type) {
				return nil, newError("shortcut::invalid_input_field", item.Label)
			}
		case db.ShortcutFormSelect:
			if !lo.ContainsBy(item.Options, func(o db.ShortcutFormOption) bool { return o.Value == value }) {
				return nil, newError("shortcut::invalid_option", item.Label)
			}
		case db.ShortcutFormPrompt:
			for _, uid := range promptRefs(value) {
				ref, ok := byUID[uid]
				if !ok || ref.UID == field.UID || ref.Type == db.FormulaFieldType {
					return nil, newError("shortcut::invalid_input_field", item.Label)
				}
			}
		}
		normalized.Inputs[item.Key] = value
	}

	if hasCycle(fields, field.UID, normalized) {
		return nil, newError("shortcut::cycle")
	}
	return normalized, nil
}

// inputFieldAllowed reports whether the field type can be picked by the field_select item, formulas can not as they are computed by the browser.
func inputFieldAllowed(item db.ShortcutFormItem, t db.SLFieldType) bool {
	if t == db.FormulaFieldType || t.IsUnknown() {
		return false
	}
	return len(item.FieldTypes) == 0 || lo.Contains(item.FieldTypes, t)
}

func promptRefs(text string) []string {
	matches := fieldRefPattern.FindAllStringSubmatch(text, -1)
	return lo.Uniq(lo.Map(matches, func(m []string, _ int) string { return m[1] }))
}

// Dependencies returns the UIDs of the fields the shortcut reads: the inputs that are field UIDs and the fields referenced by `{fieldUID}`.
// It does not need the definition, so the dependencies of all the fields can be found without looking up the custom shortcuts.
func Dependencies(shortcut *db.FieldShortcut) []string {
	if shortcut == nil {
		return nil
	}
	var deps []string
	for _, v := range shortcut.Inputs {
		s, ok := v.(string)
		if !ok {
			continue
		}
		if fieldUIDPattern.MatchString(s) {
			deps = append(deps, s)
		}
		deps = append(deps, promptRefs(s)...)
	}
	return lo.Uniq(deps)
}

// hasCycle reports whether the field depends on itself through the shortcuts of the fields, with its shortcut replaced.
func hasCycle(fields []*db.SLField, fieldUID string, shortcut *db.FieldShortcut) bool {
	graph := make(map[string][]string, len(fields)+1)
	for _, f := range fields {
		if f.Shortcut != nil {
			graph[f.UID] = Dependencies(f.Shortcut)
		}
	}
	graph[fieldUID] = Dependencies(shortcut)

	visited := map[string]bool{}
	var visit func(uid string) bool
	visit = func(uid string) bool {
		if uid == fieldUID {
			return true
		}
		if visited[uid] {
			return false
		}
		visited[uid] = true
		for _, dep := range graph[uid] {
			if visit(dep) {
				return true
			}
		}
		return false
	}

	for _, dep := range graph[fieldUID] {
		if visit(dep) {
			return true
		}
	}
	return false
}
