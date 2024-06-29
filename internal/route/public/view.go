// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"github.com/dop251/goja"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/apibuilder"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
)

type viewHandlerOptions struct {
	projectID   uint
	vm          *goja.Runtime
	viewOptions apibuilder.ViewOptions
}

func (publicHandler) viewHandler(ctx context.Context, opts viewHandlerOptions) (interface{}, error) {
	projectID := opts.projectID
	vm := opts.vm
	dataset := opts.viewOptions.Datasets[0]

	// Query dataset.
	filter, err := dataset.Filter.ToClauseExpression(vm)
	if err != nil {
		return nil, errors.Wrap(err, "parse filter expression")
	}

	result, err := db.SLTables.QueryFirst(ctx.Request().Context(), projectID, dataset.TableUID, db.QueryFirstSLTableOptions{
		Fields: dataset.Fields,
		Filter: filter,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// TODO
			return nil, nil
		}
		return nil, errors.Wrap(err, "query dataset")
	}

	// TODO
	return result, nil
}
