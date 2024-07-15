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
	vm          *goja.Runtime
	listOptions apibuilder.ListOptions
}

func (publicHandler) listHandler(ctx context.Context, opts listHandlerOptions) (interface{}, error) {
	vm := opts.vm
	datasets := opts.listOptions.Datasets
	fieldAlias := opts.listOptions.FieldMapping

	// Query datasets.
	datasetsResultSet := make(map[string][]map[string]interface{}, len(datasets))
	datasetsCountSet := make(map[string]int64, len(datasets))
	for _, dataset := range datasets {
		dataset := dataset

		tableUID := dataset.TableUID
		slTable, err := db.SLTables.GetByUID(ctx.Request().Context(), tableUID)
		if err != nil {
			return nil, errors.Wrap(err, "get sl table by UID")
		}
		slFields, err := db.SLFields.GetByTableID(ctx.Request().Context(), slTable.ID)
		if err != nil {
			return nil, errors.Wrap(err, "get sl table by UID")
		}

		uidNameSets := slFields.UIDNameSets()
		if err := dataset.Filter.SetFieldUIDToName(uidNameSets); err != nil {
			return nil, errors.Wrap(err, "set field UID to name")
		}
		uidNameSets["_uid"] = "_uid"

		// Make a field sets and set field alias.
		fields := make(map[string]string) // Name -> Alia
		if len(dataset.Fields) == 0 || dataset.Fields[0] == "*" {
			// All fields
			for _, filed := range slFields {
				fields[filed.Name] = fieldAlias[filed.UID]
			}
		} else {
			// Specific fields
			for _, fieldUID := range dataset.Fields {
				fieldName := uidNameSets[fieldUID]
				fields[fieldName] = fieldAlias[fieldUID]
			}
		}

		filter, err := dataset.Filter.ToClauseExpression(vm)
		if err != nil {
			return nil, errors.Wrap(err, "parse filter expression")
		}

		order := make([]string, 0, len(dataset.Order))
		for _, orderFieldUID := range dataset.Order {
			fieldName, ok := uidNameSets[orderFieldUID]
			if ok {
				order = append(order, fieldName)
			}
		}

		result, count, err := db.SLTables.QueryList(ctx.Request().Context(), slTable, db.QueryListSLTableOptions{
			Fields: fields,
			Filter: filter,
			Order:  order,
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
	var result interface{}
	for _, v := range datasetsResultSet {
		result = v
		break
	}

	return result, nil
}
