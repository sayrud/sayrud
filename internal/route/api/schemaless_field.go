// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/form"
)

func (schemalessRoute) Fielder(ctx context.Context) error {
	fieldUID := ctx.Param("fieldUID")
	slField, err := db.SLFields.GetByUID(ctx.Request().Context(), fieldUID)
	if err != nil {
		if errors.Is(err, db.ErrSLFieldNotFound) {
			return ctx.ApiError(http.StatusNotFound, "数据表字段不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get sl field by ID")
		return ctx.ApiServerError()
	}

	ctx.Map(slField)
	return nil
}

func (schemalessRoute) ListFields(ctx context.Context, table *db.SLTable) error {
	tableID := table.ID
	slFields, err := db.SLFields.List(ctx.Request().Context(), tableID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list sl fields")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(slFields)
}

func (schemalessRoute) CreateFields(ctx context.Context, table *db.SLTable, tx dbutil.Transactor, f form.CreateFields) error {
	tableID := table.ID

	if err := tx.Transaction(func(tx *gorm.DB) error {
		slFieldsStore := db.NewSLFieldsStore(tx)

		currentFields, err := slFieldsStore.List(ctx.Request().Context(), tableID)
		if err != nil {
			return errors.Wrap(err, "count")
		}
		var lastPosition int
		if len(currentFields) > 0 {
			lastPosition = currentFields[len(currentFields)-1].Position + 1
		}

		for _, field := range f.Fields {
			field := field

			if _, err := slFieldsStore.Create(ctx.Request().Context(), db.CreateSLFieldOptions{
				SLTableID: tableID,
				Name:      field.Name,
				Label:     field.Label,
				Type:      db.SLFieldType(field.Type),
				Options:   field.Options,
				Position:  lastPosition,
			}); err != nil {
				return errors.Wrap(err, "create")
			}

			lastPosition++
		}

		// Refresh table view.
		slTablesStore := db.NewSLTablesStore(tx)
		return slTablesStore.CreateView(ctx.Request().Context(), table)

	}); err != nil {
		if errors.Is(err, db.ErrSLFieldExists) {
			return ctx.ApiError(http.StatusConflict, "字段已存在")
		}
		if errors.Is(err, db.ErrUnexpectedType) {
			return ctx.ApiError(http.StatusBadRequest, "字段类型错误")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create sl field")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess("创建字段成功")
}

func (schemalessRoute) UpdateFields(ctx context.Context, table *db.SLTable, tx dbutil.Transactor, f form.UpdateFields) error {
	if err := tx.Transaction(func(tx *gorm.DB) error {
		slFieldsStore := db.NewSLFieldsStore(tx)

		for _, formField := range f.Fields {
			fieldUID := formField.UID
			// Make sure the field belongs to the table.
			field, err := slFieldsStore.GetByUID(ctx.Request().Context(), fieldUID)
			if err != nil {
				return errors.Wrap(err, "get field")
			}
			if field.SLTableID != table.ID {
				return db.ErrSLFieldNotFound
			}

			if err := slFieldsStore.Update(ctx.Request().Context(), field.ID, db.UpdateSLFieldOptions{
				Name:     formField.Name,
				Label:    formField.Label,
				Type:     db.SLFieldType(formField.Type),
				Options:  formField.Options,
				Position: formField.Position,
			}); err != nil {
				return errors.Wrap(err, "update")
			}
		}

		// Refresh table view.
		slTablesStore := db.NewSLTablesStore(tx)
		return slTablesStore.CreateView(ctx.Request().Context(), table)
	}); err != nil {
		if errors.Is(err, db.ErrSLFieldNotFound) {
			return ctx.ApiError(http.StatusNotFound, "数据表字段不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update sl fields")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess("更新字段信息成功")
}

func (schemalessRoute) UpdateField(ctx context.Context, field *db.SLField, tx dbutil.Transactor, f form.UpdateField) error {
	fieldID := field.ID

	if err := tx.Transaction(func(tx *gorm.DB) error {
		slFieldsStore := db.NewSLFieldsStore(tx)

		if err := slFieldsStore.Update(ctx.Request().Context(), fieldID, db.UpdateSLFieldOptions{
			Name:     f.Name,
			Label:    f.Label,
			Type:     db.SLFieldType(f.Type),
			Options:  f.Options,
			Position: f.Position,
		}); err != nil {
			return errors.Wrap(err, "update")
		}

		// Refresh table view.
		slTablesStore := db.NewSLTablesStore(tx)
		return slTablesStore.CreateView(ctx.Request().Context(), &field.SLTable)

	}); err != nil {
		if errors.Is(err, db.ErrSLFieldNotFound) {
			return ctx.ApiError(http.StatusNotFound, "数据表字段不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update sl field")
		return ctx.ApiServerError()
	}

	return ctx.ApiSuccess("更新字段信息成功")
}

func (schemalessRoute) DeleteField(ctx context.Context, field *db.SLField, tx dbutil.Transactor) error {
	fieldID := field.ID

	if err := tx.Transaction(func(tx *gorm.DB) error {
		slFieldsStore := db.NewSLFieldsStore(tx)
		if err := slFieldsStore.DeleteByID(ctx.Request().Context(), fieldID); err != nil {
			return errors.Wrap(err, "delete")
		}

		// Refresh table view.
		slTablesStore := db.NewSLTablesStore(tx)
		return slTablesStore.CreateView(ctx.Request().Context(), &field.SLTable)

	}); err != nil {
		if errors.Is(err, db.ErrSLFieldNotFound) {
			return ctx.ApiError(http.StatusNotFound, "数据表字段不存在")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete sl field")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess("删除字段成功")
}

func (schemalessRoute) FieldTypes(ctx context.Context) error {
	// FIXME: sort field types.
	fieldTypes := make(map[db.SLFieldType]string)
	for _, field := range db.FieldTypes {
		label := field.Label()
		fieldTypes[field] = label
	}
	return ctx.ApiSuccess(fieldTypes)
}
