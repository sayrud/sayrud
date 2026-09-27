package db

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ SLFieldsStore = (*slFields)(nil)

// SLFields is the default instance of the SLFieldsStore.
var SLFields SLFieldsStore

// SLFieldsStore is the persistent interface for the fields of schemaless tables.
type SLFieldsStore interface {
	// ListByTableID returns all the fields of the table ordered by position.
	ListByTableID(ctx context.Context, tableID int64) (SLFieldList, error)
	// GetByID returns the field with the given ID.
	// It returns ErrSLFieldNotFound if the field does not exist.
	GetByID(ctx context.Context, fieldID int64) (*SLField, error)
	// GetByUID returns the field with the given UID.
	// It returns ErrSLFieldNotFound if the field does not exist.
	GetByUID(ctx context.Context, fieldUID string) (*SLField, error)
	// Create creates a new field in the table.
	// It returns ErrUnexpectedType if the field type is unknown, and ErrSLFieldExists if the UID has been used in the table.
	Create(ctx context.Context, options CreateSLFieldOptions) (*SLField, error)
	// SetLabel sets the label of the field.
	SetLabel(ctx context.Context, fieldID int64, label string) error
	// SetType sets the type of the field along with the metadata of the new type, the record values are not converted.
	SetType(ctx context.Context, fieldID int64, slFieldType SLFieldType, metadata SLFieldMetadata) error
	// SetMetadata replaces the metadata of the field.
	SetMetadata(ctx context.Context, fieldID int64, metadata SLFieldMetadata) error
	// SetPosition sets the position of the field, the fields at or after the position are moved back by one.
	SetPosition(ctx context.Context, fieldID int64, position int64) error
	// Move moves the field to the zero-based index among the table fields, and renumbers all the field positions.
	Move(ctx context.Context, tableID, fieldID int64, index int) error
	// DeleteByID deletes the field with the given ID, the record values of the field are kept.
	DeleteByID(ctx context.Context, fieldID int64) error
	// Count returns the number of fields in the table.
	Count(ctx context.Context, tableID int64) (int64, error)
}

func NewSLFieldsStore(db *gorm.DB) SLFieldsStore {
	return &slFields{db}
}

// SLFieldList is a list of fields.
type SLFieldList []*SLField

// SLField represents the table fields of schemaless tables.
type SLField struct {
	// Model contains the primary key and the creation, update and deletion times.
	dbutil.Model

	// SLTableID is the ID of the table the field belongs to.
	SLTableID int64 `gorm:"index;uniqueIndex:idx_sl_table_id_uid, where:deleted_at IS NULL"`
	// UID is the public identifier of the field unique in the table, e.g. "fld" followed by 7 random characters.
	// The record values are keyed by it, so renaming the field does not affect the records.
	UID string `gorm:"uniqueIndex:idx_sl_table_id_uid, where:deleted_at IS NULL"`
	// Label is the display name of the field, unique in the table.
	Label string
	// Type is the type of the field values.
	Type SLFieldType
	// Metadata is the type-specific configuration, e.g. select options, number format or formula expression.
	Metadata datatypes.JSONType[SLFieldMetadata]
	// Position is the order of the field in the table, starting from 0. The first field is the primary field.
	Position int
}

func (slField *SLField) BeforeCreate(_ *gorm.DB) error {
	if slField.UID == "" {
		slField.UID = "fld" + randstr.String(7)
	}
	return nil
}

type slFields struct {
	// DB is the database connection the store operates on.
	*gorm.DB
}

func (db *slFields) ListByTableID(ctx context.Context, tableID int64) (SLFieldList, error) {
	var slFields SLFieldList
	if err := db.WithContext(ctx).Model(&SLField{}).Where("sl_table_id = ?", tableID).Order("position ASC, id ASC").Find(&slFields).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return slFields, nil
}

// CreateSLFieldOptions are the options of creating a field.
type CreateSLFieldOptions struct {
	// UID is generated randomly if empty.
	UID string
	// SLTableID is the ID of the table the field belongs to.
	SLTableID int64
	// Label is the display name of the field.
	Label string
	// Type is the type of the field values.
	Type SLFieldType
	// Metadata is the type-specific configuration.
	Metadata SLFieldMetadata
	// Position is the order of the field in the table.
	Position int
}

