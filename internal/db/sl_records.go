package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ SLRecordsStore = (*slRecords)(nil)

var SLRecords SLRecordsStore

type SLRecordsStore interface {
	GetByTableID(ctx context.Context, slTableID uint, opts GetByTableIDOptions) ([]*SLRecord, int64, error)
	GetView(ctx context.Context, table *SLTable, opts GetViewOptions) ([]map[string]interface{}, int64, error)
	GetByID(ctx context.Context, slRecordID uint) (*SLRecord, error)
	GetByUID(ctx context.Context, slRecordUID string) (*SLRecord, error)
	Create(ctx context.Context, slTableID uint, data map[uint]interface{}) (*SLRecord, error)
	Update(ctx context.Context, slRecordID uint, data map[uint]interface{}) error
	DeleteByID(ctx context.Context, slRecordID uint) error
}

func NewSLRecordsStore(db *gorm.DB) SLRecordsStore {
	return &slRecords{db}
}

// SLRecord represents the table records in schemaless tables.
type SLRecord struct {
	dbutil.Model
	SLTableID uint           `json:"-"`
	SLTable   SLTable        `gorm:"foreignKey:SLTableID" json:"-"`
	Data      datatypes.JSON `gorm:"type:jsonb" json:"data"`
}

type slRecords struct {
	*gorm.DB
}

type GetByTableIDOptions struct {
	Page     int
	PageSize int
}

func (db *slRecords) GetByTableID(ctx context.Context, slTableID uint, opts GetByTableIDOptions) ([]*SLRecord, int64, error) {
	var total int64
	q := db.WithContext(ctx).Model(&SLTable{})
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	limit, offset := dbutil.LimitOffset(opts.Page, opts.PageSize)
	var slRecords []*SLRecord
	return slRecords, total, q.Where("sl_table_id = ?", slTableID).Limit(limit).Offset(offset).Find(&slRecords).Error
}

type GetViewOptions struct {
	Page     int
	PageSize int
}

func (db *slRecords) GetView(ctx context.Context, table *SLTable, opts GetViewOptions) ([]map[string]interface{}, int64, error) {
	viewName := fmt.Sprintf("schemaless-%s", table.Name)

	var total int64
	q := db.WithContext(ctx).Table(viewName)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	limit, offset := dbutil.LimitOffset(opts.Page, opts.PageSize)
	var records []map[string]interface{}
	return records, total, q.Limit(limit).Offset(offset).Find(&records).Error
}

func (db *slRecords) GetByID(ctx context.Context, slRecordID uint) (*SLRecord, error) {
	return db.getBy(ctx, "id = ?", slRecordID)
}

func (db *slRecords) GetByUID(ctx context.Context, slRecordUID string) (*SLRecord, error) {
	return db.getBy(ctx, "uid = ?", slRecordUID)
}

func (db *slRecords) getBy(ctx context.Context, where string, args ...interface{}) (*SLRecord, error) {
	var slRecord SLRecord
	if err := db.WithContext(ctx).Model(&SLRecord{}).Where(where, args...).First(&slRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLRecordNotFound
		}
		return nil, err
	}
	return &slRecord, nil
}

func (db *slRecords) Create(ctx context.Context, slTableID uint, data map[uint]interface{}) (*SLRecord, error) {
	var slRecord *SLRecord

	return slRecord, db.Transaction(func(tx *gorm.DB) error {
		// Make sure the table exist.
		var slTable SLTable
		if err := tx.WithContext(ctx).Model(&SLTable{}).Where("id = ?", slTableID).First(&slTable).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSLTableNotFound
			}
			return errors.Wrap(err, "get table")
		}

		data, err := handleRecordData(ctx, tx, slTable, data)
		if err != nil {
			return errors.Wrap(err, "process and validate data")
		}

		// Ok, now we can encode the data.
		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return errors.Wrap(err, "encode data")
		}

		slRecord = &SLRecord{
			SLTableID: slTableID,
			Data:      jsonBytes,
		}
		if err := tx.WithContext(ctx).Create(slRecord).Error; err != nil {
			return errors.Wrap(err, "create sl_records")
		}
		return nil
	})
}

var ErrSLRecordNotFound = errors.New("sl_record does not exist")

func (db *slRecords) Update(ctx context.Context, slRecordID uint, data map[uint]interface{}) error {
	if _, err := db.GetByID(ctx, slRecordID); err != nil {
		return err
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(err, "encode data")
	}

	return db.WithContext(ctx).Model(&SLRecord{}).Where("id = ?", slRecordID).Updates(map[string]interface{}{
		"data": jsonBytes,
	}).Error
}

func (db *slRecords) DeleteByID(ctx context.Context, slRecordID uint) error {
	if _, err := db.GetByID(ctx, slRecordID); err != nil {
		return err
	}

	return db.WithContext(ctx).Model(&SLRecord{}).Delete(&SLRecord{}, slRecordID).Error
}

var ErrMissingRequiredField = errors.New("missing required field")
var ErrFieldTypeMismatch = errors.New("field type mismatch")

// handleRecordData validate the data type, check if the required field exist, set default value to the field.
func handleRecordData(ctx context.Context, db *gorm.DB, slTable SLTable, data map[uint]interface{}) (map[uint]interface{}, error) {
	// Get table fields.
	var slFields []*SLField
	if err := db.WithContext(ctx).Model(&SLField{}).Where("sl_table_id = ?", slTable.ID).Find(&slFields).Error; err != nil {
		return nil, errors.Wrap(err, "get table fields")
	}

	var incrementIndexFlag bool

	// Validate the input data fields.
	for _, field := range slFields {
		field := field

		val, ok := data[field.ID]
		if !ok {
			// Check if the field is a required field.
			if field.IsRequired() {
				return nil, ErrMissingRequiredField
			}
			// If the filed has default value, set it for this missing field.
			defaultValue, ok := field.Options[OptionsDefaultValue]
			if ok {
				data[field.ID] = defaultValue
			}
			// Set auto increment field value,
			// also set the incrementIndexFlag to ture, to make sure the increment index will be increased after the operation.
			if field.Type == IntFieldType && field.IsIncrementIndex() {
				data[field.ID] = slTable.IncrementIndex + 1
				incrementIndexFlag = true
			}
		} else {
			// Check if the field type match with the value.
			if !field.CheckValue(val) {
				return nil, ErrFieldTypeMismatch
			}
		}
	}

	if incrementIndexFlag {
		if err := db.WithContext(ctx).Model(&SLTable{}).Where("id = ?", slTable.ID).Set("increment_index", slTable.IncrementIndex+1).Error; err != nil {
			return nil, errors.Wrap(err, "add increment index")
		}
	}

	return data, nil
}
