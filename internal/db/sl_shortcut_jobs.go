package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ SLShortcutJobsStore = (*slShortcutJobs)(nil)

// SLShortcutJobs is the default instance of the SLShortcutJobsStore.
var SLShortcutJobs SLShortcutJobsStore

// SLShortcutJobsStore is the persistent queue of generating the cell values by the field shortcuts.
//
// There is at most one job for a cell, and the job is deleted once the value is written back, so the queue also tells
// which cells are being generated or have failed.
type SLShortcutJobsStore interface {
	// Enqueue queues the cells of the field for generating, the existing jobs of the cells are reset to pending and get a new token,
	// so the results of their running executions are discarded.
	Enqueue(ctx context.Context, tableID int64, fieldUID string, recordUIDs []string) ([]*SLShortcutJob, error)
	// Claim marks the next due pending job as running and returns it, it returns nil if there is none.
	// It is safe to claim concurrently from multiple server instances.
	Claim(ctx context.Context) (*SLShortcutJob, error)
	// Retry puts the job back to pending to run after the given time, unless it has been re-enqueued.
	Retry(ctx context.Context, job *SLShortcutJob, runAfter time.Time, jobErr ShortcutJobError) (bool, error)
	// Fail marks the job as failed, unless it has been re-enqueued.
	Fail(ctx context.Context, job *SLShortcutJob, jobErr ShortcutJobError) (bool, error)
	// Finish deletes the job unless it has been re-enqueued, it reports whether the job is deleted.
	Finish(ctx context.Context, job *SLShortcutJob) (bool, error)
	// ListByTableID returns the jobs of the table.
	ListByTableID(ctx context.Context, tableID int64) ([]*SLShortcutJob, error)
	// DeleteByField deletes the jobs of the field.
	DeleteByField(ctx context.Context, tableID int64, fieldUID string) error
	// DeleteByRecords deletes the jobs of the records.
	DeleteByRecords(ctx context.Context, tableID int64, recordUIDs []string) error
	// ResetStale puts the jobs running since before the given time back to pending, e.g. the server crashed when running them.
	ResetStale(ctx context.Context, before time.Time) (int64, error)
}

func NewSLShortcutJobsStore(db *gorm.DB) SLShortcutJobsStore {
	return &slShortcutJobs{db}
}

// ShortcutJobStatus is the state of a shortcut job.
type ShortcutJobStatus string

const (
	ShortcutJobPending ShortcutJobStatus = "pending"
	ShortcutJobRunning ShortcutJobStatus = "running"
	ShortcutJobFailed  ShortcutJobStatus = "failed"
)

// ShortcutJobError is the reason of a failed job, Key is the message key translated in the language of the reader.
type ShortcutJobError struct {
	// Key is the message key, empty if Detail is the whole message.
	Key string `json:"key,omitempty"`
	// Args are the arguments of the message.
	Args []string `json:"args,omitempty"`
	// Detail is the untranslated text, e.g. the message of the error thrown by the script.
	Detail string `json:"detail,omitempty"`
}

// SLShortcutJob is the job of generating the value of a cell by the shortcut of its field.
type SLShortcutJob struct {
	// ID is the primary key.
	ID int64 `gorm:"primarykey"`
	// SLTableID is the table of the cell.
	SLTableID int64 `gorm:"not null;uniqueIndex:idx_sl_shortcut_jobs_cell"`
	// FieldUID is the field of the cell, whose shortcut generates the value.
	FieldUID string `gorm:"type:varchar(16);not null;uniqueIndex:idx_sl_shortcut_jobs_cell"`
	// RecordUID is the record of the cell.
	RecordUID string `gorm:"type:varchar(16);not null;uniqueIndex:idx_sl_shortcut_jobs_cell"`
	// Status is the state of the job.
	Status ShortcutJobStatus `gorm:"type:varchar(16);not null;index:idx_sl_shortcut_jobs_due,priority:1"`
	// Token increases every time the job is enqueued, the result of an execution is written back only if the token is unchanged.
	Token int64 `gorm:"not null;default:1"`
	// Attempts is the number of the executions since the job was last enqueued.
	Attempts int `gorm:"not null;default:0"`
	// RunAfter is the earliest time the pending job can be claimed.
	RunAfter time.Time `gorm:"not null;index:idx_sl_shortcut_jobs_due,priority:2"`
	// LockedAt is the time the running job was claimed, nil if not running.
	LockedAt *time.Time
	// Error is the reason of the failure, or of the last attempt of a pending retry.
	Error datatypes.JSONType[ShortcutJobError] `gorm:"type:jsonb;not null"`
	// CreatedAt is the time the job was first enqueued.
	CreatedAt time.Time
	// UpdatedAt is the time the job was last updated.
	UpdatedAt time.Time
}

type slShortcutJobs struct {
	*gorm.DB
}

// enqueueBatchSize keeps the number of the bind parameters of a statement far below the limit of PostgreSQL.
const enqueueBatchSize = 1000

