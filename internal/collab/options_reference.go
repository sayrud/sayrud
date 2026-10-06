package collab

import (
	"context"
	"reflect"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
)

// Refresh dependent metadata through normal server commits, outside the source table lock.
func (h *Hub) notifyOptionReferences(ctx context.Context, project *db.Project, table *db.SLTable, operations []Operation) {
	h.runBackground(ctx, func(ctx context.Context) {
		if err := h.refreshOptionReferences(ctx, project, table, operations); err != nil {
			logrus.WithError(err).Error("Failed to refresh referenced options")
		}
	})
}

func (h *Hub) refreshOptionReferences(ctx context.Context, project *db.Project, sourceTable *db.SLTable, operations []Operation) error {
	var fields []*db.SLField
	if err := h.db.WithContext(ctx).Model(&db.SLField{}).
		Joins("JOIN sl_tables ON sl_tables.id = sl_fields.sl_table_id AND sl_tables.deleted_at IS NULL").
		Where("sl_tables.project_id = ? AND sl_fields.metadata -> 'optionsReference' ->> 'tableUID' = ?", project.ID, sourceTable.UID).
		Find(&fields).Error; err != nil {
		return err
	}

	for _, field := range fields {
		md, _ := field.Metadata.Data().(map[string]interface{})
		ref, _ := db.ReadOptionReference(md)
		if ref == nil {
			continue
		}

		relevant := map[string]bool{ref.FieldUID: true}
		conditionFields := map[string]bool{}
		for _, c := range ref.Conditions {
			relevant[c.FieldUID] = true
			conditionFields[c.FieldUID] = true
		}

		changed, records := false, false
		conditionsChanged := false
		for _, op := range operations {
			for _, a := range op.Actions {
				if relevant[a.FieldUID] {
					changed = true
				}

				conditionsChanged = conditionsChanged || conditionFields[a.FieldUID]
				if len(ref.Conditions) > 0 {
					if a.Action == ActionDeleteRecords {
						records = true
					}
					for uid := range a.Values {
						if relevant[uid] {
							records = true
						}
					}
				}

				if a.Dirty != nil {
					changed = changed || a.Dirty.Fields
					conditionsChanged = conditionsChanged || a.Dirty.Fields && len(ref.Conditions) > 0
					records = records || len(ref.Conditions) > 0 && (a.Dirty.AllRecords || len(a.Dirty.Records) > 0)
				}
			}
		}

		if !changed && !records {
			continue
		}

		table, err := db.NewSLTablesStore(h.db).GetByID(ctx, field.SLTableID)
		if err != nil {
			return err
		}

		attrs := &FieldAttrs{}
		_, err = h.CommitServer(ctx, project, table, []Operation{{Command: "SyncOptions", Actions: []Action{{Action: ActionSetField, FieldUID: field.UID, Field: attrs}}}}, func(tx *gorm.DB) (bool, error) {
			current, err := db.NewSLFieldsStore(tx).GetByID(ctx, field.ID)
			if err != nil {
				if errors.Is(err, db.ErrSLFieldNotFound) {
					return false, nil
				}
				return false, err
			}

			metadata, _ := current.Metadata.Data().(map[string]interface{})
			r, _ := db.ReadOptionReference(metadata)
			if r == nil || r.TableUID != ref.TableUID || r.FieldUID != ref.FieldUID {
				return false, nil
			}

			old := metadata["options"]
			if err := db.PrepareSelectMetadata(ctx, tx, table.ID, current.UID, current.Type, metadata); err != nil {
				// Retain displayed values when the source is deleted or no longer selectable. Resolving choices reports the unavailable source.
				if errors.Is(err, db.ErrOptionReference) {
					return false, nil
				}
				return false, err
			}

			if reflect.DeepEqual(old, metadata["options"]) {
				return false, nil
			}

			attrs.Metadata = metadata
			return true, nil
		})
		if err != nil {
			return err
		}

		if (conditionsChanged || records) && field.Shortcut != nil && field.Shortcut.AutoUpdate {
			h.NotifyChange(ctx, project, table, &Change{Shortcuts: []string{field.UID}})
		}
	}

	return nil
}
