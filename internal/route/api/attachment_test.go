package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flamego/flamego"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/i18n"
	"github.com/wuhan005/sayrud/internal/storage"
)

func attachmentTestRouter(t *testing.T) (*flamego.Flame, *db.Project, *db.SLField, *avatarFilesStore) {
	t.Helper()
	previousFiles, previousStorage := db.Files, storage.Default
	t.Cleanup(func() { db.Files, storage.Default = previousFiles, previousStorage })
	files := &avatarFilesStore{files: make(map[string]*db.File)}
	objects, err := storage.NewLocalStorage(t.TempDir())
	require.NoError(t, err)
	db.Files, storage.Default = files, objects
	project := &db.Project{Model: dbutil.Model{ID: 1}, UID: "prjOne"}
	field := &db.SLField{Model: dbutil.Model{ID: 2}, UID: "fldOne", Type: db.AttachmentFieldType,
		Metadata: datatypes.NewJSONType[db.SLFieldMetadata](map[string]interface{}{})}
	f := flamego.New()
	f.Use(i18n.Middleware(), context.Contexter(nil))
	f.Map(project, field, &db.User{Model: dbutil.Model{ID: 3}}, db.ProjectRoleEditor)
	f.Post("/upload", Project.RequireRole(db.ProjectRoleEditor), Schemaless.LimitAttachmentUpload, Schemaless.UploadAttachment)
	f.Get("/files/{fileUID}", Schemaless.Attachment)
	return f, project, field, files
}

func attachmentRequest(t *testing.T, router *flamego.Flame, content []byte, name, userAgent string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", name)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	r := httptest.NewRequest(http.MethodPost, "/upload", &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.Header.Set("User-Agent", userAgent)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, r)
	return response
}

func TestAttachmentUploadReadAndPermissions(t *testing.T) {
	router, project, field, files := attachmentTestRouter(t)
	content := avatarImage(t, "png", 2, 2)
	response := attachmentRequest(t, router, content, "photo.png", "Desktop")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var body struct {
		Data db.Attachment `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	attachment := body.Data
	require.Equal(t, "photo.png", attachment.Name)
	require.Equal(t, int64(len(content)), attachment.Size)
	require.Equal(t, "image/png", attachment.ContentType)
	require.Equal(t, "/_/projects/prjOne/attachments/"+attachment.UID, attachment.URL)
	file := files.files[attachment.UID]
	require.Equal(t, project.ID, file.ProjectID)
	require.Equal(t, "attachments", file.Category)

	// Removing the source field does not invalidate files reused by undo or table copies.
	field.Type = db.TextFieldType
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/files/"+file.UID, nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, content, response.Body.Bytes())
	require.Contains(t, response.Header().Get("Content-Disposition"), "inline")
	require.Equal(t, "nosniff", response.Header().Get("X-Content-Type-Options"))
	require.Contains(t, response.Header().Get("Content-Security-Policy"), "sandbox")
	require.Equal(t, "private, no-cache", response.Header().Get("Cache-Control"))

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/files/"+file.UID+"?download=1", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Header().Get("Content-Disposition"), "attachment")

	response = attachmentRequest(t, router, content, "photo.png", "Desktop")
	require.Equal(t, http.StatusBadRequest, response.Code)
	field.Type = db.AttachmentFieldType
	router.Map(db.ProjectRoleViewer)
	response = attachmentRequest(t, router, content, "photo.png", "Desktop")
	require.Equal(t, http.StatusForbidden, response.Code)
	require.Len(t, files.files, 1)

	project.ID = 42
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/files/"+file.UID, nil))
	require.Equal(t, http.StatusNotFound, response.Code)
	project.ID = file.ProjectID
	file.Category = "avatars"
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/files/"+file.UID, nil))
	require.Equal(t, http.StatusNotFound, response.Code)
}

func TestAttachmentUploadLimitsAndLegacyMetadata(t *testing.T) {
	router, _, field, files := attachmentTestRouter(t)
	response := attachmentRequest(t, router, bytes.Repeat([]byte("x"), AttachmentMaxSize+1), "large.txt", "Desktop")
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	require.Empty(t, files.files)

	field.Metadata = datatypes.NewJSONType[db.SLFieldMetadata](map[string]interface{}{"mobile_only": true})
	// Legacy metadata must not prevent desktop uploads or restrict the uploaded file type.
	response = attachmentRequest(t, router, []byte("document"), "document.txt", "Desktop")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Len(t, files.files, 1)
}
