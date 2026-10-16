package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ WorkflowJobsStore = (*workflowJobs)(nil)

// WorkflowJobs is the default instance of the WorkflowJobsStore.
var WorkflowJobs WorkflowJobsStore

// WorkflowJobsStore is the persistent queue of workflow background jobs: the async flows and the main flows not triggered by API. Jobs run at least once and are kept as the run history after finishing.
type WorkflowJobsStore interface {
	// Create inserts the pending jobs, usually called in the transaction that commits the event.
	Create(ctx context.Context, jobs []*WorkflowJob) error
	// Claim claims the next due pending job, it returns nil if there is none.
	Claim(ctx context.Context) (*WorkflowJob, error)
	// Renew renews the lease only if the execution still holds the job.
	Renew(ctx context.Context, job *WorkflowJob) (bool, error)
	// Release returns an interrupted execution without consuming an attempt.
	Release(ctx context.Context, job *WorkflowJob) (bool, error)
	// SaveState records the outputs of the completed steps while the execution holds the job.
	SaveState(ctx context.Context, job *WorkflowJob, state, trace []byte) (bool, error)
	// Retry puts the job back to pending to run again after the given time.
	Retry(ctx context.Context, job *WorkflowJob, runAfter time.Time, message string, trace []byte) (bool, error)
	// Fail marks the job as failed, uncertain indicates the external action may have taken effect.
	Fail(ctx context.Context, job *WorkflowJob, message string, uncertain bool, trace []byte) (bool, error)
	// Succeed marks the job as succeeded.
	Succeed(ctx context.Context, job *WorkflowJob, trace []byte) (bool, error)
	// Requeue reruns a failed job of the workflow with fresh attempts, skipping the completed steps; it returns nil if the job does not exist or has not failed.
	Requeue(ctx context.Context, workflowID, jobID int64) (*WorkflowJob, error)
	// ListByRunIDs returns the jobs of the runs in creation order.
	ListByRunIDs(ctx context.Context, runIDs []int64) ([]*WorkflowJob, error)
	// ResetStale reclaims the leases older than maxAge by the database clock. Jobs that have used maxAttempts attempts fail as
	// uncertain since the interrupted attempt may have taken effect, the others go back to pending. It returns the number of
	// pending jobs and the failed jobs.
	ResetStale(ctx context.Context, maxAge time.Duration, maxAttempts int) (int64, []*WorkflowJob, error)
}

func NewWorkflowJobsStore(db *gorm.DB) WorkflowJobsStore {
	return &workflowJobs{db}
}

// WorkflowJobStatus is the status of a background job.
type WorkflowJobStatus string

const (
	WorkflowJobPending   WorkflowJobStatus = "pending"
	WorkflowJobRunning   WorkflowJobStatus = "running"
	WorkflowJobSucceeded WorkflowJobStatus = "succeeded"
	WorkflowJobFailed    WorkflowJobStatus = "failed"
)

// WorkflowJob is a background flow of a run: an async flow triggered by API, or the main flow of other triggers.
type WorkflowJob struct {
	ID int64 `gorm:"primarykey"`
	// WorkflowID is the workflow the job belongs to.
	WorkflowID int64 `gorm:"not null;index"`
	// RunID is the run that triggered the job.
	RunID int64 `gorm:"not null;index"`
	// RevisionID is the revision the flow belongs to.
	RevisionID int64 `gorm:"not null"`
	// FlowID is the ID of the async flow in the spec, or main for the main flow.
	FlowID string `gorm:"type:varchar(32);not null"`
	// Event is the triggering event: request_received or succeeded for async flows, or the trigger type for the main flow.
	Event string `gorm:"type:varchar(32);not null"`

	Status WorkflowJobStatus `gorm:"type:varchar(16);not null;index:idx_workflow_jobs_due,priority:1"`
	// Token is incremented on claiming and reclaiming, so only the current execution can update the job.
	Token    int64     `gorm:"not null;default:1"`
	Attempts int       `gorm:"not null;default:0"`
	RunAfter time.Time `gorm:"not null;index:idx_workflow_jobs_due,priority:2"`
	LockedAt *time.Time

	// Payload is the event snapshot read by the flow, such as the input, the triggering record, the step outputs and the response.
	Payload datatypes.JSON `gorm:"type:jsonb;not null"`
	// State is the outputs of the completed steps keyed by step ID, which are skipped on retries.
	State datatypes.JSON `gorm:"type:jsonb;not null"`
	// Trace is the results of the steps in the latest attempt.
	Trace datatypes.JSON `gorm:"type:jsonb;not null"`
	// Error is the reason of the latest failure.
	Error string `gorm:"not null;default:''"`
	// Uncertain indicates the failed external action may have taken effect, e.g. timing out waiting for confirmation after an email was sent.
	Uncertain bool `gorm:"not null;default:false"`

	CreatedAt  time.Time
	UpdatedAt  time.Time
	FinishedAt *time.Time
}

