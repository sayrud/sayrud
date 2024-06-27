// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"strings"

	"github.com/dop251/goja"
	"github.com/pkg/errors"

	"github.com/wuhan005/sayrud/internal/apibuilder"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
)

const (
	RequestFieldTypeQuery = "query"
	RequestFieldTypeBody  = "body"
)

type createHandlerOptions struct {
	projectID     uint
	queryValues   map[string]interface{}
	bodyValues    map[string]interface{}
	createOptions apibuilder.CreateOptions
}

func (publicHandler) createHandler(ctx context.Context, opts createHandlerOptions) error {
	projectID := opts.projectID
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

	if err := db.SLTables.QueryInsert(ctx.Request().Context(), projectID, opts.createOptions.TableUID, db.QueryInsertSLTableOptions{
		FieldValues: fieldValues,
	}); err != nil {
		return errors.Wrap(err, "query insert")
	}

	return nil
}

type updateHandlerOptions struct {
	projectID     uint
	vm            *goja.Runtime
	queryValues   map[string]interface{}
	bodyValues    map[string]interface{}
	updateOptions apibuilder.UpdateOptions
}

func (publicHandler) updateHandler(ctx context.Context, opts updateHandlerOptions) error {
	projectID := opts.projectID
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

	filter, err := opts.updateOptions.Filter.ToClauseExpression(opts.vm)
	if err != nil {
		return errors.Wrap(err, "parse filter expression")
	}

	if err := db.SLTables.QueryUpdate(ctx.Request().Context(), projectID, opts.updateOptions.TableUID, db.QueryUpdateSLTableOptions{
		FieldValues: fieldValues,
		Filter:      filter,
	}); err != nil {
		return errors.Wrap(err, "query update")
	}

	return nil
}
