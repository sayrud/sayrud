// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"encoding/json"
	"strings"

	"github.com/dop251/goja"
	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/apibuilder"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/routeutil"
)

const (
	RequestFieldTypeQuery = "query"
	RequestFieldTypeBody  = "body"
)

type createHandlerOptions struct {
	queryValues   map[string]interface{}
	bodyValues    map[string]interface{}
	createOptions apibuilder.CreateOptions
	tx            *gorm.DB
	vm            *goja.Runtime
	response      json.RawMessage
}

func (h publicHandler) createHandler(ctx context.Context, opts createHandlerOptions) (json.RawMessage, error) {
	tableUID := opts.createOptions.TableUID
	fieldMapping := opts.createOptions.FieldMapping
	fieldValues := make(map[string]interface{}, len(fieldMapping))
	vm := opts.vm

	for requestField, databaseFieldUID := range fieldMapping {
		// Split the `requestField` with : to get the real field name.
		groups := strings.SplitN(requestField, ":", 2)
		if len(groups) < 2 {
			continue
		}

		requestType := groups[0]
		field := groups[1]
		switch requestType {
		case RequestFieldTypeQuery:
			if value, ok := opts.queryValues[field]; ok {
				fieldValues[databaseFieldUID] = value
			}
		case RequestFieldTypeBody:
			if value, ok := opts.bodyValues[field]; ok {
				fieldValues[databaseFieldUID] = value
			}
		default:
			continue
		}
	}

	slTablesStore := db.NewSLTablesStore(opts.tx)
	slTable, err := slTablesStore.GetByUID(ctx.Request().Context(), tableUID)
	if err != nil {
		return nil, errors.Wrap(err, "get sl table by UID")
	}

	jsonBytes, err := routeutil.Validate(ctx, vm, slTable.ID, opts.tx, fieldValues)
	if err != nil {
		return nil, errors.Wrap(err, "validate")
	}

	slRecords := db.NewSLRecordsStore(opts.tx)
	newRecord, err := slRecords.Create(ctx.Request().Context(), slTable.ID, jsonBytes)
	if err != nil {
		return nil, errors.Wrap(err, "create sl records")
	}

	if err := vm.Set("$data", newRecord); err != nil {
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

type updateHandlerOptions struct {
	queryValues   map[string]interface{}
	bodyValues    map[string]interface{}
	updateOptions apibuilder.UpdateOptions
	tx            *gorm.DB
	vm            *goja.Runtime
	response      json.RawMessage
}

func (h publicHandler) updateHandler(ctx context.Context, opts updateHandlerOptions) (json.RawMessage, error) {
	tableUID := opts.updateOptions.TableUID
	fieldMapping := opts.updateOptions.FieldMapping
	fieldValues := make(map[string]interface{}, len(fieldMapping))
	vm := opts.vm

	for requestField, databaseFieldUID := range fieldMapping {
		// Split the `requestField` with : to get the real field name.
		groups := strings.SplitN(requestField, ":", 2)
		if len(groups) < 2 {
			continue
		}

		requestType := groups[0]
		field := groups[1]
		switch requestType {
		case RequestFieldTypeQuery:
			if value, ok := opts.queryValues[field]; ok {
				fieldValues[databaseFieldUID] = value
			}
		case RequestFieldTypeBody:
			if value, ok := opts.bodyValues[field]; ok {
				fieldValues[databaseFieldUID] = value
			}
		default:
			continue
		}
	}

	slTablesStore := db.NewSLTablesStore(opts.tx)
	slTable, err := slTablesStore.GetByUID(ctx.Request().Context(), tableUID)
	if err != nil {
		return nil, errors.Wrap(err, "get sl table by UID")
	}

	jsonBytes, err := routeutil.Validate(ctx, vm, slTable.ID, opts.tx, fieldValues)
	if err != nil {
		return nil, errors.Wrap(err, "validate")
	}

	filter, err := opts.updateOptions.Filter.ToClauseExpression(vm)
	if err != nil {
		return nil, errors.Wrap(err, "parse filter expression")
	}

	if err := slTablesStore.QueryUpdate(ctx.Request().Context(), slTable, db.QueryUpdateSLTableOptions{
		JsonBytes: jsonBytes,
		Filter:    filter,
	}); err != nil {
		return nil, errors.Wrap(err, "query update")
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
