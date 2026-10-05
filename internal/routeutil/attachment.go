package routeutil

import (
	"context"
	"reflect"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/wuhan005/sayrud/internal/db"
)

var ErrInvalidAttachment = errors.New("attachment does not belong to this project")

func checkAttachmentValue(value interface{}) bool {
	values, ok := value.([]interface{})
	if !ok || len(values) > db.MaxAttachmentsPerCell {
		return false
	}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		attachment, ok := value.(map[string]interface{})
		if !ok {
			return false
		}
		uid, ok := attachment["uid"].(string)
		if !ok || uid == "" || seen[uid] {
			return false
		}
		seen[uid] = true
	}
	return true
}

// ValidateAttachments verifies file ownership and replaces client-supplied descriptions with stored metadata.
// Project scope permits table duplication and undo to reuse uploaded files safely.
// changed reports canonicalization so collaborative clients can refresh their record values.
func (v *RecordValidator) ValidateAttachments(ctx context.Context, tx *gorm.DB, tableID int64, data map[string]interface{}) (changed bool, err error) {
	var project *db.Project
	for uid, value := range data {
		field := v.fields[uid]
		if field == nil || field.Type != db.AttachmentFieldType || !IsStoredValue(field, value) {
			continue
		}
		if project == nil {
			table, err := db.NewSLTablesStore(tx).GetByID(ctx, tableID)
			if err != nil {
				return false, errors.Wrap(err, "get attachment table")
			}
			project, err = db.NewProjectsStore(tx).GetByID(ctx, table.ProjectID)
			if err != nil {
				return false, errors.Wrap(err, "get attachment project")
			}
		}
		next, err := canonicalAttachmentValue(ctx, db.NewFilesStore(tx), project, value)
		if err != nil {
			return false, err
		}
		changed = changed || !reflect.DeepEqual(value, next)
		data[uid] = next
	}
	return changed, nil
}

func canonicalAttachmentValue(ctx context.Context, files db.FilesStore, project *db.Project, value interface{}) ([]interface{}, error) {
	if !checkAttachmentValue(value) {
		return nil, ErrFieldTypeMismatch
	}
	values := value.([]interface{})
	result := make([]interface{}, 0, len(values))
	for _, value := range values {
		uid := value.(map[string]interface{})["uid"].(string)
		file, err := files.GetByUID(ctx, uid)
		if err != nil {
			if errors.Is(err, db.ErrFileNotFound) {
				return nil, ErrInvalidAttachment
			}
			return nil, errors.Wrap(err, "get attachment")
		}
		if file.Category != "attachments" || file.ProjectID != project.ID {
			return nil, ErrInvalidAttachment
		}
		attachment := db.ToAttachment(file, project.UID)
		result = append(result, map[string]interface{}{
			"uid": attachment.UID, "name": attachment.Name, "size": float64(attachment.Size),
			"contentType": attachment.ContentType, "url": attachment.URL,
		})
	}
	return result, nil
}
