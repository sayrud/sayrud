package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/i18n"
)

func TestLegacyShortcutSections(t *testing.T) {
	sections, err := legacyShortcutSections(nil)
	require.NoError(t, err)
	require.Equal(t, []ShortcutSection{}, sections)
	sections, err = legacyShortcutSections([]byte(`[]`))
	require.NoError(t, err)
	require.Equal(t, []ShortcutSection{}, sections)

	sections, err = legacyShortcutSections([]byte(`[
		{"key":"source","label":{"en":"Source"},"component":"field_select","required":true,"fieldTypes":["text"]},
		{"key":"style","label":"Style","component":"input","placeholder":"Formal","default":"plain"},
		{"key":"notes","label":"Notes","component":"textarea"},
		{"key":"mode","label":"Mode","component":"select","options":[{"value":"a","label":"A"}],"default":"a"},
		{"key":"categories","label":"Categories","component":"field_options"},
		{"key":"prompt","label":"Prompt","component":"prompt"}
	]`))
	require.NoError(t, err)
	require.Equal(t, []ShortcutSection{{Fields: []ShortcutField{
		{Key: "source", Widget: ShortcutWidgetFieldSelect, Label: i18n.LocalText{"en": "Source"}, Required: true, FieldTypes: []SLFieldType{TextFieldType}},
		{Key: "style", Widget: ShortcutWidgetText, Label: i18n.LocalText{"": "Style"}, Placeholder: i18n.LocalText{"": "Formal"}, Default: "plain"},
		{Key: "notes", Widget: ShortcutWidgetText, Label: i18n.LocalText{"": "Notes"}, Multiline: true},
		{Key: "mode", Widget: ShortcutWidgetSelect, Label: i18n.LocalText{"": "Mode"}, Options: []ShortcutOption{{Value: "a", Label: i18n.LocalText{"": "A"}}}, Default: "a"},
		{Key: "categories", Widget: ShortcutWidgetFieldOptions, Label: i18n.LocalText{"": "Categories"}},
		{Key: "prompt", Widget: ShortcutWidgetPrompt, Label: i18n.LocalText{"": "Prompt"}},
	}}}, sections)

	_, err = legacyShortcutSections([]byte(`{}`))
	require.Error(t, err)
}

func TestMigrateFieldShortcuts(t *testing.T) {
	ctx := context.Background()
	gormDB := newTestDB(t)
	// The table as saved by v0.1.4.
	require.NoError(t, gormDB.Exec(`CREATE TABLE field_shortcuts (
		id bigserial PRIMARY KEY, created_at timestamptz, updated_at timestamptz, deleted_at timestamptz,
		uid varchar(16) NOT NULL, name varchar(64) NOT NULL, version varchar(32) NOT NULL DEFAULT '1.0.0',
		description text NOT NULL DEFAULT '', result_type varchar(32) NOT NULL, code text NOT NULL,
		language varchar(16) NOT NULL DEFAULT 'javascript', ai_enabled boolean NOT NULL DEFAULT false,
		form_items jsonb NOT NULL, domains jsonb NOT NULL, credentials jsonb NOT NULL,
		secrets text NOT NULL DEFAULT '', timeout_seconds bigint NOT NULL DEFAULT 30, enabled boolean NOT NULL DEFAULT false
	)`).Error)
	require.NoError(t, gormDB.Exec(`INSERT INTO field_shortcuts (uid, name, description, result_type, code, form_items, domains, credentials) VALUES
		('fscAAAAAAA', 'Translate', 'Translates the text', 'text', 'function execute() {}', '[{"key":"text","label":"Text","component":"textarea","required":true}]', '[]', '[]'),
		('fscBBBBBBB', 'Empty', '', 'number', 'function execute() {}', '[]', '[]', '[]')`).Error)

	require.NoError(t, gormDB.Transaction(func(tx *gorm.DB) error {
		if err := migrateFieldShortcuts(tx); err != nil {
			return err
		}
		return tx.AutoMigrate(&CustomFieldShortcut{})
	}))
	// Migrating again changes nothing.
	require.NoError(t, migrateFieldShortcuts(gormDB))
	require.False(t, gormDB.Migrator().HasColumn("field_shortcuts", "form_items"))

	store := NewFieldShortcutsStore(gormDB)
	got, err := store.GetByUID(ctx, "fscAAAAAAA")
	require.NoError(t, err)
	require.Equal(t, i18n.LocalText{"": "Translate"}, got.Name)
	require.Equal(t, i18n.LocalText{"": "Translates the text"}, got.Description)
	require.Equal(t, []ShortcutField{{Key: "text", Widget: ShortcutWidgetText, Label: i18n.LocalText{"": "Text"}, Required: true, Multiline: true}}, got.Fields())

	got, err = store.GetByUID(ctx, "fscBBBBBBB")
	require.NoError(t, err)
	require.Equal(t, "Empty", got.Name.In("en"))
	require.Empty(t, got.Description)
	require.Empty(t, got.Fields())

	list, total, err := store.ListAdmin(ctx, ListFieldShortcutsOptions{Keyword: "translates"})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "fscAAAAAAA", list[0].UID)
}
