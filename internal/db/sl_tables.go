package db

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ SLTablesStore = (*slTables)(nil)

var SLTables SLTablesStore

type SLTablesStore interface {
	Query(ctx context.Context, options QuerySLTableOptions) ([]*SLTable, int64, error)
	GetByID(ctx context.Context, tableID int64) (*SLTable, error)
	GetByUID(ctx context.Context, tableUID string) (*SLTable, error)
	Create(ctx context.Context, projectID int64, options CreateSLTableOptions) (*SLTable, error)
	Update(ctx context.Context, tableID int64, options UpdateSLTableOptions) error
	DeleteByID(ctx context.Context, tableID int64) error
}

func NewSLTablesStore(db *gorm.DB) SLTablesStore {
	return &slTables{db}
}

// SLTable represents the structure of the schemaless table.
type SLTable struct {
	dbutil.Model

	UID       string `gorm:"uniqueIndex:idx_sl_table_uid, where:deleted_at IS NULL"`
	ProjectID int64  `gorm:"index"`
	Name      string
}

func (slTable *SLTable) BeforeCreate(_ *gorm.DB) error {
	slTable.UID = "tbl" + randstr.String(13)
	return nil
}

type slTables struct {
	*gorm.DB
}

type QuerySLTableOptions struct {
	ProjectID int64
	dbutil.Pagination
}

// Query 按创建顺序分页返回项目下的数据表，以及该项目的数据表总数。
func (db *slTables) Query(ctx context.Context, options QuerySLTableOptions) ([]*SLTable, int64, error) {
	q := db.WithContext(ctx).Model(&SLTable{}).Where("project_id = ?", options.ProjectID)

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	limit, offset := options.LimitOffset()
	var slTables []*SLTable
	if err := q.Order("id DESC").Limit(limit).Offset(offset).Find(&slTables).Error; err != nil {
		return nil, 0, errors.Wrap(err, "find")
	}
	return slTables, count, nil
}

func (db *slTables) GetByID(ctx context.Context, tableID int64) (*SLTable, error) {
	return db.getBy(ctx, "id = ?", tableID)
}

func (db *slTables) GetByUID(ctx context.Context, tableUID string) (*SLTable, error) {
	return db.getBy(ctx, "uid = ?", tableUID)
}

var ErrSLTableNotFound = errors.New("sl_table does not exist")

func (db *slTables) getBy(ctx context.Context, where string, args ...interface{}) (*SLTable, error) {
	var slTable SLTable
	if err := db.WithContext(ctx).Model(&SLTable{}).Where(where, args...).First(&slTable).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLTableNotFound
		}
		return nil, err
	}
	return &slTable, nil
}

type CreateSLTableOptions struct {
	Name string
}

var ErrSLTableExists = errors.New("sl_table exists")

func (db *slTables) Create(ctx context.Context, projectID int64, options CreateSLTableOptions) (*SLTable, error) {
	slTable := &SLTable{
		ProjectID: projectID,
		Name:      options.Name,
	}
	if err := db.WithContext(ctx).Create(slTable).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_sl_table_uid") {
			return nil, ErrSLTableExists
		}
		return nil, err
	}
	return slTable, nil
}

type UpdateSLTableOptions struct {
	Name string
}

func (db *slTables) Update(ctx context.Context, tableID int64, options UpdateSLTableOptions) error {
	if err := db.WithContext(ctx).Model(&SLTable{}).Where("id = ?", tableID).Updates(map[string]interface{}{
		"name":       options.Name,
		"updated_at": dbutil.Now(),
	}).Error; err != nil {
		return errors.Wrap(err, "update")
	}
	return nil
}

func (db *slTables) DeleteByID(ctx context.Context, tableID int64) error {
	if err := db.WithContext(ctx).Delete(&SLTable{}, tableID).Error; err != nil {
		return errors.Wrap(err, "delete")
	}
	return nil
}