var (
	ErrSLFieldExists  = errors.New("sl_field exists")
	ErrUnexpectedType = errors.New("unexpected type")
)

// Create creates the field in the given schemaless table.
func (db *slFields) Create(ctx context.Context, options CreateSLFieldOptions) (*SLField, error) {
	if !options.Type.Check() {
		return nil, ErrUnexpectedType
	}

	slField := &SLField{
		UID:       options.UID,
		SLTableID: options.SLTableID,
		Label:     options.Label,
		Type:      options.Type,
		Metadata:  datatypes.NewJSONType(options.Metadata),
		Position:  options.Position,
	}
	if err := db.WithContext(ctx).Create(slField).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_sl_table_id_uid") {
			return nil, ErrSLFieldExists
		}
		return nil, err
	}
	return slField, nil
}

// GetByID returns the field with the given ID.
func (db *slFields) GetByID(ctx context.Context, fieldID int64) (*SLField, error) {
	return db.getBy(ctx, "id = ?", fieldID)
}

func (db *slFields) GetByUID(ctx context.Context, fieldUID string) (*SLField, error) {
	return db.getBy(ctx, "uid = ?", fieldUID)
}

func (db *slFields) getBy(ctx context.Context, where string, args ...interface{}) (*SLField, error) {
	var slField SLField
	if err := db.WithContext(ctx).Model(&SLField{}).Where(where, args...).First(&slField).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLFieldNotFound
		}
		return nil, err
	}
	return &slField, nil
}

func (db *slFields) SetLabel(ctx context.Context, fieldID int64, label string) error {
	return db.set(ctx, fieldID, map[string]interface{}{
		"label": label,
	})
}

func (db *slFields) SetType(ctx context.Context, fieldID int64, slFieldType SLFieldType, metadata SLFieldMetadata) error {
	return db.set(ctx, fieldID, map[string]interface{}{
		"type":     slFieldType,
		"metadata": datatypes.NewJSONType(metadata),
	})
}

func (db *slFields) SetMetadata(ctx context.Context, fieldID int64, metadata SLFieldMetadata) error {
	return db.set(ctx, fieldID, map[string]interface{}{
		"metadata": datatypes.NewJSONType(metadata),
	})
}

func (db *slFields) SetPosition(ctx context.Context, fieldID int64, position int64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var tableID int64
		if err := tx.WithContext(ctx).Model(&SLField{}).Where("id = ?", fieldID).Pluck("sl_table_id", &tableID).Error; err != nil {
			return errors.Wrap(err, "get table id")
		}

		// Fields below the given position will have their positions incremented by 1 to make space for the field being moved.
		if err := tx.WithContext(ctx).Model(&SLField{}).Where("sl_table_id = ? AND position >= ?", tableID, position).Update("position", gorm.Expr("position + 1")).Error; err != nil {
			return errors.Wrap(err, "update below position")
		}

		// The field being moved will have its position set to the given position.
		if err := tx.WithContext(ctx).Model(&SLField{}).Where("id = ?", fieldID).Update("position", position).Error; err != nil {
			return errors.Wrap(err, "update new position")
		}
		return nil
	})
}

func (db *slFields) Move(ctx context.Context, tableID, fieldID int64, index int) error {
	fields, err := db.ListByTableID(ctx, tableID)
	if err != nil {
		return errors.Wrap(err, "list fields")
	}
	return movePositions(ctx, db.DB, &SLField{}, fields, func(f *SLField) int64 { return f.ID }, fieldID, index)
}

func (db *slFields) set(ctx context.Context, id int64, fields map[string]interface{}) error {
	fields["updated_at"] = dbutil.Now()
	if err := db.WithContext(ctx).Model(&SLField{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return errors.Wrap(err, "update")
	}
	return nil
}

var ErrSLFieldNotFound = errors.New("sl_field does not exist")

// DeleteByID deletes the field by the given fieldID ID.
func (db *slFields) DeleteByID(ctx context.Context, fieldID int64) error {
	if err := db.WithContext(ctx).Delete(&SLField{}, fieldID).Error; err != nil {
		return errors.Wrap(err, "delete")
	}
	return nil
}

func (db *slFields) Count(ctx context.Context, tableID int64) (int64, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&SLField{}).Where("sl_table_id = ?", tableID).Count(&count).Error; err != nil {
		return 0, errors.Wrap(err, "count")
	}
	return count, nil
}
