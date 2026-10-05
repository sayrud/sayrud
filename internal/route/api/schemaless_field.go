// Copyright 2023 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
)

// Fielder maps the field of the fieldUID path parameter as *db.SLField, it must belong to the current table.
func (schemalessRoute) Fielder(ctx context.Context, table *db.SLTable) error {
	fieldUID := ctx.Param("fieldUID")
	slField, err := db.SLFields.GetByUID(ctx.Request().Context(), fieldUID)
	if err != nil {
		if errors.Is(err, db.ErrSLFieldNotFound) {
			return ctx.ApiError(http.StatusNotFound, "field::not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get sl field by UID")
		return ctx.ApiServerError()
	}

	if slField.SLTableID != table.ID {
		return ctx.ApiError(http.StatusNotFound, "field::not_found")
	}

	ctx.Map(slField)
	return nil
}

// ListFields
// @Summary List fields
// @Description List all the fields of the table, ordered by position.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Success 200 {array} dto.Field
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 500 {string} string "Internal server error"
// @ID listFields
// @Router /projects/{projectUID}/tables/{tableUID}/fields [get]
func (schemalessRoute) ListFields(ctx context.Context, table *db.SLTable) error {
	slFields, err := db.SLFields.ListByTableID(ctx.Request().Context(), table.ID)
	if err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to list sl fields")
		return ctx.ApiServerError()
	}
	fields := dto.ToFields(table, slFields)
	prepareSharedFields(fields, sharedToken(ctx))
	return ctx.ApiSuccess(fields)
}

const ReserveUIDFieldName = "_uid"

var (
	ErrReserveUIDField  = errors.New("reserve uid field name")
	ErrEmptyFieldLabel  = errors.New("empty field label")
	ErrFieldLabelExists = errors.New("field label exists")
)

// checkFieldLabel checks the label is not empty, reserved, or used by other fields.
func checkFieldLabel(label string, otherLabels []string) error {
	switch {
	case label == "":
		return ErrEmptyFieldLabel
	case label == ReserveUIDFieldName:
		return ErrReserveUIDField
	case lo.Contains(otherLabels, label):
		return ErrFieldLabelExists
	default:
		return nil
	}
}

// fieldErrorResponse returns the response of the field validation errors, ok is false if the error is unexpected.
func fieldErrorResponse(err error) (statusCode int, msg string, ok bool) {
	switch {
	case errors.Is(err, db.ErrOptionReference):
		return http.StatusBadRequest, "field::invalid_options_reference", true
	case errors.Is(err, ErrEmptyFieldLabel):
		return http.StatusBadRequest, "field::title_required", true
	case errors.Is(err, ErrReserveUIDField):
		return http.StatusBadRequest, "field::uid_reserved", true
	case errors.Is(err, ErrFieldLabelExists), errors.Is(err, db.ErrSLFieldExists):
		return http.StatusConflict, "field::exists", true
	case errors.Is(err, db.ErrUnexpectedType):
		return http.StatusBadRequest, "field::invalid_type", true
	default:
		return 0, "", false
	}
}

// ResolveOptions
// @Summary Resolve referenced select options for a record or draft field
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param data body form.ResolveFieldOptions true "Field configuration and record values"
// @Success 200 {array} map[string]interface{}
// @Failure 400 {string} string "Invalid option reference"
// @Failure 403 {string} string "Permission denied"
// @ID resolveFieldOptions
// @Router /projects/{projectUID}/tables/{tableUID}/fields/options/resolve [post]
func (schemalessRoute) ResolveOptions(ctx context.Context, table *db.SLTable, tx dbutil.Transactor, f form.ResolveFieldOptions) error {
	var options interface{}
	err := tx.Transaction(func(tx *gorm.DB) error {
		stored, err := db.NewSLFieldsStore(tx).GetByUID(ctx.Request().Context(), f.FieldUID)
		if err != nil && !errors.Is(err, db.ErrSLFieldNotFound) {
			return err
		}

		// Keep persisted option IDs while source metadata synchronization is pending.
		if stored == nil || stored.SLTableID != table.ID || stored.Type != db.SLFieldType(f.Type) || !reflect.DeepEqual(stored.Metadata.Data(), f.Metadata) {
			if err := db.PrepareSelectMetadata(ctx.Request().Context(), tx, table.ID, f.FieldUID, db.SLFieldType(f.Type), f.Metadata); err != nil {
				return err
			}
		}

		fields, err := db.NewSLFieldsStore(tx).ListByTableID(ctx.Request().Context(), table.ID)
		if err != nil {
			return err
		}

		field := &db.SLField{UID: f.FieldUID, SLTableID: table.ID, Type: db.SLFieldType(f.Type), Metadata: datatypes.NewJSONType[db.SLFieldMetadata](f.Metadata)}
		filtered, err := db.FilterReferencedOptions(ctx.Request().Context(), tx, field, fields, f.Data)
		if err != nil {
			return err
		}

		md, _ := filtered.Metadata.Data().(map[string]interface{})
		options = md["options"]
		return nil
	})

	if err != nil {
		if code, msg, ok := fieldErrorResponse(err); ok {
			return ctx.ApiError(code, msg)
		}
		return ctx.ApiServerError()
	}

	if options == nil {
		options = []interface{}{}
	}

	return ctx.ApiSuccess(options)
}

