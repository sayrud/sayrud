// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"encoding/json"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/apibuilder"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
)

type viewHandlerOptions struct {
	vm          *goja.Runtime
	viewOptions apibuilder.ViewOptions
	response    json.RawMessage
}

func (publicHandler) viewHandler(ctx context.Context, opts viewHandlerOptions) (json.RawMessage, error) {
	vm := opts.vm
	dataset := opts.viewOptions.Datasets[0]
	fieldAlias := opts.viewOptions.FieldMapping

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

	result, err := db.SLTables.QueryFirst(ctx.Request().Context(), slTable, db.QueryFirstSLTableOptions{
		Fields: fields,
		Filter: filter,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// TODO
			return nil, nil
		}
		return nil, errors.Wrap(err, "query dataset")
	}

	if err := vm.Set("$data", result); err != nil {
		return nil, errors.Wrap(err, "set $data")
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
