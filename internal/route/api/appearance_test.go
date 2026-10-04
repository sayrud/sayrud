package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/form"
)

func TestValidAppearanceUpdate(t *testing.T) {
	for _, tc := range []struct {
		body string
		want bool
	}{
		{`{}`, true},
		{`{"name":"Renamed"}`, true},
		{`{"icon":"calendar","color":"purple"}`, true},
		{`{"icon":"chart-no-axes-combined"}`, true},
		{`{"icon":"","color":""}`, true},
		{`{"icon":null,"color":null}`, true},
		{`{"name":""}`, false},
		{`{"name":"  "}`, false},
		{`{"icon":"Calendar"}`, false},
		{`{"icon":"calendar--days"}`, false},
		{`{"icon":"<script>"}`, false},
		{`{"icon":"` + strings.Repeat("a", 65) + `"}`, false},
		{`{"color":"#ac8cff"}`, false},
		{`{"color":"Purple"}`, false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			var project form.UpdateProject
			require.NoError(t, json.Unmarshal([]byte(tc.body), &project))
			require.Equal(t, tc.want, validAppearanceUpdate(project.Name, project.Icon, project.Color))

			var table form.UpdateTable
			require.NoError(t, json.Unmarshal([]byte(tc.body), &table))
			require.Equal(t, tc.want, validAppearanceUpdate(table.Name, table.Icon, table.Color))
		})
	}

	for _, color := range []string{"orange", "coral", "pink", "purple", "indigo", "blue", "teal", "green"} {
		require.True(t, validAppearanceUpdate(nil, nil, &color), color)
	}
}
