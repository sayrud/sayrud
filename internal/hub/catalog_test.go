package hub

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestListPackages(t *testing.T) {
	var query url.Values
	var lang string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/open/v1/packages", r.URL.Path)
		query, lang = r.URL.Query(), r.Header.Get("Accept-Language")
		writeData(w, map[string]interface{}{
			"packages": []map[string]interface{}{{"kind": KindWorkflowPlugin, "id": "slack", "name": "Slack", "latestVersion": "1.2.0", "publisher": map[string]string{"userName": "sayrud"}}},
			"total":    21,
		})
	}))
	t.Cleanup(server.Close)
	t.Cleanup(UseForTest(server.URL))

	c, err := ListPackages(t.Context(), ListOptions{Kind: KindWorkflowPlugin, Keyword: "chat", Sort: SortPopular, Page: 2, PageSize: 500}, "zh-CN")
	require.NoError(t, err)
	require.Equal(t, int64(21), c.Total)
	require.Len(t, c.Packages, 1)
	require.Equal(t, "Slack", c.Packages[0].Name)
	require.Equal(t, "sayrud", c.Packages[0].Publisher.UserName)
	require.Equal(t, url.Values{"kind": {KindWorkflowPlugin}, "q": {"chat"}, "sort": {SortPopular}, "page": {"2"}, "pageSize": {"100"}}, query,
		"the empty options are omitted and the page size is capped")
	require.Equal(t, "zh-CN", lang)
}

func TestGetPackage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open/v1/packages/field-shortcut/uppercase":
			writeData(w, map[string]interface{}{
				"kind": KindFieldShortcut, "id": "uppercase", "name": "Uppercase", "readme": "# Uppercase",
				"latest": map[string]interface{}{"version": "1.0.0", "permissions": map[string]interface{}{"fetch": []string{"api.example.com"}, "ai": true}},
			})
		case "/open/v1/packages/field-shortcut/other":
			writeData(w, map[string]interface{}{"kind": KindFieldShortcut, "id": "uppercase"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(UseForTest(server.URL))

	p, err := GetPackage(t.Context(), KindFieldShortcut, "uppercase", "en")
	require.NoError(t, err)
	require.Equal(t, "# Uppercase", p.Readme)
	require.Equal(t, "1.0.0", p.Latest.Version)
	require.Equal(t, []string{"api.example.com"}, p.Latest.Permissions.Fetch)
	require.True(t, p.Latest.Permissions.AI)

	_, err = GetPackage(t.Context(), KindFieldShortcut, "other", "en")
	var hubErr *Error
	require.ErrorAs(t, err, &hubErr, "a response about another package is refused")

	_, err = GetPackage(t.Context(), KindFieldShortcut, "missing", "en")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCompatible(t *testing.T) {
	for _, tc := range []struct {
		app, minimum string
		want         bool
	}{
		{"1.4.0", "", true},
		{"1.4.0", "1.4.0", true},
		{"1.4.0", "1.3.9", true},
		{"1.4.0", "1.5.0", false},
		{"1.5.0-rc.1", "1.5.0", false},
		{"", "9.0.0", true},
		{"dev", "9.0.0", true},
	} {
		require.Equal(t, tc.want, Compatible(tc.app, tc.minimum), "%s with %s", tc.app, tc.minimum)
	}
}