// CreateFields
// @Summary Create fields
// @Description Create fields in batch, which are appended to the end of the table.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param data body form.CreateFields true "Fields to create"
// @Success 200 {array} dto.Field
// @Failure 400 {string} string "Invalid label or type"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or table not found"
// @Failure 409 {string} string "Field label already exists"
// @Failure 500 {string} string "Internal server error"
// @ID createFields
// @Router /projects/{projectUID}/tables/{tableUID}/fields [post]
func (schemalessRoute) CreateFields(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable, tx dbutil.Transactor, f form.CreateFields) error {
	var createdFields []*db.SLField
	if err := tx.Transaction(func(tx *gorm.DB) error {
		slFieldsStore := db.NewSLFieldsStore(tx)

		currentFields, err := slFieldsStore.ListByTableID(ctx.Request().Context(), table.ID)
		if err != nil {
			return errors.Wrap(err, "list fields")
		}
		labels := lo.Map(currentFields, func(field *db.SLField, _ int) string { return field.Label })

		var position int
		if len(currentFields) > 0 {
			position = currentFields[len(currentFields)-1].Position + 1
		}

		for _, field := range f.Fields {
			label := strings.TrimSpace(field.Label)
			if err := checkFieldLabel(label, labels); err != nil {
				return err
			}

			metadata := field.Metadata
			if metadata == nil {
				metadata = map[string]interface{}{}
			}

			slField, err := slFieldsStore.Create(ctx.Request().Context(), db.CreateSLFieldOptions{
				SLTableID: table.ID,
				Label:     label,
				Type:      db.SLFieldType(field.Type),
				Metadata:  metadata,
				Position:  position,
			})
			if err != nil {
				return errors.Wrap(err, "create")
			}

			createdFields = append(createdFields, slField)
			labels = append(labels, label)
			position++
		}
		return nil
	}); err != nil {
		if statusCode, msg, ok := fieldErrorResponse(err); ok {
			return ctx.ApiError(statusCode, msg)
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to create sl fields")
		return ctx.ApiServerError()
	}
	notifyDirty(ctx, hub, project, table, collab.DirtyScope{Fields: true})
	return ctx.ApiSuccess(dto.ToFields(table, createdFields))
}

// UpdateField
// @Summary Update a field
// @Description Update the label, type or metadata of the field. Existing record values are kept as is when the type changes.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param fieldUID path string true "Field UID"
// @Param data body form.UpdateField true "Field properties"
// @Success 200 {object} dto.Field
// @Failure 400 {string} string "Invalid label or type"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table or field not found"
// @Failure 409 {string} string "Field label already exists"
// @Failure 500 {string} string "Internal server error"
// @ID updateField
// @Router /projects/{projectUID}/tables/{tableUID}/fields/{fieldUID} [put]
func (schemalessRoute) UpdateField(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable, field *db.SLField, tx dbutil.Transactor, f form.UpdateField) error {
	var updatedField *db.SLField
	if err := tx.Transaction(func(tx *gorm.DB) error {
		slFieldsStore := db.NewSLFieldsStore(tx)

		if f.Label != nil {
			currentFields, err := slFieldsStore.ListByTableID(ctx.Request().Context(), table.ID)
			if err != nil {
				return errors.Wrap(err, "list fields")
			}
			otherLabels := lo.FilterMap(currentFields, func(item *db.SLField, _ int) (string, bool) {
				return item.Label, item.ID != field.ID
			})

			label := strings.TrimSpace(*f.Label)
			if err := checkFieldLabel(label, otherLabels); err != nil {
				return err
			}
			if err := slFieldsStore.SetLabel(ctx.Request().Context(), field.ID, label); err != nil {
				return errors.Wrap(err, "set label")
			}
		}

		if f.Type != nil && db.SLFieldType(*f.Type) != field.Type {
			fieldType := db.SLFieldType(*f.Type)
			if !fieldType.Check() {
				return db.ErrUnexpectedType
			}
			metadata := f.Metadata
			if metadata == nil {
				metadata = map[string]interface{}{}
			}
			if err := slFieldsStore.SetType(ctx.Request().Context(), field.ID, fieldType, metadata); err != nil {
				return errors.Wrap(err, "set type")
			}
		} else if f.Metadata != nil {
			if err := slFieldsStore.SetMetadata(ctx.Request().Context(), field.ID, f.Metadata); err != nil {
				return errors.Wrap(err, "set metadata")
			}
		}

		var err error
		updatedField, err = slFieldsStore.GetByID(ctx.Request().Context(), field.ID)
		if err != nil {
			return errors.Wrap(err, "get field")
		}
		return nil
	}); err != nil {
		if statusCode, msg, ok := fieldErrorResponse(err); ok {
			return ctx.ApiError(statusCode, msg)
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to update sl field")
		return ctx.ApiServerError()
	}
	notifyDirty(ctx, hub, project, table, collab.DirtyScope{Fields: true, AllRecords: f.Type != nil || f.Metadata != nil})
	if f.Type != nil || f.Metadata != nil {
		hub.NotifyChange(ctx.Request().Context(), project, table, &collab.Change{Fields: []string{field.UID}})
	}

	return ctx.ApiSuccess(dto.ToField(table, updatedField))
}

// UpdateFieldPosition
// @Summary Move a field
// @Description Move the field to the zero-based index among the table fields.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param fieldUID path string true "Field UID"
// @Param data body form.UpdateFieldPosition true "Field position"
// @Success 204 "No Content"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table or field not found"
// @Failure 500 {string} string "Internal server error"
// @ID updateFieldPosition
// @Router /projects/{projectUID}/tables/{tableUID}/fields/{fieldUID}/position [put]
func (schemalessRoute) UpdateFieldPosition(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable, field *db.SLField, f form.UpdateFieldPosition) error {
	if err := db.SLFields.Move(ctx.Request().Context(), table.ID, field.ID, int(f.Position)); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to move sl field")
		return ctx.ApiServerError()
	}
	notifyDirty(ctx, hub, project, table, collab.DirtyScope{Fields: true})
	return ctx.Status(http.StatusNoContent)
}

// DeleteField
// @Summary Delete a field
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param fieldUID path string true "Field UID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table or field not found"
// @Failure 500 {string} string "Internal server error"
// @ID deleteField
// @Router /projects/{projectUID}/tables/{tableUID}/fields/{fieldUID} [delete]
func (schemalessRoute) DeleteField(ctx context.Context, hub *collab.Hub, project *db.Project, table *db.SLTable, field *db.SLField, tx dbutil.Transactor) error {
	if err := tx.Transaction(func(tx *gorm.DB) error {
		if err := db.NewSLFieldsStore(tx).DeleteByID(ctx.Request().Context(), field.ID); err != nil {
			return errors.Wrap(err, "delete field")
		}
		if err := db.NewSLShortcutJobsStore(tx).DeleteByField(ctx.Request().Context(), table.ID, field.UID); err != nil {
			return errors.Wrap(err, "delete shortcut jobs")
		}
		return db.NewSLRecordsStore(tx).RemoveFieldData(ctx.Request().Context(), table.ID, field.UID)
	}); err != nil {
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to delete sl field")
		return ctx.ApiServerError()
	}
	notifyDirty(ctx, hub, project, table, collab.DirtyScope{Fields: true, AllRecords: true})
	return ctx.Status(http.StatusNoContent)
}

// FieldTypes
// @Summary List field types
// @Description Return the labels of all the field types, keyed by field type.
// @Produce json
// @Param projectUID path string true "Project UID"
// @Success 200 {object} map[string]string
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @ID listFieldTypes
// @Router /projects/{projectUID}/tables/types [get]
func (schemalessRoute) FieldTypes(ctx context.Context) error {
	fieldTypes := make(map[db.SLFieldType]string, len(db.AllFieldTypes))
	for _, fieldType := range db.AllFieldTypes {
		fieldTypes[fieldType] = ctx.Tr(fieldType.LabelKey())
	}
	return ctx.ApiSuccess(fieldTypes)
}
