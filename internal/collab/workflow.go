package collab

import (
	"context"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
)

// The kinds of record changes.
const (
	RecordCreated = "created"
	RecordUpdated = "updated"
	RecordDeleted = "deleted"
)

// RecordEvent is the creation, update or deletion of a record in a write transaction.
type RecordEvent struct {
	Kind string
	// Record is the record after the write, or the record before the deletion when deleted.
	Record *db.SLRecord
	// Previous is the values before the update, only set for updated.
	Previous datatypes.JSON
	// Fields are the UIDs of the fields whose values changed, only set for updated.
	Fields []string
}

// Origin is the source of a write: it is the zero value for user writes, and workflow writes carry the workflow ID and execution depth to avoid cyclic triggers.
type Origin struct {
	WorkflowID int64
	Depth      int
}

// WorkflowHooks lets workflows enqueue the runs triggered by record changes within the transaction that writes the records.
type WorkflowHooks interface {
	// RecordsChanged enqueues runs for the record changes in the write transaction and returns the number enqueued; returning an error rolls back the write.
	RecordsChanged(ctx context.Context, tx *gorm.DB, project *db.Project, table *db.SLTable, origin Origin, events []RecordEvent) (int, error)
	// RunsQueued is called after a transaction that enqueued runs commits.
	RunsQueued()
}

// SetWorkflowHooks connects the workflows, it must be called before serving.
func (h *Hub) SetWorkflowHooks(hooks WorkflowHooks) {
	h.workflows = hooks
}

// recordsChanged hands the record changes in the transaction to the workflows and reports whether any runs were enqueued; updates without changes are ignored.
func (h *Hub) recordsChanged(ctx context.Context, tx *gorm.DB, project *db.Project, table *db.SLTable, origin Origin, events []RecordEvent) (bool, error) {
	if h.workflows == nil || project == nil {
		return false, nil
	}
	kept := events[:0:0]
	for _, ev := range events {
		if ev.Kind != RecordUpdated || len(ev.Fields) > 0 {
			kept = append(kept, ev)
		}
	}
	if len(kept) == 0 {
		return false, nil
	}
	n, err := h.workflows.RecordsChanged(ctx, tx, project, table, origin, kept)
	return n > 0, err
}

// runsQueued wakes up the workflow workers after committing.
func (h *Hub) runsQueued(queued bool) {
	if queued && h.workflows != nil {
		h.workflows.RunsQueued()
	}
}

// MutateRecords is the same as Mutate, and the record changes returned by mutate are handed to the workflows to enqueue triggers in the same transaction.
func (h *Hub) MutateRecords(ctx context.Context, project *db.Project, table *db.SLTable, mutate func(tx *gorm.DB) ([]RecordEvent, error), scope func() DirtyScope) error {
	queued := false
	err := h.Mutate(ctx, project, table, func(tx *gorm.DB) error {
		events, err := mutate(tx)
		if err != nil {
			return err
		}
		queued, err = h.recordsChanged(ctx, tx, project, table, Origin{}, events)
		return err
	}, scope)
	if err != nil {
		return err
	}
	h.runsQueued(queued)
	return nil
}
