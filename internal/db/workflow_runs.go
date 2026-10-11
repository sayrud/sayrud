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

var _ WorkflowRunsStore = (*workflowRuns)(nil)

// WorkflowRuns is the default instance of the WorkflowRunsStore.
var WorkflowRuns WorkflowRunsStore

// WorkflowRunsStore is the persistent interface for workflow runs.
type WorkflowRunsStore interface {
	// Create inserts the runs.
	Create(ctx context.Context, runs ...*WorkflowRun) error
	// Start marks the queued run as running.
	Start(ctx context.Context, runID int64) error
	// Requeue puts a background run being rerun back to queued.
	Requeue(ctx context.Context, runID int64) error
	// Finish records the result of the run.
	Finish(ctx context.Context, runID int64, opts FinishWorkflowRunOptions) error
	// GetByUID returns the run of the workflow with the given UID, it returns ErrWorkflowRunNotFound if it does not exist.
	GetByUID(ctx context.Context, workflowID int64, runUID string) (*WorkflowRun, error)
	// List returns the runs of the workflow (without the request, trigger data and trace), the newest first.
	List(ctx context.Context, workflowID int64, opts ListWorkflowRunsOptions) ([]*WorkflowRun, error)
	// DeleteBefore deletes the runs created before the given time and their finished jobs, and returns the number of deleted runs.
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

func NewWorkflowRunsStore(db *gorm.DB) WorkflowRunsStore {
	return &workflowRuns{db}
}

// WorkflowRunStatus is the status of a run.
type WorkflowRunStatus string

const (
	// WorkflowRunQueued is a run waiting for background execution.
	WorkflowRunQueued    WorkflowRunStatus = "queued"
	WorkflowRunRunning   WorkflowRunStatus = "running"
	WorkflowRunSucceeded WorkflowRunStatus = "succeeded"
	WorkflowRunFailed    WorkflowRunStatus = "failed"
	WorkflowRunRejected  WorkflowRunStatus = "rejected"
)

// WorkflowRunTrigger is the source of a run: http, debug, or the trigger type of a background trigger, such as record_created or schedule.
type WorkflowRunTrigger string

const (
	WorkflowRunTriggerHTTP  WorkflowRunTrigger = "http"
	WorkflowRunTriggerDebug WorkflowRunTrigger = "debug"
)

// WorkflowRun is the run of a trigger, including the trigger data, the step trace and the API response.
type WorkflowRun struct {
	ID int64 `gorm:"primarykey"`
	// UID is the run ID, returned in the X-Request-Id response header when triggered by API.
	UID string `gorm:"type:varchar(24);not null;uniqueIndex:idx_workflow_runs_uid"`
	// WorkflowID is the workflow the run belongs to.
	WorkflowID int64 `gorm:"not null;index:idx_workflow_runs_workflow,priority:1"`
	// RevisionID and Version are the revision that handled the trigger.
	RevisionID int64 `gorm:"not null"`
	Version    int   `gorm:"not null;default:0"`

	Trigger    WorkflowRunTrigger `gorm:"type:varchar(32);not null"`
	Status     WorkflowRunStatus  `gorm:"type:varchar(16);not null"`
	StatusCode int                `gorm:"not null;default:0"`
	DurationMs int64              `gorm:"not null;default:0"`
	// IP is the address of the API caller.
	IP string `gorm:"not null;default:''"`
	// UserID is the member who started the debug run, 0 for other sources.
	UserID int64 `gorm:"not null;default:0"`

	// Request is the bounded snapshot of the API request without credentials.
	Request datatypes.JSON `gorm:"type:jsonb;not null"`
	// Event is the trigger data of a background trigger, such as the triggering record, referenced by the trigger variable.
	Event datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"`
	// Response is the status code, headers and body returned by the API.
	Response datatypes.JSON `gorm:"type:jsonb;not null"`
	// Trace is the results of the steps.
	Trace datatypes.JSON `gorm:"type:jsonb;not null"`
	// Error is the reason of the failure or rejection.
	Error datatypes.JSON `gorm:"type:jsonb;not null"`

	CreatedAt  time.Time `gorm:"index:idx_workflow_runs_workflow,priority:2"`
	FinishedAt *time.Time
}

func (run *WorkflowRun) BeforeCreate(_ *gorm.DB) error {
	if run.UID == "" {
		run.UID = "run" + randstr.String(16)
	}
	for _, field := range []*datatypes.JSON{&run.Request, &run.Event, &run.Response, &run.Trace, &run.Error} {
		if len(*field) == 0 {
			*field = datatypes.JSON("{}")
		}
	}
	return nil
}

type workflowRuns struct {
	*gorm.DB
}

var ErrWorkflowRunNotFound = errors.New("workflow run does not exist")

func (db *workflowRuns) Create(ctx context.Context, runs ...*WorkflowRun) error {
	if len(runs) == 0 {
		return nil
	}
	return errors.Wrap(db.WithContext(ctx).Create(runs).Error, "create")
}

func (db *workflowRuns) Start(ctx context.Context, runID int64) error {
	return db.WithContext(ctx).Model(&WorkflowRun{}).Where("id = ? AND status = ?", runID, WorkflowRunQueued).Update("status", WorkflowRunRunning).Error
}

func (db *workflowRuns) Requeue(ctx context.Context, runID int64) error {
	return db.WithContext(ctx).Model(&WorkflowRun{}).Where("id = ?", runID).
		Updates(map[string]interface{}{"status": WorkflowRunQueued, "finished_at": nil}).Error
}

// FinishWorkflowRunOptions is the result of a run.
type FinishWorkflowRunOptions struct {
	Status     WorkflowRunStatus
	StatusCode int
	DurationMs int64
	Response   []byte
	Trace      []byte
	Error      []byte
}

func (db *workflowRuns) Finish(ctx context.Context, runID int64, opts FinishWorkflowRunOptions) error {
	values := map[string]interface{}{
		"status":      opts.Status,
		"status_code": opts.StatusCode,
		"duration_ms": opts.DurationMs,
		"finished_at": dbutil.Now(),
	}
	for column, value := range map[string][]byte{"response": opts.Response, "trace": opts.Trace, "error": opts.Error} {
		if len(value) > 0 {
			values[column] = datatypes.JSON(value)
		}
	}
	return db.WithContext(ctx).Model(&WorkflowRun{}).Where("id = ?", runID).Updates(values).Error
}

func (db *workflowRuns) GetByUID(ctx context.Context, workflowID int64, runUID string) (*WorkflowRun, error) {
	var run WorkflowRun
	if err := db.WithContext(ctx).Where("workflow_id = ? AND uid = ?", workflowID, runUID).First(&run).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowRunNotFound
		}
		return nil, errors.Wrap(err, "get")
	}
	return &run, nil
}