func (db *slShortcutJobs) Enqueue(ctx context.Context, tableID int64, fieldUID string, recordUIDs []string) ([]*SLShortcutJob, error) {
	// A statement can not upsert the same row twice.
	recordUIDs = lo.Uniq(recordUIDs)
	now := dbutil.Now()

	result := make([]*SLShortcutJob, 0, len(recordUIDs))
	for start := 0; start < len(recordUIDs); start += enqueueBatchSize {
		batch := recordUIDs[start:min(start+enqueueBatchSize, len(recordUIDs))]
		jobs := make([]*SLShortcutJob, 0, len(batch))
		for _, recordUID := range batch {
			jobs = append(jobs, &SLShortcutJob{
				SLTableID: tableID,
				FieldUID:  fieldUID,
				RecordUID: recordUID,
				Status:    ShortcutJobPending,
				Token:     1,
				RunAfter:  now,
				Error:     datatypes.NewJSONType(ShortcutJobError{}),
				CreatedAt: now,
				UpdatedAt: now,
			})
		}

		if err := db.WithContext(ctx).Clauses(
			clause.OnConflict{
				Columns: []clause.Column{{Name: "sl_table_id"}, {Name: "field_uid"}, {Name: "record_uid"}},
				DoUpdates: clause.Set{
					{Column: clause.Column{Name: "status"}, Value: ShortcutJobPending},
					{Column: clause.Column{Name: "token"}, Value: gorm.Expr("sl_shortcut_jobs.token + 1")},
					{Column: clause.Column{Name: "attempts"}, Value: 0},
					{Column: clause.Column{Name: "run_after"}, Value: now},
					{Column: clause.Column{Name: "locked_at"}, Value: nil},
					{Column: clause.Column{Name: "error"}, Value: datatypes.NewJSONType(ShortcutJobError{})},
					{Column: clause.Column{Name: "updated_at"}, Value: now},
				},
			},
			clause.Returning{},
		).Create(&jobs).Error; err != nil {
			return nil, errors.Wrap(err, "upsert")
		}
		result = append(result, jobs...)
	}
	return result, nil
}

func (db *slShortcutJobs) Claim(ctx context.Context) (*SLShortcutJob, error) {
	now := dbutil.Now()
	var jobs []*SLShortcutJob
	if err := db.WithContext(ctx).Raw(`
UPDATE sl_shortcut_jobs SET status = ?, locked_at = ?, attempts = attempts + 1, updated_at = ?
WHERE id = (
	SELECT id FROM sl_shortcut_jobs WHERE status = ? AND run_after <= ?
	ORDER BY run_after ASC, id ASC LIMIT 1 FOR UPDATE SKIP LOCKED
)
RETURNING *`, ShortcutJobRunning, now, now, ShortcutJobPending, now).Scan(&jobs).Error; err != nil {
		return nil, errors.Wrap(err, "claim")
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	return jobs[0], nil
}

// updateIfCurrent updates the job only if it has not been re-enqueued since claimed.
func (db *slShortcutJobs) updateIfCurrent(ctx context.Context, job *SLShortcutJob, values map[string]interface{}) (bool, error) {
	values["updated_at"] = dbutil.Now()
	result := db.WithContext(ctx).Model(&SLShortcutJob{}).Where("id = ? AND token = ?", job.ID, job.Token).Updates(values)
	if result.Error != nil {
		return false, errors.Wrap(result.Error, "update")
	}
	return result.RowsAffected > 0, nil
}

func (db *slShortcutJobs) Retry(ctx context.Context, job *SLShortcutJob, runAfter time.Time, jobErr ShortcutJobError) (bool, error) {
	return db.updateIfCurrent(ctx, job, map[string]interface{}{
		"status":    ShortcutJobPending,
		"run_after": runAfter,
		"locked_at": nil,
		"error":     datatypes.NewJSONType(jobErr),
	})
}

func (db *slShortcutJobs) Fail(ctx context.Context, job *SLShortcutJob, jobErr ShortcutJobError) (bool, error) {
	return db.updateIfCurrent(ctx, job, map[string]interface{}{
		"status":    ShortcutJobFailed,
		"locked_at": nil,
		"error":     datatypes.NewJSONType(jobErr),
	})
}

func (db *slShortcutJobs) Finish(ctx context.Context, job *SLShortcutJob) (bool, error) {
	result := db.WithContext(ctx).Where("id = ? AND token = ?", job.ID, job.Token).Delete(&SLShortcutJob{})
	if result.Error != nil {
		return false, errors.Wrap(result.Error, "delete")
	}
	return result.RowsAffected > 0, nil
}

func (db *slShortcutJobs) ListByTableID(ctx context.Context, tableID int64) ([]*SLShortcutJob, error) {
	var jobs []*SLShortcutJob
	if err := db.WithContext(ctx).Where("sl_table_id = ?", tableID).Order("id ASC").Find(&jobs).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return jobs, nil
}

func (db *slShortcutJobs) DeleteByField(ctx context.Context, tableID int64, fieldUID string) error {
	if err := db.WithContext(ctx).Where("sl_table_id = ? AND field_uid = ?", tableID, fieldUID).Delete(&SLShortcutJob{}).Error; err != nil {
		return errors.Wrap(err, "delete")
	}
	return nil
}

func (db *slShortcutJobs) DeleteByRecords(ctx context.Context, tableID int64, recordUIDs []string) error {
	if len(recordUIDs) == 0 {
		return nil
	}
	if err := db.WithContext(ctx).Where("sl_table_id = ? AND record_uid IN ?", tableID, recordUIDs).Delete(&SLShortcutJob{}).Error; err != nil {
		return errors.Wrap(err, "delete")
	}
	return nil
}

func (db *slShortcutJobs) ResetStale(ctx context.Context, before time.Time) (int64, error) {
	result := db.WithContext(ctx).Model(&SLShortcutJob{}).Where("status = ? AND locked_at < ?", ShortcutJobRunning, before).Updates(map[string]interface{}{
		"status":     ShortcutJobPending,
		"locked_at":  nil,
		"run_after":  dbutil.Now(),
		"updated_at": dbutil.Now(),
	})
	if result.Error != nil {
		return 0, errors.Wrap(result.Error, "update")
	}
	return result.RowsAffected, nil
}
