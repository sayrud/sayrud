// Copyright 2024 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package db

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/wuhan005/sayrud/internal/dbutil"
)

var _ WorkflowsStore = (*workflows)(nil)

// Workflows is the default instance of the WorkflowsStore.
var Workflows WorkflowsStore

// WorkflowsStore is the persistent interface for project workflows.
type WorkflowsStore interface {
	// List returns the workflows of the project ordered by position.
	List(ctx context.Context, projectID int64) ([]*Workflow, error)
	// GetByUID returns the workflow with the given UID, it returns ErrWorkflowNotFound if it does not exist.
	GetByUID(ctx context.Context, workflowUID string) (*Workflow, error)
	// GetByID returns the workflow with the given ID, it returns ErrWorkflowNotFound if it does not exist.
	GetByID(ctx context.Context, workflowID int64) (*Workflow, error)
	// GetByRoute returns the published API workflow in the project matching the method and normalized path, it returns ErrWorkflowNotFound if none matches.
	GetByRoute(ctx context.Context, projectID int64, method, path string) (*Workflow, error)
	// ListByTriggerTable returns the published and enabled workflows triggered by the record changes of the table.
	ListByTriggerTable(ctx context.Context, tableID int64, triggerTypes []string) ([]*Workflow, error)
	// LockDue locks the published and enabled workflows that have reached NextRunAt in the transaction, skipping the ones locked by other instances.
	LockDue(ctx context.Context, now time.Time, limit int) ([]*Workflow, error)
	// SetSchedule updates the next processing time and the scanned-up-to time of the schedule and date triggers.
	SetSchedule(ctx context.Context, workflowID int64, nextRunAt, cursor *time.Time) error
	// Create creates the workflow as a draft.
	Create(ctx context.Context, opts CreateWorkflowOptions) (*Workflow, error)
	// UpdateDraft saves the draft and returns the new version only if the draft version is still expectedVersion, it returns ErrWorkflowVersionConflict if someone else has saved it.
	UpdateDraft(ctx context.Context, workflowID int64, expectedVersion int64, opts UpdateWorkflowDraftOptions) (int64, error)
	// SetEnabled enables or disables the published revision; if schedule is not nil, it also updates the next processing time and the scan position.
	SetEnabled(ctx context.Context, workflowID int64, enabled bool, schedule *WorkflowSchedule) error
	// Publish freezes the spec into a new immutable revision and updates the published pointer in a transaction, it returns ErrWorkflowRouteConflict if a published API in the project already uses the same method and path.
	// A rollback (RestoredFrom > 0) also overwrites the draft with the spec.
	Publish(ctx context.Context, workflowID int64, opts PublishWorkflowOptions) (*WorkflowRevision, error)
	// DeleteByID deletes the workflow, the revisions are kept for the running jobs.
	DeleteByID(ctx context.Context, workflowID int64) error
	// FindReference returns a workflow of the project whose published spec contains the identifier (e.g. a field or table UID), it returns ErrWorkflowNotFound if there is no reference.
	FindReference(ctx context.Context, projectID int64, identifier string) (*Workflow, error)
}

func NewWorkflowsStore(db *gorm.DB) WorkflowsStore {
	return &workflows{db}
}

