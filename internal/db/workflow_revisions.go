package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var _ WorkflowRevisionsStore = (*workflowRevisions)(nil)

// WorkflowRevisions is the default instance of the WorkflowRevisionsStore.
var WorkflowRevisions WorkflowRevisionsStore

// WorkflowRevisionsStore is the persistent interface for the immutable published revisions of workflows.
type WorkflowRevisionsStore interface {
	// GetByID returns the revision with the given ID, it returns ErrWorkflowRevisionNotFound if it does not exist.
	GetByID(ctx context.Context, revisionID int64) (*WorkflowRevision, error)
	// ListByWorkflowID returns the revisions of the workflow (without the spec), the newest first.
	ListByWorkflowID(ctx context.Context, workflowID int64, limit int) ([]*WorkflowRevision, error)
}

func NewWorkflowRevisionsStore(db *gorm.DB) WorkflowRevisionsStore {
	return &workflowRevisions{db}
}

// WorkflowRevision is an immutable published revision of a workflow, runs and jobs always use the revision at the time of triggering.
type WorkflowRevision struct {
	ID int64 `gorm:"primarykey"`
	// WorkflowID is the workflow the revision belongs to.
	WorkflowID int64 `gorm:"not null;uniqueIndex:idx_workflow_revisions_version"`
	// Version starts from 1 within each workflow.
	Version int `gorm:"not null;uniqueIndex:idx_workflow_revisions_version"`
	// TriggerType is the trigger type of the revision, Method and Path are the entry of the API trigger and are empty for other triggers.
	TriggerType string `gorm:"type:varchar(32);not null;default:''"`
	Method      string `gorm:"type:varchar(8);not null"`
	Path        string `gorm:"not null"`
	// Spec is the frozen workflow.Spec.
	Spec datatypes.JSON `gorm:"type:jsonb;not null"`
	// CreatedBy is the publisher, i.e. the identity the revision runs as.
	CreatedBy int64 `gorm:"not null;default:0"`
	CreatedAt time.Time
}

type workflowRevisions struct {
	*gorm.DB
}

var ErrWorkflowRevisionNotFound = errors.New("workflow revision does not exist")

func (db *workflowRevisions) GetByID(ctx context.Context, revisionID int64) (*WorkflowRevision, error) {
	var revision WorkflowRevision
	if err := db.WithContext(ctx).Where("id = ?", revisionID).First(&revision).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowRevisionNotFound
		}
		return nil, errors.Wrap(err, "get")
	}
	return &revision, nil
}

func (db *workflowRevisions) ListByWorkflowID(ctx context.Context, workflowID int64, limit int) ([]*WorkflowRevision, error) {
	var list []*WorkflowRevision
	if err := db.WithContext(ctx).Omit("spec").Where("workflow_id = ?", workflowID).Order("version DESC").Limit(limit).Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return list, nil
}
