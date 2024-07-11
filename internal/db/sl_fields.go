package db

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ SLFieldsStore = (*slFields)(nil)

var SLFields SLFieldsStore

type SLFieldsStore interface {
	List(ctx context.Context, tableID uint) ([]*SLField, error)
	GetByID(ctx context.Context, fieldID uint) (*SLField, error)
	GetByUID(ctx context.Context, fieldUID string) (*SLField, error)
	GetByTableID(ctx context.Context, tableID uint) ([]*SLField, error)
	Create(ctx context.Context, opts CreateSLFieldOptions) (*SLField, error)
	Update(ctx context.Context, fieldID uint, opts UpdateSLFieldOptions) error
	DeleteByID(ctx context.Context, fieldID uint) error
	Count(ctx context.Context, tableID uint) (int64, error)
}

func NewSLFieldsStore(db *gorm.DB) SLFieldsStore {
	return &slFields{db}
}

// SLField represents the table fields of schemaless tables.
type SLField struct {
	dbutil.Model
	SLTableID uint           `gorm:"uniqueIndex:idx_sl_tables_id_name, where:deleted_at IS NULL" json:"-"`
	SLTable   SLTable        `gorm:"foreignKey:SLTableID" json:"-"`
	Name      string         `gorm:"uniqueIndex:idx_sl_tables_id_name, where:deleted_at IS NULL" json:"name"`
	Label     string         `json:"label"`
	Type      SLFieldType    `json:"type"`
	Options   dbutil.Options `json:"options"`
	Position  int            `json:"position"`
}

const OptionsRequired = "required"
const OptionsIncrementIndex = "increment_index"
const OptionsDefaultValue = "default"
const OptionsReferenceUID = "reference_uid"
const OptionsExpression = "expression"

func (f *SLField) IsRequired() bool {
	_, ok := f.Options[OptionsRequired]
	return ok
}

func (f *SLField) IsIncrementIndex() bool {
	_, ok := f.Options[OptionsIncrementIndex]
	return ok
}

func (f *SLField) ReferenceUID() string {
	v, _ := f.Options[OptionsReferenceUID]
	return cast.ToString(v)
}

func (f *SLField) Expression() string {
	v, _ := f.Options[OptionsExpression]
	return cast.ToString(v)
}

func (f *SLField) CheckValue(val interface{}) bool {
	if f.Type.IsGeneratedValue() {
		return true
	}

	kind := reflect.TypeOf(val).Kind()
	_, ok := internalKindMatch[kind]
	return ok
}

type slFields struct {
	*gorm.DB
}

// List returns the table field list from the given schemaless table.
// It returns ErrSLTableNotFound if the table does not exist.
func (db *slFields) List(ctx context.Context, tableID uint) ([]*SLField, error) {
	var slFields []*SLField
	return slFields, db.WithContext(ctx).Model(&SLField{}).Preload("SLTable").Where("sl_table_id = ?", tableID).Order("position ASC").Find(&slFields).Error
}

type CreateSLFieldOptions struct {
	SLTableID uint
	Name      string
	Label     string
	Type      SLFieldType
	Options   dbutil.Options
	Position  int
}

var ErrSLFieldExists = errors.New("sl_field exists")
var ErrUnexpectedType = errors.New("unexpected type")

// Create creates the field in the given schemaless table.
// It returns ErrSLTableNotFound if the table is not exists.
// It returns ErrUnexpectedType if the type of the field is incorrect.
// It returns ErrSLFieldExists if the field name already existed.
func (db *slFields) Create(ctx context.Context, opts CreateSLFieldOptions) (*SLField, error) {
	// Check the field type.
	if !opts.Type.Check() {
		return nil, ErrUnexpectedType
	}

	slField := &SLField{
		SLTableID: opts.SLTableID,
		Name:      opts.Name,
		Label:     opts.Label,
		Type:      opts.Type,
		Options:   opts.Options,
		Position:  opts.Position,
	}
	if err := db.WithContext(ctx).Create(slField).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_sl_tables_id_name") {
			return nil, ErrSLFieldExists
		}
		return nil, err
	}
	return slField, nil
}

// GetByID returns the field with the given ID.
// It returns ErrSLFieldNotFound if the field does not exist.
func (db *slFields) GetByID(ctx context.Context, fieldID uint) (*SLField, error) {
	return db.getBy(ctx, "id = ?", fieldID)
}

func (db *slFields) GetByUID(ctx context.Context, fieldUID string) (*SLField, error) {
	return db.getBy(ctx, "uid = ?", fieldUID)
}

func (db *slFields) GetByTableID(ctx context.Context, tableID uint) ([]*SLField, error) {
	var slFields []*SLField
	return slFields, db.WithContext(ctx).Model(&SLField{}).Preload("SLTable").Where("sl_table_id = ?", tableID).Order("position ASC").Find(&slFields).Error
}

func (db *slFields) getBy(ctx context.Context, where string, args ...interface{}) (*SLField, error) {
	var slField SLField
	if err := db.WithContext(ctx).Model(&SLField{}).Preload("SLTable").Where(where, args...).First(&slField).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLFieldNotFound
		}
		return nil, err
	}
	return &slField, nil
}

type UpdateSLFieldOptions struct {
	Name     string
	Label    string
	Type     SLFieldType
	Options  dbutil.Options
	Position int
}

func (db *slFields) Update(ctx context.Context, fieldID uint, opts UpdateSLFieldOptions) error {
	_, err := db.GetByID(ctx, fieldID)
	if err != nil {
		return errors.Wrap(err, "get by id")
	}

	// Check the field type.
	if !opts.Type.Check() {
		return ErrUnexpectedType
	}

	if err := db.WithContext(ctx).Model(&SLField{}).Where("id = ?", fieldID).Updates(map[string]interface{}{
		"name":     opts.Name,
		"label":    opts.Label,
		"type":     opts.Type,
		"options":  opts.Options,
		"position": opts.Position,
	}).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_sl_tables_id_name") {
			return ErrSLFieldExists
		}
		return err
	}
	return nil
}

var ErrSLFieldNotFound = errors.New("sl_field does not exist")

// DeleteByID deletes the field by the given fieldID ID.
// It returns ErrSLFieldNotFound if the field does not exist.
func (db *slFields) DeleteByID(ctx context.Context, fieldID uint) error {
	return db.WithContext(ctx).Delete(&SLField{}, fieldID).Error
}

func (db *slFields) Count(ctx context.Context, tableID uint) (int64, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&SLField{}).Where("sl_table_id = ?", tableID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