type workflowJobs struct {
	*gorm.DB
}

func (db *workflowJobs) Create(ctx context.Context, jobs []*WorkflowJob) error {
	if len(jobs) == 0 {
		return nil
	}
	now := dbutil.Now()
	for _, job := range jobs {
		job.Status = WorkflowJobPending
		job.Token = 1
		job.RunAfter = now
		for _, field := range []*datatypes.JSON{&job.Payload, &job.State, &job.Trace} {
			if len(*field) == 0 {
				*field = datatypes.JSON("{}")
			}
		}
	}
	return db.WithContext(ctx).Create(&jobs).Error
}

func (db *workflowJobs) Claim(ctx context.Context) (*WorkflowJob, error) {
	var jobs []*WorkflowJob
	if err := db.WithContext(ctx).Raw(`
UPDATE workflow_jobs SET status = ?, locked_at = CURRENT_TIMESTAMP, token = token + 1, attempts = attempts + 1, updated_at = CURRENT_TIMESTAMP
WHERE id = (
	SELECT id FROM workflow_jobs WHERE status = ? AND run_after <= CURRENT_TIMESTAMP
	ORDER BY run_after ASC, id ASC LIMIT 1 FOR UPDATE SKIP LOCKED
)
RETURNING *`, WorkflowJobRunning, WorkflowJobPending).Scan(&jobs).Error; err != nil {
		return nil, errors.Wrap(err, "claim")
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	return jobs[0], nil
}

func (db *workflowJobs) updateIfCurrent(ctx context.Context, job *WorkflowJob, values map[string]interface{}) (bool, error) {
	values["updated_at"] = dbutil.Now()
	result := db.WithContext(ctx).Model(&WorkflowJob{}).Where("id = ? AND token = ? AND status = ?", job.ID, job.Token, WorkflowJobRunning).Updates(values)
	if result.Error != nil {
		return false, errors.Wrap(result.Error, "update")
	}
	return result.RowsAffected > 0, nil
}

func (db *workflowJobs) Renew(ctx context.Context, job *WorkflowJob) (bool, error) {
	return db.updateIfCurrent(ctx, job, map[string]interface{}{"locked_at": gorm.Expr("CURRENT_TIMESTAMP")})
}

func (db *workflowJobs) Release(ctx context.Context, job *WorkflowJob) (bool, error) {
	return db.updateIfCurrent(ctx, job, map[string]interface{}{
		"status":    WorkflowJobPending,
		"locked_at": nil,
		"run_after": gorm.Expr("CURRENT_TIMESTAMP"),
		"token":     gorm.Expr("token + 1"),
		"attempts":  gorm.Expr("GREATEST(attempts - 1, 0)"),
	})
}

func (db *workflowJobs) SaveState(ctx context.Context, job *WorkflowJob, state, trace []byte) (bool, error) {
	return db.updateIfCurrent(ctx, job, map[string]interface{}{"state": datatypes.JSON(state), "trace": datatypes.JSON(trace)})
}

func (db *workflowJobs) Retry(ctx context.Context, job *WorkflowJob, runAfter time.Time, message string, trace []byte) (bool, error) {
	return db.updateIfCurrent(ctx, job, map[string]interface{}{
		"status":    WorkflowJobPending,
		"run_after": runAfter,
		"locked_at": nil,
		"error":     message,
		"trace":     datatypes.JSON(trace),
	})
}

func (db *workflowJobs) Fail(ctx context.Context, job *WorkflowJob, message string, uncertain bool, trace []byte) (bool, error) {
	return db.updateIfCurrent(ctx, job, map[string]interface{}{
		"status":      WorkflowJobFailed,
		"locked_at":   nil,
		"error":       message,
		"uncertain":   uncertain,
		"trace":       datatypes.JSON(trace),
		"finished_at": dbutil.Now(),
	})
}

func (db *workflowJobs) Succeed(ctx context.Context, job *WorkflowJob, trace []byte) (bool, error) {
	return db.updateIfCurrent(ctx, job, map[string]interface{}{
		"status":      WorkflowJobSucceeded,
		"locked_at":   nil,
		"error":       "",
		"uncertain":   false,
		"trace":       datatypes.JSON(trace),
		"finished_at": dbutil.Now(),
	})
}

func (db *workflowJobs) Requeue(ctx context.Context, workflowID, jobID int64) (*WorkflowJob, error) {
	var jobs []*WorkflowJob
	result := db.WithContext(ctx).Model(&jobs).Clauses(clause.Returning{}).Where("id = ? AND workflow_id = ? AND status = ?", jobID, workflowID, WorkflowJobFailed).Updates(map[string]interface{}{
		"status":      WorkflowJobPending,
		"attempts":    0,
		"run_after":   gorm.Expr("CURRENT_TIMESTAMP"),
		"token":       gorm.Expr("token + 1"),
		"error":       "",
		"uncertain":   false,
		"finished_at": nil,
		"updated_at":  dbutil.Now(),
	})
	if result.Error != nil {
		return nil, errors.Wrap(result.Error, "update")
	}
	if len(jobs) == 0 {
		return nil, nil
	}
	return jobs[0], nil
}

func (db *workflowJobs) ListByRunIDs(ctx context.Context, runIDs []int64) ([]*WorkflowJob, error) {
	var list []*WorkflowJob
	if len(runIDs) == 0 {
		return list, nil
	}
	if err := db.WithContext(ctx).Omit("payload", "state").Where("run_id IN ?", runIDs).Order("id ASC").Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return list, nil
}

// WorkflowJobInterruptedMessage is the error message of the jobs failed by ResetStale.
const WorkflowJobInterruptedMessage = "the job was interrupted repeatedly"

func (db *workflowJobs) ResetStale(ctx context.Context, maxAge time.Duration, maxAttempts int) (int64, []*WorkflowJob, error) {
	const stale = "status = ? AND locked_at < CURRENT_TIMESTAMP - (? * INTERVAL '1 microsecond')"
	var reset int64
	failed := []*WorkflowJob{}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := dbutil.Now()
		if err := tx.Model(&failed).Clauses(clause.Returning{}).Where(stale+" AND attempts >= ?", WorkflowJobRunning, maxAge.Microseconds(), maxAttempts).Updates(map[string]interface{}{
			"status":      WorkflowJobFailed,
			"locked_at":   nil,
			"error":       WorkflowJobInterruptedMessage,
			"uncertain":   true,
			"finished_at": now,
			"updated_at":  now,
			"token":       gorm.Expr("token + 1"),
		}).Error; err != nil {
			return errors.Wrap(err, "fail")
		}
		result := tx.Model(&WorkflowJob{}).Where(stale, WorkflowJobRunning, maxAge.Microseconds()).Updates(map[string]interface{}{
			"status":     WorkflowJobPending,
			"locked_at":  nil,
			"run_after":  gorm.Expr("CURRENT_TIMESTAMP"),
			"updated_at": now,
			"token":      gorm.Expr("token + 1"),
		})
		if result.Error != nil {
			return errors.Wrap(result.Error, "reset")
		}
		reset = result.RowsAffected
		return nil
	})
	if err != nil {
		return 0, nil, err
	}
	return reset, failed, nil
}
