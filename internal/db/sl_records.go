package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	escape "github.com/tj/go-pg-escape"
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
	Query(ctx context.Context, slTableID uint, options QuerySLRecordsOptions) ([]*SLRecord, error)
	Import(ctx context.Context, slTableID uint, options ImportSLRecordsOptions) error
	Create(ctx context.Context, slTableID uint, jsonBytes json.RawMessage) (*SLRecord, error)
	Update(ctx context.Context, slRecordID uint, jsonBytes json.RawMessage) error
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
	q := db.WithContext(ctx).Model(&SLTable{}).Where("sl_table_id = ?", slTableID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	limit, offset := dbutil.LimitOffset(opts.Page, opts.PageSize)
	var slRecords []*SLRecord
	return slRecords, total, q.Limit(limit).Offset(offset).Find(&slRecords).Error
}

type GetViewOptions struct {
	Page     int
	PageSize int
}

func (db *slRecords) GetView(ctx context.Context, table *SLTable, opts GetViewOptions) ([]map[string]interface{}, int64, error) {
	viewName := escape.Escape("%I.%I", table.Project.SchemaName, table.Name)

	var total int64
	q := db.WithContext(ctx).Table(viewName).Order("_created_at DESC")
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	limit, offset := dbutil.LimitOffset(opts.Page, opts.PageSize)
	records := make([]map[string]interface{}, 0)
	return records, total, q.Limit(limit).Offset(offset).Find(&records).Error
}

func (db *slRecords) GetByID(ctx context.Context, slRecordID uint) (*SLRecord, error) {
	return db.getBy(ctx, "id = ?", slRecordID)
}

func (db *slRecords) GetByUID(ctx context.Context, slRecordUID string) (*SLRecord, error) {
	return db.getBy(ctx, "uid = ?", slRecordUID)
}

type QuerySLRecordsOptions struct {
	FieldUID   string
	FieldValue string
}

func (db *slRecords) Query(ctx context.Context, slTableID uint, options QuerySLRecordsOptions) ([]*SLRecord, error) {
	var slRecords []*SLRecord
	fieldUID := strings.TrimSpace(options.FieldUID)
	fieldValue := "%" + options.FieldValue + "%"

	var field SLField
	if err := db.WithContext(ctx).Model(&SLField{}).Where("sl_table_id = ? AND uid = ?", slTableID, fieldUID).First(&field).Error; err != nil {
		return nil, errors.Wrap(err, "get field")
	}

	if err := db.WithContext(ctx).Model(&SLRecord{}).Where(fmt.Sprintf("sl_table_id = ? AND data->>'%s' LIKE ?", escape.Escape(field.UID)), slTableID, fieldValue).Find(&slRecords).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return slRecords, nil
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

type ImportSLRecordsOptions struct {
	Data map[string]json.RawMessage
}

func (db *slRecords) Import(ctx context.Context, slTableID uint, options ImportSLRecordsOptions) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for uid, jsonBytes := range options.Data {
			if err := tx.WithContext(ctx).Create(&SLRecord{
				Model: dbutil.Model{
					UID: uid,
				},
				SLTableID: slTableID,
				Data:      datatypes.JSON(jsonBytes),
			}).Error; err != nil {
				return errors.Wrap(err, "create")
			}
		}
		return nil
	})
}

func (db *slRecords) Create(ctx context.Context, slTableID uint, jsonBytes json.RawMessage) (*SLRecord, error) {
	slRecord := &SLRecord{
		SLTableID: slTableID,
		Data:      datatypes.JSON(jsonBytes),
	}
	if err := db.WithContext(ctx).Create(slRecord).Error; err != nil {
		return nil, errors.Wrap(err, "create sl_records")
	}
	return slRecord, nil
}

var ErrSLRecordNotFound = errors.New("sl_record does not exist")

func (db *slRecords) Update(ctx context.Context, slRecordID uint, jsonBytes json.RawMessage) error {
	if _, err := db.GetByID(ctx, slRecordID); err != nil {
		return err
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
