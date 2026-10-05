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
	// GetByShareToken returns an enabled public share, excluding deleted tables.
	GetByShareToken(ctx context.Context, token string) (*SLTable, error)
	// Create creates a new table in the project.
	// It returns ErrSLTableExists if the generated UID collides.
	Create(ctx context.Context, projectID int64, options CreateSLTableOptions) (*SLTable, error)
	// Update updates the table with the given ID.
	Update(ctx context.Context, tableID int64, options UpdateSLTableOptions) error
	// DeleteByID deletes the table with the given ID, its fields, records and views are kept.
	DeleteByID(ctx context.Context, tableID int64) error
	// CountByProjectID returns the number of tables in the project.
	CountByProjectID(ctx context.Context, projectID int64) (int64, error)
	// CountByProjectIDs returns the number of tables keyed by project ID, the projects without tables are omitted.
	CountByProjectIDs(ctx context.Context, projectIDs []int64) (map[int64]int64, error)
	// Count returns the number of tables in the projects that are not deleted.
	Count(ctx context.Context) (int64, error)
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

	// Icon is the optional icon identifier displayed with the table name.
	Icon string `gorm:"not null;default:''"`
	// Color is the optional palette color of the table icon.
	Color string `gorm:"not null;default:''"`

	// Rev is the revision of the table data, it increases by one for each changeset.
	Rev int64 `gorm:"not null;default:0"`

	// Sharing settings are managed separately and never included in table responses.
	ShareToken           string `json:"-" gorm:"not null;default:'';uniqueIndex:idx_sl_table_share_token,where:share_token <> '' AND deleted_at IS NULL"`
	ShareEnabled         bool   `json:"-" gorm:"not null;default:false"`
	ShareIncludeChildren bool   `json:"-" gorm:"not null;default:false"`
	SharePasswordHash    string `json:"-" gorm:"not null;default:''"`
	// SharePasswordSealed lets managers retrieve the password without storing plaintext.
	SharePasswordSealed string `json:"-" gorm:"not null;default:''"`
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

func (db *slTables) GetByShareToken(ctx context.Context, token string) (*SLTable, error) {
	if token == "" {
		return nil, ErrSLTableNotFound
	}

	return db.getBy(ctx, "share_token = ? AND share_enabled = true", token)
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
	Name *string

	// Icon and Color replace the appearance when provided, empty strings clear it.
	Icon  *string
	Color *string

	ShareToken           *string
	ShareEnabled         *bool
	ShareIncludeChildren *bool
	SharePasswordHash    *string
	SharePasswordSealed  *string
}

func (db *slTables) Update(ctx context.Context, tableID int64, options UpdateSLTableOptions) error {
	updates := map[string]interface{}{"updated_at": dbutil.Now()}
	if options.Name != nil {
		updates["name"] = *options.Name
	}

	if options.Icon != nil {
		updates["icon"] = *options.Icon
	}
	if options.Color != nil {
		updates["color"] = *options.Color
	}

	if options.ShareToken != nil {
		updates["share_token"] = *options.ShareToken
	}
	if options.ShareEnabled != nil {
		updates["share_enabled"] = *options.ShareEnabled
	}
	if options.ShareIncludeChildren != nil {
		updates["share_include_children"] = *options.ShareIncludeChildren
	}
	if options.SharePasswordHash != nil {
		updates["share_password_hash"] = *options.SharePasswordHash
	}
	if options.SharePasswordSealed != nil {
		updates["share_password_sealed"] = *options.SharePasswordSealed
	}

	if err := db.WithContext(ctx).Model(&SLTable{}).Where("id = ?", tableID).Updates(updates).Error; err != nil {
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

func (db *slTables) CountByProjectIDs(ctx context.Context, projectIDs []int64) (map[int64]int64, error) {
	counts := make(map[int64]int64, len(projectIDs))
	if len(projectIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		ProjectID int64
		Count     int64
	}
	if err := db.WithContext(ctx).Model(&SLTable{}).
		Select("project_id, COUNT(*) AS count").
		Where("project_id IN ?", projectIDs).
		Group("project_id").
		Scan(&rows).Error; err != nil {
		return nil, errors.Wrap(err, "count")
	}
	for _, r := range rows {
		counts[r.ProjectID] = r.Count
	}
	return counts, nil
}

func (db *slTables) Count(ctx context.Context) (int64, error) {
	var count int64
	live := db.WithContext(ctx).Model(&Project{}).Select("id")
	if err := db.WithContext(ctx).Model(&SLTable{}).Where("project_id IN (?)", live).Count(&count).Error; err != nil {
		return 0, errors.Wrap(err, "count")
	}
	return count, nil
}
