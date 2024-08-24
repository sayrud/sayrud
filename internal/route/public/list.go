// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"encoding/json"

	"github.com/dop251/goja"
	"github.com/pkg/errors"

	"github.com/wuhan005/sayrud/internal/apibuilder"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
)

type listHandlerOptions struct {
	vm          *goja.Runtime
	response    json.RawMessage
	listOptions apibuilder.ListOptions
}

func (publicHandler) listHandler(ctx context.Context, opts listHandlerOptions) (json.RawMessage, error) {
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

		orderSets := make(map[string]string, len(dataset.Orders))
		for _, order := range dataset.Orders {
			fieldName, ok := uidNameSets[order.FieldUID]
			if ok {
				orderSets[fieldName] = order.OrderType
			}
		}

		// TODO: Pagination
		result, count, err := db.SLTables.QueryList(ctx.Request().Context(), slTable, db.QueryListSLTableOptions{
			Fields: fields,
			Filter: filter,
			Orders: orderSets,
			Limit:  0,
			Offset: 0,
		})
		if err != nil {
			return nil, errors.Wrap(err, "query dataset")
		}

		datasetsResultSet[slTable.Name] = result
		datasetsCountSet[slTable.Name] = count
	}

	if err := vm.Set("$data", datasetsResultSet); err != nil {
		return nil, errors.Wrap(err, "set $data")
	}
	if err := vm.Set("$count", datasetsCountSet); err != nil {
		return nil, errors.Wrap(err, "set $count")
	}

	var responseTemplate interface{}
	if err := json.Unmarshal(opts.response, &responseTemplate); err != nil {
		return nil, errors.Wrap(err, "unmarshal response")
	}
	response, err := setResponseData(vm, responseTemplate)
	if err != nil {
		return nil, errors.Wrap(err, "make response")
	}
	return json.Marshal(response)
}
