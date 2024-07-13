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
	vm            *goja.Runtime
	deleteOptions apibuilder.DeleteOptions
}

func (publicHandler) deleteHandler(ctx context.Context, opts deleteHandlerOptions) (interface{}, error) {
	vm := opts.vm
	tableUID := opts.deleteOptions.TableUID

	slTable, err := db.SLTables.GetByUID(ctx.Request().Context(), tableUID)
	if err != nil {
		return nil, errors.Wrap(err, "get sl table by UID")
	}
	slFields, err := db.SLFields.GetByTableID(ctx.Request().Context(), slTable.ID)
	if err != nil {
		return nil, errors.Wrap(err, "get sl table by UID")
	}

	uidNameSets := slFields.UIDNameSets()
	if err := opts.deleteOptions.Filter.SetFieldUIDToName(uidNameSets); err != nil {
		return nil, errors.Wrap(err, "set field UID to name")
	}

	filter, err := opts.deleteOptions.Filter.ToClauseExpression(vm)
	if err != nil {
		return nil, errors.Wrap(err, "parse filter expression")
	}

	affectRows, err := db.SLTables.QueryDelete(ctx.Request().Context(), slTable, db.QueryDeleteSLTableOptions{
		Filter: filter,
	})
	if err != nil {
		return nil, errors.Wrap(err, "query delete")
	}

	// TODO
	return affectRows, nil
}