// Workflow is a workflow of the project: the draft can be edited freely, and triggers are handled by the published immutable revision.
type Workflow struct {
	// Model contains the primary key and the creation, update and deletion times.
	dbutil.Model

	// UID is the public identifier of the workflow, "wfl" followed by 13 random characters.
	UID string `gorm:"uniqueIndex:idx_workflows_uid, where:deleted_at IS NULL"`
	// ProjectID is the project the workflow belongs to.
	ProjectID int64 `gorm:"not null;uniqueIndex:idx_workflows_route, where:deleted_at IS NULL AND path <> '';index:idx_workflows_project_position,priority:1"`
	// Name is the display name of the workflow.
	Name string `gorm:"not null;default:''"`

	// Position is the sidebar order shared by tables and workflows in the project.
	Position int `gorm:"not null;default:0;index:idx_workflows_project_position,priority:2"`

	// Description is the optional description.
	Description string `gorm:"not null;default:''"`

	// TriggerType is the trigger type of the published revision, empty before publishing.
	TriggerType string `gorm:"type:varchar(32);not null;default:''"`
	// Method and Path are the entry of the published API trigger, empty for other triggers.
	Method string `gorm:"type:varchar(8);not null;default:'';uniqueIndex:idx_workflows_route, where:deleted_at IS NULL AND path <> ''"`
	Path   string `gorm:"not null;default:'';uniqueIndex:idx_workflows_route, where:deleted_at IS NULL AND path <> ''"`
	// TriggerTableID is the table watched by the published record and date triggers, 0 for other triggers.
	TriggerTableID int64 `gorm:"not null;default:0;index:idx_workflows_trigger_table, where:deleted_at IS NULL AND trigger_table_id > 0"`
	// NextRunAt is the next run time of the schedule trigger or the next scan time of the date trigger, empty for other triggers.
	NextRunAt *time.Time `gorm:"index:idx_workflows_next_run, where:deleted_at IS NULL AND next_run_at IS NOT NULL"`
	// Cursor is the time the date trigger has scanned up to, only record times reached after it trigger.
	Cursor *time.Time
	// Enabled indicates whether the published revision responds to triggers.
	Enabled bool `gorm:"not null;default:true"`

	// Draft is the spec being edited, see workflow.Spec.
	Draft datatypes.JSON `gorm:"type:jsonb;not null"`
	// DraftVersion is incremented on every draft save to prevent concurrent editors from overwriting each other.
	DraftVersion int64 `gorm:"not null;default:1"`

	// PublishedRevisionID is the revision that handles triggers, 0 if unpublished.
	PublishedRevisionID int64 `gorm:"not null;default:0"`
	// PublishedVersion is the version number of the published revision.
	PublishedVersion int `gorm:"not null;default:0"`
	// PublishedDraftVersion is the draft version at publishing, used to tell whether the draft has unpublished changes.
	PublishedDraftVersion int64 `gorm:"not null;default:0"`
	PublishedAt           *time.Time

	CreatedBy int64 `gorm:"not null;default:0"`
	UpdatedBy int64 `gorm:"not null;default:0"`
}

func (w *Workflow) BeforeCreate(_ *gorm.DB) error {
	if w.UID == "" {
		w.UID = "wfl" + randstr.String(13)
	}
	return nil
}

type workflows struct {
	*gorm.DB
}

var (
	ErrWorkflowNotFound        = errors.New("workflow does not exist")
	ErrWorkflowVersionConflict = errors.New("workflow draft version conflict")
	ErrWorkflowRouteConflict   = errors.New("workflow route conflict")
)

func (db *workflows) List(ctx context.Context, projectID int64) ([]*Workflow, error) {
	var list []*Workflow
	if err := db.WithContext(ctx).Where("project_id = ?", projectID).Order("position ASC, id DESC").Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return list, nil
}

func (db *workflows) getBy(ctx context.Context, where string, args ...interface{}) (*Workflow, error) {
	var w Workflow
	if err := db.WithContext(ctx).Where(where, args...).First(&w).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, errors.Wrap(err, "get")
	}
	return &w, nil
}

func (db *workflows) GetByUID(ctx context.Context, workflowUID string) (*Workflow, error) {
	return db.getBy(ctx, "uid = ?", workflowUID)
}

func (db *workflows) GetByID(ctx context.Context, workflowID int64) (*Workflow, error) {
	return db.getBy(ctx, "id = ?", workflowID)
}

func (db *workflows) GetByRoute(ctx context.Context, projectID int64, method, path string) (*Workflow, error) {
	return db.getBy(ctx, "project_id = ? AND method = ? AND path = ? AND published_revision_id > 0", projectID, method, path)
}

