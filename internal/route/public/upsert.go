// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
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
}

func (h publicHandler) createHandler(ctx context.Context, opts createHandlerOptions) error {
	tableUID := opts.createOptions.TableUID
	fieldMapping := opts.createOptions.FieldMapping
	fieldValues := make(map[string]interface{}, len(fieldMapping))

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
		return errors.Wrap(err, "get sl table by UID")
	}

	jsonBytes, err := routeutil.Validate(ctx, slTable.ID, opts.tx, fieldValues)
	if err != nil {
		return errors.Wrap(err, "validate")
	}

	slRecords := db.NewSLRecordsStore(opts.tx)
	if _, err := slRecords.Create(ctx.Request().Context(), slTable.ID, jsonBytes); err != nil {
		return errors.Wrap(err, "create sl records")
	}
	return nil
}

type updateHandlerOptions struct {
	vm            *goja.Runtime
	queryValues   map[string]interface{}
	bodyValues    map[string]interface{}
	updateOptions apibuilder.UpdateOptions
	tx            *gorm.DB
}

func (h publicHandler) updateHandler(ctx context.Context, opts updateHandlerOptions) error {
	tableUID := opts.updateOptions.TableUID
	fieldMapping := opts.updateOptions.FieldMapping
	fieldValues := make(map[string]interface{}, len(fieldMapping))

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
		return errors.Wrap(err, "get sl table by UID")
	}

	jsonBytes, err := routeutil.Validate(ctx, slTable.ID, opts.tx, fieldValues)
	if err != nil {
		return errors.Wrap(err, "validate")
	}

	filter, err := opts.updateOptions.Filter.ToClauseExpression(opts.vm)
	if err != nil {
		return errors.Wrap(err, "parse filter expression")
	}

	if err := slTablesStore.QueryUpdate(ctx.Request().Context(), slTable, db.QueryUpdateSLTableOptions{
		JsonBytes: jsonBytes,
		Filter:    filter,
	}); err != nil {
		return errors.Wrap(err, "query update")
	}

	return nil
}
