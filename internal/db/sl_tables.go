package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	escape "github.com/tj/go-pg-escape"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/sqlutil"
)

var _ SLTablesStore = (*slTables)(nil)

var SLTables SLTablesStore

type SLTablesStore interface {
	All(ctx context.Context) ([]*SLTable, error)
	AllByProjectID(ctx context.Context, projectID uint) ([]*SLTable, error)
	ListByProjectID(ctx context.Context, projectID uint, opts ListSLTableOptions) ([]*SLTable, int64, error)
	GetByID(ctx context.Context, tableID uint) (*SLTable, error)
	GetByUID(ctx context.Context, tableUID string) (*SLTable, error)
	GetByName(ctx context.Context, tableName string) (*SLTable, error)
	Create(ctx context.Context, projectID uint, opts CreateSLTableOptions) (*SLTable, error)
	Update(ctx context.Context, tableID uint, opts UpdateSLTableOptions) error
	DeleteByID(ctx context.Context, tableID uint) error
	CreateView(ctx context.Context, table *SLTable) error
	ViewCount(ctx context.Context, schemaName, tableName string) (int64, error)
	SetIncrementIndex(ctx context.Context, tableID uint, index int64) error

	QueryList(ctx context.Context, projectID uint, tableUID string, options QueryListSLTableOptions) ([]map[string]interface{}, int64, error)
	QueryFirst(ctx context.Context, projectID uint, tableUID string, options QueryFirstSLTableOptions) (map[string]interface{}, error)
	QueryInsert(ctx context.Context, projectID uint, tableUID string, options QueryInsertSLTableOptions) error
	QueryUpdate(ctx context.Context, projectID uint, tableUID string, options QueryUpdateSLTableOptions) error
	QueryDelete(ctx context.Context, projectID uint, tableUID string, options QueryDeleteSLTableOptions) (int64, error)
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

func (db *slTables) AllByProjectID(ctx context.Context, projectID uint) ([]*SLTable, error) {
	var slTables []*SLTable
	return slTables, db.WithContext(ctx).Model(&SLTable{}).Preload("Project").Where("project_id = ?", projectID).Find(&slTables).Error
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

		fieldNameUIDs := lo.SliceToMap(fields, func(field *SLField) (string, string) {
			return field.Name, fmt.Sprintf("sl_records.data->>'%s'", field.UID)
		})

		fieldsDefinition := []string{
			escape.Escape(`sl_records.uid AS _uid`),
			escape.Escape(`sl_records.created_at AS _created_at`),
		}
		for _, field := range fields {
			// Normal value types.
			recordValueQuery := escape.Escape("(sl_records.data ->> %L::text)", field.UID)

			switch field.Type {
			case IntFieldType:
				recordValueQuery += escape.Escape(`::INTEGER AS %I`, field.Name)
			case TextFieldType:
				recordValueQuery += escape.Escape(`::TEXT AS %I`, field.Name)
			case BoolFieldType:
				recordValueQuery += escape.Escape(`::BOOLEAN AS %I`, field.Name)
			case FloatFieldType:
				recordValueQuery += escape.Escape(`::DOUBLE PRECISION AS %I`, field.Name)
			case TimestampFieldType:
				recordValueQuery += escape.Escape(`::TIMESTAMP WITHOUT TIME ZONE AS %I`, field.Name)
			case DateFieldType:
				recordValueQuery += escape.Escape(`::DATE AS %I`, field.Name)
			case ReferenceFieldType:
				// The value is the UID of the referenced record.
				recordValueQuery += escape.Escape(`::TEXT AS %I`, field.Name)
			case GeneratedFieldType:
				expression := field.Expression()
				expression, err := sqlutil.SterilizeExpression(ctx, expression, fieldNameUIDs)
				if err != nil {
					return err
				}
				recordValueQuery = escape.Escape(`(%s) AS %I`, expression, field.Name)
			}
			fieldsDefinition = append(fieldsDefinition, recordValueQuery)
		}

		tableIDStr := cast.ToString(tableID)
		viewName := table.Name
		q := escape.Escape(`DROP VIEW IF EXISTS %I.%I`, schemaName, viewName)
		if err := tx.WithContext(ctx).Exec(q).Error; err != nil {
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

func (db *slTables) SetIncrementIndex(ctx context.Context, tableID uint, index int64) error {
	return db.WithContext(ctx).Model(&SLTable{}).Where("id = ?", tableID).Set("increment_index", index).Error
}

type QueryListSLTableOptions struct {
	Fields []string
	Filter clause.Expression
	Order  []string
	Limit  int
	Offset int
}

func (db *slTables) QueryList(ctx context.Context, projectID uint, tableUID string, options QueryListSLTableOptions) ([]map[string]interface{}, int64, error) {
	slTable, err := db.GetByUID(ctx, tableUID)
	if err != nil {
		return nil, 0, errors.Wrap(err, "get sl table by uid")
	}
	if slTable.ProjectID != projectID {
		return nil, 0, ErrSLTableNotFound
	}
	schemaName := slTable.Project.SchemaName
	tableName := slTable.Name

	selectFields := make([]string, 0, len(options.Fields))
	if len(options.Fields) == 1 && options.Fields[0] == "*" {
		selectFields = []string{"*"}
	} else {
		for _, field := range options.Fields {
			selectFields = append(selectFields, escape.Escape(`%I`, field))
		}
	}

	q := db.WithContext(ctx).
		Table(escape.Escape(`%I.%I`, schemaName, tableName)).
		Select(selectFields)
	if options.Filter != nil {
		q = q.Where(options.Filter)
	}
	if len(options.Order) > 0 {
		q = q.Order(strings.Join(options.Order, ", "))
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	if options.Limit > 0 {
		q = q.Limit(options.Limit)
	}
	if options.Offset > 0 {
		q = q.Offset(options.Offset)
	}

	result := make([]map[string]interface{}, 0)
	if err := q.Find(&result).Error; err != nil {
		return nil, 0, errors.Wrap(err, "scan")
	}
	return result, total, nil
}

type QueryFirstSLTableOptions struct {
	Fields []string
	Filter clause.Expression
}

func (db *slTables) QueryFirst(ctx context.Context, projectID uint, tableUID string, options QueryFirstSLTableOptions) (map[string]interface{}, error) {
	slTable, err := db.GetByUID(ctx, tableUID)
	if err != nil {
		return nil, errors.Wrap(err, "get sl table by uid")
	}
	if slTable.ProjectID != projectID {
		return nil, ErrSLTableNotFound
	}
	schemaName := slTable.Project.SchemaName
	tableName := slTable.Name

	selectFields := make([]string, 0, len(options.Fields))
	if len(options.Fields) == 1 && options.Fields[0] == "*" {
		selectFields = []string{"*"}
	} else {
		for _, field := range options.Fields {
			selectFields = append(selectFields, escape.Escape(`%I`, field))
		}
	}

	q := db.WithContext(ctx).
		Table(escape.Escape(`%I.%I`, schemaName, tableName)).
		Select(selectFields)
	if options.Filter != nil {
		q = q.Where(options.Filter)
	}

	result := make(map[string]interface{})
	q = q.Find(&result)
	if err := q.Error; err != nil {
		return nil, errors.Wrap(err, "scan")
	}
	if q.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return result, nil
}

type QueryInsertSLTableOptions struct {
	FieldValues map[string]interface{}
	Filter      clause.Expression
}

func (db *slTables) QueryInsert(ctx context.Context, projectID uint, tableUID string, options QueryInsertSLTableOptions) error {
	slTable, err := db.GetByUID(ctx, tableUID)
	if err != nil {
		return errors.Wrap(err, "get sl table by uid")
	}
	if slTable.ProjectID != projectID {
		return ErrSLTableNotFound
	}

	dataBytes, err := json.Marshal(options.FieldValues)
	if err != nil {
		return errors.Wrap(err, "marshal field values")
	}

	if err := db.WithContext(ctx).Model(&SLRecord{}).Create(&SLRecord{
		SLTableID: slTable.ID,
		Data:      dataBytes,
	}).Error; err != nil {
		return errors.Wrap(err, "create record")
	}
	return nil
}

type QueryUpdateSLTableOptions struct {
	FieldValues map[string]interface{}
	Filter      clause.Expression
}

func (db *slTables) QueryUpdate(ctx context.Context, projectID uint, tableUID string, options QueryUpdateSLTableOptions) error {
	slTable, err := db.GetByUID(ctx, tableUID)
	if err != nil {
		return errors.Wrap(err, "get sl table by uid")
	}
	if slTable.ProjectID != projectID {
		return ErrSLTableNotFound
	}
	schemaName := slTable.Project.SchemaName
	tableName := slTable.Name

	dataBytes, err := json.Marshal(options.FieldValues)
	if err != nil {
		return errors.Wrap(err, "marshal field values")
	}

	var uids []string
	q := db.WithContext(ctx).
		Table(escape.Escape(`%I.%I`, schemaName, tableName)).
		Select("_uid")
	if options.Filter != nil {
		q = q.Where(options.Filter)
	}
	if err := q.Scan(&uids).Error; err != nil {
		return errors.Wrap(err, "scan uids")
	}

	// None of the records match the filter.
	if options.Filter != nil && len(uids) == 0 {
		return nil
	}

	// Update the records in sl_record table.
	q = db.WithContext(ctx).Model(&SLRecord{})
	if len(uids) == 1 {
		q = q.Where("uid = ?", uids[0])
	} else if len(uids) > 1 {
		q = q.Where("uid IN ?", uids)
	} else {
		// Allow global update.
		q = q.Session(&gorm.Session{AllowGlobalUpdate: true})
	}

	if err := q.Updates(&SLRecord{
		SLTableID: slTable.ID,
		Data:      dataBytes,
	}).Error; err != nil {
		return errors.Wrap(err, "update record")
	}
	return nil
}

type QueryDeleteSLTableOptions struct {
	Filter clause.Expression
}

func (db *slTables) QueryDelete(ctx context.Context, projectID uint, tableUID string, options QueryDeleteSLTableOptions) (int64, error) {
	slTable, err := db.GetByUID(ctx, tableUID)
	if err != nil {
		return 0, errors.Wrap(err, "get sl table by uid")
	}
	if slTable.ProjectID != projectID {
		return 0, ErrSLTableNotFound
	}
	schemaName := slTable.Project.SchemaName
	tableName := slTable.Name

	// TODO: soft delete in sl_recrods.
	q := db.WithContext(ctx).
		Table(escape.Escape(`%I.%I`, schemaName, tableName))
	if options.Filter != nil {
		q = q.Where(options.Filter)
	}

	result := q.Delete(nil)
	if err := result.Error; err != nil {
		return 0, errors.Wrap(err, "delete")
	}
	return result.RowsAffected, nil
}
