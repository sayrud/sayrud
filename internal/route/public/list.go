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

type listHandlerOptions struct {
	projectID   uint
	vm          *goja.Runtime
	listOptions apibuilder.ListOptions
}

func (publicHandler) listHandler(ctx context.Context, opts listHandlerOptions) (interface{}, error) {
	projectID := opts.projectID
	vm := opts.vm
	datasets := opts.listOptions.Datasets

	// Query datasets.
	datasetsResultSet := make(map[string][]map[string]interface{}, len(datasets))
	datasetsCountSet := make(map[string]int64, len(datasets))
	for _, dataset := range datasets {
		dataset := dataset

		filter, err := dataset.Filter.ToClauseExpression(vm)
		if err != nil {
			return nil, errors.Wrap(err, "parse filter expression")
		}

		result, count, err := db.SLTables.Query(ctx.Request().Context(), projectID, dataset.TableUID, db.QuerySLTableOptions{
			Fields: dataset.Fields,
			Filter: filter,
			Order:  dataset.Order,
			Limit:  0,
			Offset: 0,
		})
		if err != nil {
			return nil, errors.Wrap(err, "query dataset")
		}

		datasetsResultSet[dataset.TableUID] = result
		datasetsCountSet[dataset.TableUID] = count
	}

	// TODO
	return datasetsResultSet, nil
}
