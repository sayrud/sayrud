package workflow

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cockroachdb/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
)

// PublishOptions returns the options for publishing a spec that passed the checks: the API entry, the watched table and the schedule state; the spec's path is normalized.
func PublishOptions(ctx context.Context, gormDB *gorm.DB, spec *Spec, draftVersion, userID int64, now time.Time) (db.PublishWorkflowOptions, error) {
	opts := db.PublishWorkflowOptions{DraftVersion: draftVersion, TriggerType: spec.Trigger.Type, UserID: userID}
	if spec.IsAPI() {
		spec.Trigger.API.Path, _ = NormalizePath(spec.Trigger.API.Path)
		opts.Method, opts.Path = spec.Trigger.API.Method, spec.Trigger.API.Path
	} else if spec.Trigger.Record != nil {
		table, err := db.NewSLTablesStore(gormDB).GetByUID(ctx, spec.Trigger.Record.TableUID)
		if err != nil {
			return opts, errors.Wrap(err, "get trigger table")
		}
		opts.TriggerTableID = table.ID
	}
	opts.Schedule = func(current *db.Workflow) db.WorkflowSchedule { return ScheduleState(spec, current, now) }
	raw, err := json.Marshal(spec)
	if err != nil {
		return opts, errors.Wrap(err, "encode spec")
	}
	opts.Spec = raw
	return opts, nil
}
