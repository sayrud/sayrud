// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sqlutil

import (
	"context"
	"strings"

	"github.com/auxten/postgresql-parser/pkg/sql/parser"
	"github.com/auxten/postgresql-parser/pkg/sql/sem/tree"
	"github.com/auxten/postgresql-parser/pkg/walk"
	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
)

var (
	ErrExpressionSyntaxError = errors.New("expression syntax error")
	ErrForbiddenExpression   = errors.New("forbidden expression")
)

var whiteFunctions = []string{
	"md5", "sha1", "sha256", "sha512",
	"concat", "substring", "substr", "length", "lower", "upper",
}

func SterilizeExpression(ctx context.Context, input string, allowFields map[string]string) (string, error) {
	exp := "SELECT " + input + ";"
	allowFieldNames := lo.Keys(allowFields)

	stmts, err := parser.Parse(exp)
	if err != nil {
		return "", ErrExpressionSyntaxError
	}

	inputFields := make([]string, 0)

	var walkErr error
	w := &walk.AstWalker{
		Fn: func(ctx interface{}, node interface{}) (stop bool) {
			switch v := node.(type) {
			case *tree.Update, *tree.Delete, *tree.Insert,
				*tree.Where, *tree.OrderBy, *tree.Order, *tree.JoinTableExpr, *tree.ParenTableExpr, *tree.ParenExpr:
				walkErr = ErrForbiddenExpression
				return true
			case *tree.From:
				if len(v.Tables) == 0 {
					return false
				}
				walkErr = ErrForbiddenExpression
				return true
			case *tree.BinaryExpr:
				inputFields = append(inputFields, v.String())

			case *tree.UnresolvedName:
				inputFields = append(inputFields, v.String())

				// HACK: We add separator to get the field name.
				v.Parts[0] = "!<----!" + allowFields[v.Parts[0]] + "!---->!"

			case *tree.FuncExpr:
				funcName := v.Func.String()
				if !lo.Contains(whiteFunctions, funcName) {
					walkErr = ErrForbiddenExpression
					return true
				}
			}

			return false
		},
	}

	ok, err := w.Walk(stmts, ctx)
	if err != nil {
		return "", ErrExpressionSyntaxError
	}
	if !ok {
		return "", ErrExpressionSyntaxError
	}

	inputFields = lo.Uniq(inputFields)
	unknownFields, _ := lo.Difference(inputFields, allowFieldNames)
	if len(unknownFields) > 0 {
		return "", ErrForbiddenExpression
	}
	if walkErr != nil {
		return "", walkErr
	}

	sql := stmts.String()
	sql = sql[7:]

	// Remove the separator.
	sql = strings.ReplaceAll(sql, `"!<----!`, "")
	sql = strings.ReplaceAll(sql, `!---->!"`, "")
	return sql, nil
}