func (db *workflows) ListByTriggerTable(ctx context.Context, tableID int64, triggerTypes []string) ([]*Workflow, error) {
	var list []*Workflow
	if err := db.WithContext(ctx).Omit("draft").
		Where("trigger_table_id = ? AND trigger_type IN ? AND enabled AND published_revision_id > 0", tableID, triggerTypes).
		Order("id ASC").Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "find")
	}
	return list, nil
}

func (db *workflows) LockDue(ctx context.Context, now time.Time, limit int) ([]*Workflow, error) {
	var list []*Workflow
	if err := db.WithContext(ctx).Omit("draft").Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("next_run_at <= ? AND enabled AND published_revision_id > 0", now).
		Order("next_run_at ASC").Limit(limit).Find(&list).Error; err != nil {
		return nil, errors.Wrap(err, "lock")
	}
	return list, nil
}

func (db *workflows) SetSchedule(ctx context.Context, workflowID int64, nextRunAt, cursor *time.Time) error {
	return db.WithContext(ctx).Model(&Workflow{}).Where("id = ?", workflowID).
		Updates(map[string]interface{}{"next_run_at": nextRunAt, "cursor": cursor}).Error
}

// CreateWorkflowOptions are the options of creating a workflow.
type CreateWorkflowOptions struct {
	ProjectID   int64
	Name        string
	Description string
	Draft       []byte
	UserID      int64
}

func (db *workflows) Create(ctx context.Context, opts CreateWorkflowOptions) (*Workflow, error) {
	w := &Workflow{
		ProjectID:    opts.ProjectID,
		Name:         opts.Name,
		Description:  opts.Description,
		Enabled:      true,
		Draft:        datatypes.JSON(opts.Draft),
		DraftVersion: 1,
		CreatedBy:    opts.UserID,
		UpdatedBy:    opts.UserID,
	}

	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		position, err := nextProjectNavigationPosition(ctx, tx, opts.ProjectID)
		if err != nil {
			return err
		}

		w.Position = position
		return tx.Create(w).Error
	}); err != nil {
		return nil, errors.Wrap(err, "create")
	}

	return w, nil
}

// UpdateWorkflowDraftOptions are the options of saving a draft, all fields are overwritten as a whole.
type UpdateWorkflowDraftOptions struct {
	Name        string
	Description string
	Draft       []byte
	UserID      int64
}

func (db *workflows) UpdateDraft(ctx context.Context, workflowID int64, expectedVersion int64, opts UpdateWorkflowDraftOptions) (int64, error) {
	result := db.WithContext(ctx).Model(&Workflow{}).Where("id = ? AND draft_version = ?", workflowID, expectedVersion).Updates(map[string]interface{}{
		"name":          opts.Name,
		"description":   opts.Description,
		"draft":         datatypes.JSON(opts.Draft),
		"draft_version": gorm.Expr("draft_version + 1"),
		"updated_by":    opts.UserID,
	})
	if result.Error != nil {
		return 0, errors.Wrap(result.Error, "update")
	}
	if result.RowsAffected == 0 {
		return 0, ErrWorkflowVersionConflict
	}
	return expectedVersion + 1, nil
}

// WorkflowSchedule is the scheduling state of the schedule and date triggers.
type WorkflowSchedule struct {
	NextRunAt *time.Time
	Cursor    *time.Time
}

func (db *workflows) SetEnabled(ctx context.Context, workflowID int64, enabled bool, schedule *WorkflowSchedule) error {
	values := map[string]interface{}{"enabled": enabled}
	if schedule != nil {
		values["next_run_at"], values["cursor"] = schedule.NextRunAt, schedule.Cursor
	}
	return db.WithContext(ctx).Model(&Workflow{}).Where("id = ?", workflowID).Updates(values).Error
}

