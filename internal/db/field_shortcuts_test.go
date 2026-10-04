package db

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

func TestListAdminFieldShortcuts(t *testing.T) {
	ctx := context.Background()
	store := NewFieldShortcutsStore(newTestDB(t, &CustomFieldShortcut{}))
	shortcuts := []*CustomFieldShortcut{
		{Name: "Alpha shortcut", Description: "Ordinary description"},
		{Name: "Second shortcut", Description: "ALPHA in the description"},
		{Name: "Literal 100%", Description: `Path C:\shortcuts`},
		{Name: "Under_score"},
		{Name: "Other shortcut"},
	}
	for i := 0; i < 20; i++ {
		shortcuts = append(shortcuts, &CustomFieldShortcut{Name: fmt.Sprintf("Filler %d", i)})
	}
	for _, s := range shortcuts {
		s.ResultType = TextFieldType
		s.Code = "async function execute() { return ''; }"
		s.FormItems = datatypes.NewJSONType([]ShortcutFormItem{})
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
