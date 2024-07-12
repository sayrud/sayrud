// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sqlutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_SterilizeExpression(t *testing.T) {
	ctx := context.Background()

	allowFields := map[string]string{
		"aa": "UID->>AA",
		"bb": "UID->>BB",
		"cc": "UID->>CC",
	}

	for _, tc := range []struct {
		name           string
		input          string
		wantExpression string
		isError        bool
	}{
		{name: "select", input: "SELECT * FROM mytable", isError: true},
		{name: "insert", input: "INSERT INTO mytable VALUES (1, 2, 3)", isError: true},
		{name: "update", input: "UPDATE mytable SET a = 1", isError: true},
		{name: "delete", input: "DELETE FROM mytable WHERE a = 1", isError: true},
		{name: "from", input: "aa FROM mytable", isError: true},
		{name: "union", input: "aa UNION bb", isError: true},
		{name: "order", input: "aa ORDER BY aa ASC", isError: true},
		{name: "join", input: "aa FROM aa JOIN (SELECT bb) cc ON cc.bb = aa.aa", isError: true},
		{name: "unexpected fields", input: "md5(dd)", isError: true},
		{name: "unexpected json fields", input: "md5(bb->>'aaa')", isError: true},
		{name: "internal function", input: "user", isError: true},
		{name: "internal function generate_series", input: "generate_series(aa,bb,cc)", isError: true},

		{name: "ok", input: "md5(aa)", isError: false, wantExpression: `md5(UID->>AA)`},
		{name: "concat", input: "concat(md5(aa), 'Hello')", isError: false, wantExpression: `concat(md5(UID->>AA), 'Hello')`},
		{name: "comment", input: "md5(aa) ----comment", isError: false, wantExpression: `md5(UID->>AA)`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotExpression, gotError := SterilizeExpression(ctx, tc.input, allowFields)

			require.Equal(t, tc.isError, gotError != nil)
			require.Equal(t, tc.wantExpression, gotExpression)
		})
	}
}
