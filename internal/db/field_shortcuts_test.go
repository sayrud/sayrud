package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/i18n"
)

func TestListAdminFieldShortcuts(t *testing.T) {
	ctx := context.Background()
	store := NewFieldShortcutsStore(newTestDB(t, &CustomFieldShortcut{}))
	shortcuts := []*CustomFieldShortcut{
		{Name: i18n.LocalText{"": "Alpha shortcut"}, Description: i18n.LocalText{"": "Ordinary description"}},
		{Name: i18n.LocalText{"": "Second shortcut"}, Description: i18n.LocalText{"en": "ALPHA in the description", "zh-CN": "翻译的描述"}},
		{Name: i18n.LocalText{"": "Literal 100%"}, Description: i18n.LocalText{"": `Path C:\shortcuts`}},
		{Name: i18n.LocalText{"": "Under_score"}},
		{Name: i18n.LocalText{"": "Other shortcut"}},
	}
	for i := 0; i < 20; i++ {
		shortcuts = append(shortcuts, &CustomFieldShortcut{Name: i18n.LocalText{"": fmt.Sprintf("Filler %d", i)}})
	}
	for _, s := range shortcuts {
		s.ResultType = TextFieldType
		s.Code = "async function execute() { return ''; }"
		s.Sections = datatypes.NewJSONType([]ShortcutSection{})
		s.Domains = datatypes.NewJSONType([]string{})
		s.Credentials = datatypes.NewJSONType([]ShortcutCredential{})
		require.NoError(t, store.Create(ctx, s))
	}

	page, total, err := store.ListAdmin(ctx, ListFieldShortcutsOptions{})
	require.NoError(t, err)
	require.Equal(t, int64(len(shortcuts)), total)
	require.Len(t, page, dbutil.DefaultPageSize)
	require.Equal(t, shortcuts[0].UID, page[0].UID)

	for _, tc := range []struct {
		name    string
		keyword string
		page    int
		total   int64
		indices []int
	}{
		{name: "first page", page: 1, total: 25, indices: []int{0, 1}},
		{name: "second page", page: 2, total: 25, indices: []int{2, 3}},
		{name: "name and description", keyword: " aLpHa ", page: 1, total: 2, indices: []int{0, 1}},
		{name: "search before pagination", keyword: "shortcut", page: 2, total: 4, indices: []int{2, 4}},
		{name: "literal percent", keyword: "%", page: 1, total: 1, indices: []int{2}},
		{name: "literal underscore", keyword: "_", page: 1, total: 1, indices: []int{3}},
		{name: "literal backslash", keyword: `\`, page: 1, total: 1, indices: []int{2}},
		{name: "translation", keyword: "翻译", page: 1, total: 1, indices: []int{1}},
		{name: "language keys are not searched", keyword: "zh", page: 1},
		{name: "no match", keyword: "missing", page: 1},
		{name: "past the final page", page: 14, total: 25},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page, total, err := store.ListAdmin(ctx, ListFieldShortcutsOptions{
				Pagination: dbutil.Pagination{Page: tc.page, PageSize: 2},
				Keyword:    tc.keyword,
			})
			require.NoError(t, err)
			require.Equal(t, tc.total, total)
			require.Len(t, page, len(tc.indices))
			for i, index := range tc.indices {
				require.Equal(t, shortcuts[index].UID, page[i].UID)
			}
		})
	}

	all, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, all, len(shortcuts), "the member shortcut catalog still needs all shortcuts")
}

func TestFieldShortcutLanguage(t *testing.T) {
	ctx := context.Background()
	store := NewFieldShortcutsStore(newTestDB(t, &CustomFieldShortcut{}))
	s := &CustomFieldShortcut{
		Name: i18n.LocalText{"": "Legacy"}, ResultType: TextFieldType, Code: "function execute() { return 1 }",
		Sections: datatypes.NewJSONType([]ShortcutSection{}), Domains: datatypes.NewJSONType([]string{}), Credentials: datatypes.NewJSONType([]ShortcutCredential{}),
	}
	require.NoError(t, store.Create(ctx, s))
	got, err := store.GetByUID(ctx, s.UID)
	require.NoError(t, err)
	require.Equal(t, "javascript", got.Language)

	got.Language = "typescript"
	got.Code = "export function execute(): number { return 1 }"
	require.NoError(t, store.Update(ctx, got))
	got, err = store.GetByUID(ctx, s.UID)
	require.NoError(t, err)
	require.Equal(t, "typescript", got.Language)
	require.Equal(t, "export function execute(): number { return 1 }", got.Code)
}

func TestFieldShortcutVersion(t *testing.T) {
	ctx := context.Background()
	store := NewFieldShortcutsStore(newTestDB(t, &CustomFieldShortcut{}))
	s := &CustomFieldShortcut{
		ManifestID: "version_test", Homepage: "https://example.com/plugin", Name: i18n.LocalText{"": "Legacy"}, ResultType: TextFieldType, Code: "function execute() { return 'text' }",
		Sections: datatypes.NewJSONType([]ShortcutSection{}), Domains: datatypes.NewJSONType([]string{}), Credentials: datatypes.NewJSONType([]ShortcutCredential{}),
	}
	require.NoError(t, store.Create(ctx, s))
	got, err := store.GetByUID(ctx, s.UID)
	require.NoError(t, err)
	require.Equal(t, "1.0.0", got.Version)
	require.Equal(t, "version_test", got.ManifestID)
	require.Equal(t, "https://example.com/plugin", got.Homepage)

	got.Version = "2.3.0"
	got.Homepage = "https://example.com/support"
	require.NoError(t, store.Update(ctx, got))
	list, total, err := store.ListAdmin(ctx, ListFieldShortcutsOptions{})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "2.3.0", list[0].Version)
	require.Equal(t, "version_test", list[0].ManifestID)
	require.Equal(t, "https://example.com/support", list[0].Homepage)
	require.Equal(t, s.UID, list[0].UID)
	got.Homepage = ""
	require.NoError(t, store.Update(ctx, got))
	got, err = store.GetByUID(ctx, s.UID)
	require.NoError(t, err)
	require.Empty(t, got.Homepage)
	require.Equal(t, "version_test", got.ManifestID)
}