// PublishWorkflowOptions are the options of publishing a workflow.
type PublishWorkflowOptions struct {
	// DraftVersion is the draft version to publish, ErrWorkflowVersionConflict is returned if the draft has changed.
	DraftVersion int64
	TriggerType  string
	// Method and Path are the entry of the API trigger, empty for other triggers.
	Method string
	Path   string
	// TriggerTableID is the table watched by the record and date triggers.
	TriggerTableID int64
	// Schedule returns the scheduling state of the schedule and date triggers given the currently published workflow; the zero value means no scheduling.
	Schedule func(current *Workflow) WorkflowSchedule
	Spec     []byte
	// RestoredFrom is the version being rolled back to; when set, the draft is replaced by Spec in the same transaction.
	RestoredFrom int
	UserID       int64
}

func (db *workflows) Publish(ctx context.Context, workflowID int64, opts PublishWorkflowOptions) (*WorkflowRevision, error) {
	var revision *WorkflowRevision
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var w Workflow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", workflowID).First(&w).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrWorkflowNotFound
			}
			return errors.Wrap(err, "lock workflow")
		}
		if w.DraftVersion != opts.DraftVersion {
			return ErrWorkflowVersionConflict
		}

		if opts.Path != "" {
			var conflict int64
			if err := tx.Model(&Workflow{}).Where("project_id = ? AND method = ? AND path = ? AND id <> ?", w.ProjectID, opts.Method, opts.Path, w.ID).Count(&conflict).Error; err != nil {
				return errors.Wrap(err, "check route")
			}
			if conflict > 0 {
				return ErrWorkflowRouteConflict
			}
		}

		revision = &WorkflowRevision{
			WorkflowID:   w.ID,
			Version:      w.PublishedVersion + 1,
			TriggerType:  opts.TriggerType,
			Method:       opts.Method,
			Path:         opts.Path,
			Spec:         datatypes.JSON(opts.Spec),
			RestoredFrom: opts.RestoredFrom,
			CreatedBy:    opts.UserID,
		}
		if err := tx.Create(revision).Error; err != nil {
			return errors.Wrap(err, "create revision")
		}

		var schedule WorkflowSchedule
		if opts.Schedule != nil {
			schedule = opts.Schedule(&w)
		}
		draftVersion := opts.DraftVersion
		values := map[string]interface{}{
			"trigger_type":          opts.TriggerType,
			"method":                opts.Method,
			"path":                  opts.Path,
			"trigger_table_id":      opts.TriggerTableID,
			"next_run_at":           schedule.NextRunAt,
			"cursor":                schedule.Cursor,
			"published_revision_id": revision.ID,
			"published_version":     revision.Version,
			"published_at":          dbutil.Now(),
			"updated_by":            opts.UserID,
		}
		if opts.RestoredFrom > 0 {
			draftVersion++
			values["draft"], values["draft_version"] = datatypes.JSON(opts.Spec), draftVersion
		}
		values["published_draft_version"] = draftVersion
		if err := tx.Model(&Workflow{}).Where("id = ?", w.ID).Updates(values).Error; err != nil {
			if dbutil.IsUniqueViolation(err, "idx_workflows_route") {
				return ErrWorkflowRouteConflict
			}
			return errors.Wrap(err, "update workflow")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return revision, nil
}

func (db *workflows) DeleteByID(ctx context.Context, workflowID int64) error {
	return db.WithContext(ctx).Where("id = ?", workflowID).Delete(&Workflow{}).Error
}

func (db *workflows) FindReference(ctx context.Context, projectID int64, identifier string) (*Workflow, error) {
	var w Workflow
	err := db.WithContext(ctx).Model(&Workflow{}).
		Joins("JOIN workflow_revisions ON workflow_revisions.id = workflows.published_revision_id").
		Where("workflows.project_id = ? AND strpos(workflow_revisions.spec::text, ?) > 0", projectID, identifier).
		Order("workflows.id ASC").First(&w).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWorkflowNotFound
		}
		return nil, errors.Wrap(err, "find reference")
	}
	return &w, nil
}
