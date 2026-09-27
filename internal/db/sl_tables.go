package db

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ SLTablesStore = (*slTables)(nil)

// SLTables is the default instance of the SLTablesStore.
var SLTables SLTablesStore

// SLTablesStore is the persistent interface for schemaless tables.
type SLTablesStore interface {
	// Query returns the paginated tables of the project in creation order, along with the total count of the project tables.
	Query(ctx context.Context, options QuerySLTableOptions) ([]*SLTable, int64, error)
	// GetByID returns the table with the given ID.
	// It returns ErrSLTableNotFound if the table does not exist.
	GetByID(ctx context.Context, tableID int64) (*SLTable, error)
	// GetByUID returns the table with the given UID.
	// It returns ErrSLTableNotFound if the table does not exist.
	GetByUID(ctx context.Context, tableUID string) (*SLTable, error)
	// Create creates a new table in the project.
	// It returns ErrSLTableExists if the generated UID collides.
	Create(ctx context.Context, projectID int64, options CreateSLTableOptions) (*SLTable, error)
	// Update updates the table with the given ID.
	Update(ctx context.Context, tableID int64, options UpdateSLTableOptions) error
	// DeleteByID deletes the table with the given ID, its fields, records and views are kept.
	DeleteByID(ctx context.Context, tableID int64) error
	// CountByProjectID returns the number of tables in the project.
	CountByProjectID(ctx context.Context, projectID int64) (int64, error)
	// IncreaseRev increases the revision of the table by one and returns the new revision.
	// The table row stays locked until the transaction ends, so the changes of a table are serialized.
	IncreaseRev(ctx context.Context, tableID int64) (int64, error)
}

func NewSLTablesStore(db *gorm.DB) SLTablesStore {
	return &slTables{db}
}

// SLTable represents the structure of the schemaless table.
type SLTable struct {
	// Model contains the primary key and the creation, update and deletion times.
	dbutil.Model

	// UID is the unique public identifier of the table, e.g. "tbl" followed by 13 random characters.
	UID string `gorm:"uniqueIndex:idx_sl_table_uid, where:deleted_at IS NULL"`
	// ProjectID is the ID of the project the table belongs to.
	ProjectID int64 `gorm:"index"`
	// Name is the display name of the table.
	Name string
	// Rev is the revision of the table data, it increases by one for each changeset.
	Rev int64 `gorm:"not null;default:0"`
}

func (slTable *SLTable) BeforeCreate(_ *gorm.DB) error {
	if slTable.UID == "" {
		slTable.UID = "tbl" + randstr.String(13)
	}
	return nil
}

type slTables struct {
	// DB is the database connection the store operates on.
	*gorm.DB
}

// QuerySLTableOptions are the options of querying the tables.
type QuerySLTableOptions struct {
	// ProjectID is the ID of the project whose tables are queried.
	ProjectID int64
	// Pagination is the page and page size of the list.
	dbutil.Pagination
}

func (db *slTables) Query(ctx context.Context, options QuerySLTableOptions) ([]*SLTable, int64, error) {
	q := db.WithContext(ctx).Model(&SLTable{}).Where("project_id = ?", options.ProjectID)

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return nil, 0, errors.Wrap(err, "count")
	}

	limit, offset := options.LimitOffset()
	var slTables []*SLTable
	if err := q.Order("id ASC").Limit(limit).Offset(offset).Find(&slTables).Error; err != nil {
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

// CreateSLTableOptions are the options of creating a table.
type CreateSLTableOptions struct {
	// Name is the display name of the table.
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

// UpdateSLTableOptions are the options of updating a table.
type UpdateSLTableOptions struct {
	// Name is the new display name of the table.
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

func (db *slTables) IncreaseRev(ctx context.Context, tableID int64) (int64, error) {
	var rev int64
	if err := db.WithContext(ctx).Raw(`UPDATE sl_tables SET rev = rev + 1 WHERE id = ? RETURNING rev`, tableID).Scan(&rev).Error; err != nil {
		return 0, errors.Wrap(err, "update")
	}
	return rev, nil
}

func (db *slTables) CountByProjectID(ctx context.Context, projectID int64) (int64, error) {
	var count int64
	if err := db.WithContext(ctx).Model(&SLTable{}).Where("project_id = ?", projectID).Count(&count).Error; err != nil {
		return 0, errors.Wrap(err, "count")
	}
	return count, nil
}
