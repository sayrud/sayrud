package routeutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

type attachmentFiles struct {
	db.FilesStore
	file *db.File
}

func (s attachmentFiles) GetByUID(_ context.Context, uid string) (*db.File, error) {
	if s.file != nil && s.file.UID == uid {
		return s.file, nil
	}
	return nil, db.ErrFileNotFound
}

func TestCanonicalAttachmentValue(t *testing.T) {
	project := &db.Project{Model: dbutil.Model{ID: 1}, UID: "prjOne"}
	file := &db.File{UID: "filOne", Category: "attachments", ProjectID: 1, Name: "photo.png", Size: 12, ContentType: "image/png"}
	value := []interface{}{map[string]interface{}{
		"uid": "filOne", "name": "forged.html", "size": -1.0, "contentType": "text/html", "url": "https://example.com/evil.svg",
	}}
	result, err := canonicalAttachmentValue(context.Background(), attachmentFiles{file: file}, project, value)
	require.NoError(t, err)
	require.Equal(t, []interface{}{map[string]interface{}{
		"uid": "filOne", "name": "photo.png", "size": 12.0, "contentType": "image/png", "url": "/_/projects/prjOne/attachments/filOne",
	}}, result)

	for _, tc := range []struct {
		name     string
		file     *db.File
		expected error
	}{
		{"missing", nil, ErrInvalidAttachment},
		{"other project", &db.File{UID: "filOne", Category: "attachments", ProjectID: 2}, ErrInvalidAttachment},
		{"avatar", &db.File{UID: "filOne", Category: "avatars", ProjectID: 1}, ErrInvalidAttachment},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := canonicalAttachmentValue(context.Background(), attachmentFiles{file: tc.file}, project, value)
			require.ErrorIs(t, err, tc.expected)
		})
	}

	field := &db.SLField{UID: "fldOne", Type: db.AttachmentFieldType}
	validator := NewRecordValidator([]*db.SLField{field})
	for _, invalid := range []interface{}{"filOne", []interface{}{"filOne"}, []interface{}{map[string]interface{}{}}, append(value, value...)} {
		_, err := validator.Validate(map[string]interface{}{"fldOne": invalid})
		require.ErrorIs(t, err, ErrFieldTypeMismatch)
	}
	tooMany := make([]interface{}, db.MaxAttachmentsPerCell+1)
	require.False(t, CheckValue(field, tooMany))
	cleared, err := validator.Validate(map[string]interface{}{"fldOne": []interface{}{}})
	require.NoError(t, err)
	require.Empty(t, cleared)
}
