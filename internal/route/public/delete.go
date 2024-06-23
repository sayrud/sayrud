// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"github.com/dop251/goja"
	"github.com/pkg/errors"

	"github.com/wuhan005/sayrud/internal/apibuilder"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
)

type deleteHandlerOptions struct {
	projectID     uint
	vm            *goja.Runtime
	deleteOptions apibuilder.DeleteOptions
}

func (publicHandler) deleteHandler(ctx context.Context, opts deleteHandlerOptions) (interface{}, error) {
	projectID := opts.projectID
	vm := opts.vm

	// Query dataset.
	filter, err := opts.deleteOptions.Filter.ToClauseExpression(vm)
	if err != nil {
		return nil, errors.Wrap(err, "parse filter expression")
	}

	affectRows, err := db.SLTables.QueryDelete(ctx.Request().Context(), projectID, opts.deleteOptions.TableUID, db.QueryDeleteSLTableOptions{
		Filter: filter,
	})
	if err != nil {
		return nil, errors.Wrap(err, "query delete")
	}

	// TODO
	return affectRows, nil
}
