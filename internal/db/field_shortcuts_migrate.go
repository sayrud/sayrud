package db

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/i18n"
)

// migrateFieldShortcuts converts the shortcuts saved before their manifest was unified with the workflow plugins:
// the name and the description become translations, and the form items become the fields of a section.
func migrateFieldShortcuts(tx *gorm.DB) error {
	m := tx.Migrator()
	if !m.HasTable("field_shortcuts") {
		return nil
	}

	columnTypes, err := m.ColumnTypes("field_shortcuts")
	if err != nil {
		return errors.Wrap(err, "get column types")
	}
	for _, ct := range columnTypes {
		column := ct.Name()
		if (column != "name" && column != "description") || strings.EqualFold(ct.DatabaseTypeName(), "jsonb") {
			continue
		}
		if err := tx.Exec(fmt.Sprintf(`ALTER TABLE field_shortcuts ALTER COLUMN %[1]s DROP DEFAULT, ALTER COLUMN %[1]s TYPE jsonb USING CASE WHEN %[1]s = '' THEN '{}'::jsonb ELSE jsonb_build_object('', %[1]s) END`, column)).Error; err != nil {
			return errors.Wrapf(err, "convert field_shortcuts.%s", column)
		}
	}

	if !m.HasColumn("field_shortcuts", "form_items") {
		return nil
	}
	if err := tx.Exec(`ALTER TABLE field_shortcuts ADD COLUMN IF NOT EXISTS sections jsonb`).Error; err != nil {
		return errors.Wrap(err, "add field_shortcuts.sections")
	}
	var rows []struct {
		ID        int64
		FormItems []byte
	}
	if err := tx.Raw(`SELECT id, form_items FROM field_shortcuts`).Scan(&rows).Error; err != nil {
		return errors.Wrap(err, "read field_shortcuts.form_items")
	}
	for _, row := range rows {
		sections, err := legacyShortcutSections(row.FormItems)
		if err != nil {
			return errors.Wrapf(err, "convert the form items of field shortcut %d", row.ID)
		}
		raw, err := json.Marshal(sections)
		if err != nil {
			return errors.Wrap(err, "encode sections")
		}
		if err := tx.Exec(`UPDATE field_shortcuts SET sections = ? WHERE id = ?`, string(raw), row.ID).Error; err != nil {
			return errors.Wrapf(err, "update field shortcut %d", row.ID)
		}
	}
	if err := tx.Exec(`ALTER TABLE field_shortcuts ALTER COLUMN sections SET NOT NULL, DROP COLUMN form_items`).Error; err != nil {
		return errors.Wrap(err, "drop field_shortcuts.form_items")
	}
	return nil
}

// legacyShortcutFormItem is a form item saved before the fields of the shortcuts were named like those of the workflow plugins.
type legacyShortcutFormItem struct {
	Key         string           `json:"key"`
	Label       i18n.LocalText   `json:"label"`
	Component   string           `json:"component"`
	Required    bool             `json:"required"`
	Placeholder i18n.LocalText   `json:"placeholder"`
	FieldTypes  []SLFieldType    `json:"fieldTypes"`
	Options     []ShortcutOption `json:"options"`
	Default     string           `json:"default"`
}

// legacyShortcutSections puts the saved form items into an untitled section, input and textarea become text.
func legacyShortcutSections(raw []byte) ([]ShortcutSection, error) {
	var items []legacyShortcutFormItem
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
	}
	if len(items) == 0 {
		return []ShortcutSection{}, nil
	}
	fields := make([]ShortcutField, 0, len(items))
	for _, item := range items {
		field := ShortcutField{
			Key: item.Key, Widget: ShortcutWidget(item.Component), Label: item.Label, Placeholder: item.Placeholder,
			Required: item.Required, FieldTypes: item.FieldTypes, Options: item.Options, Default: item.Default,
		}
		switch item.Component {
		case "input":
			field.Widget = ShortcutWidgetText
		case "textarea":
			field.Widget, field.Multiline = ShortcutWidgetText, true
		}
		fields = append(fields, field)
	}
	return []ShortcutSection{{Fields: fields}}, nil
}
