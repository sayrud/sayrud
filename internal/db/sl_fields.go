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

var SLFields SLFieldsStore

type SLFieldsStore interface {
	ListByTableID(ctx context.Context, tableID int64) (SLFieldList, error)
	GetByID(ctx context.Context, fieldID int64) (*SLField, error)
	GetByUID(ctx context.Context, fieldUID string) (*SLField, error)
	Create(ctx context.Context, options CreateSLFieldOptions) (*SLField, error)
	SetLabel(ctx context.Context, fieldID int64, label string) error
	SetType(ctx context.Context, fieldID int64, slFieldType SLFieldType, metadata SLFieldMetadata) error
	SetMetadata(ctx context.Context, fieldID int64, metadata SLFieldMetadata) error
	SetPosition(ctx context.Context, fieldID int64, position int64) error
	DeleteByID(ctx context.Context, fieldID int64) error
	Count(ctx context.Context, tableID int64) (int64, error)
}

func NewSLFieldsStore(db *gorm.DB) SLFieldsStore {
	return &slFields{db}
}

type SLFieldList []*SLField

// SLField represents the table fields of schemaless tables.
type SLField struct {
	dbutil.Model

	SLTableID int64  `gorm:"index;uniqueIndex:idx_sl_table_id_uid, where:deleted_at IS NULL"`
	UID       string `gorm:"uniqueIndex:idx_sl_table_id_uid, where:deleted_at IS NULL"`
	Label     string
	Type      SLFieldType
	Metadata  datatypes.JSONType[SLFieldMetadata]
	Position  int
}

func (slField *SLField) BeforeCreate(_ *gorm.DB) error {
	slField.UID = "fld" + randstr.String(7)
	return nil
}

type slFields struct {
	*gorm.DB
}

// ListByTableID returns the table field list from the given schemaless table.
// It returns ErrSLTableNotFound if the table does not exist.
func (db *slFields) ListByTableID(ctx context.Context, tableID int64) (SLFieldList, error) {
	var slFields SLFieldList
	if err := db.WithContext(ctx).Model(&SLField{}).Where("sl_table_id = ?", tableID).Order("position ASC").Find(&slFields).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return slFields, nil
}

type CreateSLFieldOptions struct {
	SLTableID int64
	Label     string
	Type      SLFieldType
	Metadata  SLFieldMetadata
	Position  int
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
		// Fields below the given position will have their positions incremented by 1 to make space for the field being moved.
		if err := tx.WithContext(ctx).Model(&SLField{}).Where("position >= ?", position).Update("position", gorm.Expr("position + 1")).Error; err != nil {
			return errors.Wrap(err, "update below position")
		}

		// The field being moved will have its position set to the given position.
		if err := tx.WithContext(ctx).Model(&SLField{}).Where("id = ?", fieldID).Update("position", position).Error; err != nil {
			return errors.Wrap(err, "update new position")
		}
		return nil
	})
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