// ListWorkflowRunsOptions are the options of listing runs.
type ListWorkflowRunsOptions struct {
	// Status filters by status if not empty.
	Status WorkflowRunStatus
	// BeforeID returns the earlier runs, used for pagination.
	BeforeID int64
	// Limit defaults to 50.
	Limit int
}

func (db *workflowRuns) List(ctx context.Context, workflowID int64, opts ListWorkflowRunsOptions) ([]*WorkflowRun, error) {
	q := db.WithContext(ctx).Omit("request", "event", "trace").Where("workflow_id = ?", workflowID)
	if opts.Status != "" {
		q = q.Where("status = ?", opts.Status)
	}
	if opts.BeforeID > 0 {
		q = q.Where("id < ?", opts.BeforeID)
	}
	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var list []*WorkflowRun
	if err := q.Order("id DESC").Limit(limit).Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return list, nil
}

func (db *workflowRuns) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	var deleted int64
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		stale := tx.Model(&WorkflowRun{}).Select("id").Where("created_at < ? AND status NOT IN ?", before, []WorkflowRunStatus{WorkflowRunQueued, WorkflowRunRunning})
		if err := tx.Where("run_id IN (?) AND status IN ?", stale, []WorkflowJobStatus{WorkflowJobSucceeded, WorkflowJobFailed}).Delete(&WorkflowJob{}).Error; err != nil {
			return errors.Wrap(err, "delete jobs")
		}
		// Runs with unfinished jobs are kept until the jobs finish.
		result := tx.Where("created_at < ? AND status NOT IN ? AND NOT EXISTS (SELECT 1 FROM workflow_jobs WHERE workflow_jobs.run_id = workflow_runs.id)", before, []WorkflowRunStatus{WorkflowRunQueued, WorkflowRunRunning}).Delete(&WorkflowRun{})
		if result.Error != nil {
			return errors.Wrap(result.Error, "delete runs")
		}
		deleted = result.RowsAffected
		return nil
	})
	return deleted, err
}
