package db

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
	escape "github.com/tj/go-pg-escape"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ SLTablesStore = (*slTables)(nil)

var SLTables SLTablesStore

type SLTablesStore interface {
	All(ctx context.Context) ([]*SLTable, error)
	ListByProjectID(ctx context.Context, projectID uint, opts ListSLTableOptions) ([]*SLTable, int64, error)
	GetByID(ctx context.Context, tableID uint) (*SLTable, error)
	GetByUID(ctx context.Context, tableUID string) (*SLTable, error)
	GetByName(ctx context.Context, tableName string) (*SLTable, error)
	Create(ctx context.Context, projectID uint, opts CreateSLTableOptions) (*SLTable, error)
	Update(ctx context.Context, tableID uint, opts UpdateSLTableOptions) error
	DeleteByID(ctx context.Context, tableID uint) error
	CreateView(ctx context.Context, table *SLTable) error
	ViewCount(ctx context.Context, schemaName, tableName string) (int64, error)
}

func NewSLTablesStore(db *gorm.DB) SLTablesStore {
	return &slTables{db}
}

// SLTable represents the structure of the schemaless table.
type SLTable struct {
	dbutil.Model
	ProjectID      uint    `gorm:"uniqueIndex:idx_sl_table_project_id_name, where:deleted_at IS NULL" json:"-"`
	Project        Project `gorm:"foreignKey:ProjectID" json:"-"`
	Name           string  `gorm:"uniqueIndex:idx_sl_table_project_id_name, where:deleted_at IS NULL" json:"name"`
	Label          string  `json:"label"`
	Desc           string  `json:"desc"`
	IncrementIndex int64   `json:"incrementIndex"`
}

type slTables struct {
	*gorm.DB
}

func (db *slTables) All(ctx context.Context) ([]*SLTable, error) {
	var slTables []*SLTable
	return slTables, db.WithContext(ctx).Model(&SLTable{}).Preload("Project").Find(&slTables).Error
}

type ListSLTableOptions struct {
	Page     int
	PageSize int
}

func (db *slTables) ListByProjectID(ctx context.Context, projectID uint, opts ListSLTableOptions) ([]*SLTable, int64, error) {
	var total int64
	q := db.WithContext(ctx).Model(&SLTable{}).Preload("Project").Where("project_id = ?", projectID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	limit, offset := dbutil.LimitOffset(opts.Page, opts.PageSize)
	var slTables []*SLTable
	if err := q.Order("id ASC").Limit(limit).Offset(offset).Find(&slTables).Error; err != nil {
		return nil, 0, err
	}
	return slTables, total, nil
}

var ErrSLTableNotFound = errors.New("sl_table dose not exist")

func (db *slTables) getBy(ctx context.Context, where string, args ...interface{}) (*SLTable, error) {
	var slTable SLTable
	if err := db.WithContext(ctx).Model(&SLTable{}).Preload("Project").Where(where, args...).First(&slTable).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLTableNotFound
		}
		return nil, err
	}
	return &slTable, nil
}

func (db *slTables) GetByID(ctx context.Context, tableID uint) (*SLTable, error) {
	return db.getBy(ctx, "id = ?", tableID)
}

func (db *slTables) GetByUID(ctx context.Context, tableUID string) (*SLTable, error) {
	return db.getBy(ctx, "uid = ?", tableUID)
}

func (db *slTables) GetByName(ctx context.Context, tableName string) (*SLTable, error) {
	return db.getBy(ctx, "name = ?", tableName)
}

type CreateSLTableOptions struct {
	Name  string
	Label string
	Desc  string
}

var ErrSLTableExists = errors.New("sl_tables exists")

func (db *slTables) Create(ctx context.Context, projectID uint, opts CreateSLTableOptions) (*SLTable, error) {
	slTable := &SLTable{
		ProjectID: projectID,
		Name:      opts.Name,
		Label:     opts.Label,
		Desc:      opts.Desc,
	}
	if err := db.WithContext(ctx).Create(slTable).Error; err != nil {
		if dbutil.IsUniqueViolation(err, "idx_sl_table_project_id_name") {
			return nil, ErrSLTableExists
		}
		return nil, err
	}
	return slTable, nil
}

type UpdateSLTableOptions struct {
	Name  string
	Label string
	Desc  string
}

func (db *slTables) Update(ctx context.Context, tableID uint, opts UpdateSLTableOptions) error {
	_, err := db.GetByID(ctx, tableID)
	if err != nil {
		return err
	}

	return db.WithContext(ctx).Model(&SLTable{}).
		Where("id = ?", tableID).
		Updates(map[string]interface{}{
			"name":  opts.Name,
			"label": opts.Label,
			"desc":  opts.Desc,
		}).Error
}

func (db *slTables) DeleteByID(ctx context.Context, tableID uint) error {
	_, err := db.GetByID(ctx, tableID)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Delete(&SLTable{}, tableID).Error
}

func (db *slTables) CreateView(ctx context.Context, table *SLTable) error {
	tableID := table.ID
	schemaName := table.Project.SchemaName

	return db.Transaction(func(tx *gorm.DB) error {
		var fields []*SLField
		if err := tx.WithContext(ctx).Model(&SLField{}).Where("sl_table_id = ?", tableID).Order("position ASC").Find(&fields).Error; err != nil {
			return errors.Wrap(err, "get table fields")
		}

		fieldsDefinition := []string{
			escape.Escape(`sl_records.id AS _id`),
			escape.Escape(`sl_records.created_at AS _created_at`),
		}
		for _, field := range fields {
			recordValueQuery := escape.Escape("(sl_records.data ->> %L::text)", cast.ToString(field.ID))

			switch field.Type {
			case IntFieldType:
				recordValueQuery += escape.Escape(`::INTEGER AS %I`, field.Name)
			case TextFieldType:
				recordValueQuery += escape.Escape(`::TEXT AS %I`, field.Name)
			case BoolFieldType:
				recordValueQuery += escape.Escape(`::BOOLEAN AS %I`, field.Name)
			case FloatFieldType:
				recordValueQuery += escape.Escape(`::DOUBLE PRECISION AS %I`, field.Name)
			}
			fieldsDefinition = append(fieldsDefinition, recordValueQuery)
		}

		tableIDStr := cast.ToString(tableID)
		viewName := table.Name
		q := escape.Escape(`DROP VIEW IF EXISTS %I.%I`, schemaName, viewName)
		if err := tx.WithContext(ctx).Debug().Exec(q).Error; err != nil {
			return errors.Wrap(err, "drop old view")
		}

		q = escape.Escape(`CREATE VIEW %I.%I AS
	SELECT
	`+strings.Join(fieldsDefinition, ", ")+`
	FROM public.sl_records
	WHERE sl_records.sl_table_id = %L AND sl_records.deleted_at IS NULL`, schemaName, viewName, tableIDStr)
		if err := tx.WithContext(ctx).Debug().Exec(q).Error; err != nil {
			return errors.Wrap(err, "create new view")
		}

		return nil
	})
}

func (db *slTables) ViewCount(ctx context.Context, schemaName, tableName string) (int64, error) {
	var count int64
	if err := db.WithContext(ctx).Raw(escape.Escape("SELECT COUNT(*) FROM %I.%I", schemaName, tableName)).Scan(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
