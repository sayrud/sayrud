package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ SLImportsStore = (*slImports)(nil)

// SLImports is the default instance of the SLImportsStore.
var SLImports SLImportsStore

// SLImportsStore persists the imports of Excel / CSV files, so their progress can be read from any server instance.
type SLImportsStore interface {
	// Create creates a pending import of the uploaded file.
	Create(ctx context.Context, opts CreateSLImportOptions) (*SLImport, error)
	// GetByUID returns the import with the given UID.
	// It returns ErrSLImportNotFound if the import does not exist.
	GetByUID(ctx context.Context, uid string) (*SLImport, error)
	// Start marks a pending, failed or stale running import as running, it reports false if another execution owns it or it has succeeded.
	Start(ctx context.Context, id int64, staleBefore time.Time) (bool, error)
	// SetTable records the table the import writes to, e.g. the table created for the import.
	SetTable(ctx context.Context, id, tableID int64) error
	// SetProgress records the number of the rows imported, it also renews the liveness of the running import.
	SetProgress(ctx context.Context, id int64, done, total int) error
	// Finish marks the running import as succeeded, or failed with the reason if importErr is not nil.
	Finish(ctx context.Context, id int64, done int, importErr *ImportError) error
	// ListExpired returns the imports created before the given time.
	ListExpired(ctx context.Context, before time.Time) ([]*SLImport, error)
	// DeleteByID deletes the import.
	DeleteByID(ctx context.Context, id int64) error
}

func NewSLImportsStore(db *gorm.DB) SLImportsStore {
	return &slImports{db}
}

// SLImportStatus is the state of an import.
type SLImportStatus string // @name SLImportStatus

const (
	// SLImportPending is uploaded and waiting for the column mapping.
	SLImportPending SLImportStatus = "pending"
	// SLImportRunning is being written by a server instance.
	SLImportRunning SLImportStatus = "running"
	// SLImportSucceeded has written all the records.
	SLImportSucceeded SLImportStatus = "succeeded"
	// SLImportFailed has written nothing and can be started again.
	SLImportFailed SLImportStatus = "failed"
)

// ImportError is the reason of a failed import, Key is the message key translated in the language of the reader.
type ImportError struct {
	// Key is the message key.
	Key string `json:"key"`
	// Args are the arguments of the message.
	Args []string `json:"args,omitempty"`
}

// SLImport is an import of an Excel / CSV file into a table.
type SLImport struct {
	// Model contains the primary key and the creation, update and deletion times, UpdatedAt is also the liveness of a running import.
	dbutil.Model

	// UID is the public identifier of the import.
	UID string `gorm:"uniqueIndex:idx_sl_imports_uid, where:deleted_at IS NULL"`
	// ProjectID is the project of the target table.
	ProjectID int64 `gorm:"index"`
	// UserID is the uploader, only the uploader can start the import.
	UserID int64
	// FileID is the uploaded file kept in the storage until the import expires.
	FileID int64
	// FileName is the original name of the uploaded file.
	FileName string
	// Status is the state of the import.
	Status SLImportStatus `gorm:"type:varchar(16);not null"`
	// SLTableID is the table the import writes to, zero before it starts.
	SLTableID int64
	// Done is the number of the records inserted, they are visible only after the import succeeds.
	Done int
	// Total is the number of the records to insert.
	Total int
	// Error is the reason of the failure.
	Error datatypes.JSONType[ImportError] `gorm:"type:jsonb"`
}

// NewSLImportUID generates an import UID.
func NewSLImportUID() string {
	return "imp" + randstr.String(13)
}

type slImports struct {
	*gorm.DB
}

// CreateSLImportOptions are the options of creating an import.
type CreateSLImportOptions struct {
	// ProjectID is the project to import into.
	ProjectID int64
	// UserID is the uploader.
	UserID int64
	// FileID is the uploaded file.
	FileID int64
	// FileName is the original name of the uploaded file.
	FileName string
}

func (db *slImports) Create(ctx context.Context, opts CreateSLImportOptions) (*SLImport, error) {
	item := &SLImport{
		UID:       NewSLImportUID(),
		ProjectID: opts.ProjectID,
		UserID:    opts.UserID,
		FileID:    opts.FileID,
		FileName:  opts.FileName,
		Status:    SLImportPending,
	}
	if err := db.WithContext(ctx).Create(item).Error; err != nil {
		return nil, errors.Wrap(err, "create")
	}
	return item, nil
}

var ErrSLImportNotFound = errors.New("import does not exist")

func (db *slImports) GetByUID(ctx context.Context, uid string) (*SLImport, error) {
	var item SLImport
	if err := db.WithContext(ctx).Where("uid = ?", uid).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSLImportNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (db *slImports) Start(ctx context.Context, id int64, staleBefore time.Time) (bool, error) {
	result := db.WithContext(ctx).Model(&SLImport{}).
		Where("id = ? AND (status IN ? OR (status = ? AND updated_at < ?))", id, []SLImportStatus{SLImportPending, SLImportFailed}, SLImportRunning, staleBefore).
		Updates(map[string]interface{}{
			"status": SLImportRunning,
			"done":   0,
			"total":  0,
			"error":  datatypes.NewJSONType(ImportError{}),
		})
	if result.Error != nil {
		return false, errors.Wrap(result.Error, "update")
	}
	return result.RowsAffected > 0, nil
}

func (db *slImports) SetTable(ctx context.Context, id, tableID int64) error {
	return db.WithContext(ctx).Model(&SLImport{}).Where("id = ?", id).Update("sl_table_id", tableID).Error
}

func (db *slImports) SetProgress(ctx context.Context, id int64, done, total int) error {
	return db.WithContext(ctx).Model(&SLImport{}).Where("id = ? AND status = ?", id, SLImportRunning).
		Updates(map[string]interface{}{
			"done":  done,
			"total": total,
		}).Error
}

func (db *slImports) Finish(ctx context.Context, id int64, done int, importErr *ImportError) error {
	updates := map[string]interface{}{
		"status": SLImportSucceeded,
		"done":   done,
	}
	if importErr != nil {
		updates["status"] = SLImportFailed
		updates["error"] = datatypes.NewJSONType(*importErr)
	}
	return db.WithContext(ctx).Model(&SLImport{}).Where("id = ? AND status = ?", id, SLImportRunning).Updates(updates).Error
}

func (db *slImports) ListExpired(ctx context.Context, before time.Time) ([]*SLImport, error) {
	var items []*SLImport
	if err := db.WithContext(ctx).Where("created_at < ? AND status <> ?", before, SLImportRunning).Limit(100).Find(&items).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return items, nil
}

func (db *slImports) DeleteByID(ctx context.Context, id int64) error {
	return db.WithContext(ctx).Where("id = ?", id).Delete(&SLImport{}).Error
}
