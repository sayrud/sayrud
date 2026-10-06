package api

import (
	"io/fs"
	"mime"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/storage"
)

// AttachmentMaxSize is the maximum number of bytes in a single uploaded attachment.
const AttachmentMaxSize = 20 << 20

// LimitAttachmentUpload bounds the request body and removes multipart temporary files.
func (schemalessRoute) LimitAttachmentUpload(ctx context.Context) {
	r := ctx.Request().Request
	r.Body = http.MaxBytesReader(ctx.ResponseWriter(), r.Body, AttachmentMaxSize+(64<<10))
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	ctx.Next()
}

// UploadAttachment
// @Summary Upload an attachment
// @Description Upload a single file up to 20 MiB for an attachment field. The returned descriptor can be used in record values within this project.
// @Accept multipart/form-data
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param tableUID path string true "Table UID"
// @Param fieldUID path string true "Attachment field UID"
// @Param file formData file true "Attachment file"
// @Success 200 {object} db.Attachment
// @Failure 400 {string} string "Invalid multipart body or field type"
// @Failure 401 {string} string "Not signed in"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, table or field not found"
// @Failure 413 {string} string "File is too large"
// @Failure 500 {string} string "Internal server error"
// @ID uploadAttachment
// @Router /projects/{projectUID}/tables/{tableUID}/fields/{fieldUID}/attachments [post]
func (schemalessRoute) UploadAttachment(ctx context.Context, user *db.User, project *db.Project, field *db.SLField) error {
	if field.Type != db.AttachmentFieldType || field.Shortcut != nil {
		return ctx.ApiError(http.StatusBadRequest, "attachment::invalid_field")
	}

	r := ctx.Request().Request
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return ctx.ApiError(http.StatusRequestEntityTooLarge, "attachment::too_large")
		}
		return ctx.ApiError(http.StatusBadRequest, "common::invalid_body")
	}

	uploaded, header, err := r.FormFile("file")
	if err != nil {
		return ctx.ApiError(http.StatusBadRequest, "common::invalid_body")
	}
	defer func() { _ = uploaded.Close() }()
	if header.Size > AttachmentMaxSize {
		return ctx.ApiError(http.StatusRequestEntityTooLarge, "attachment::too_large")
	}
	if len(r.MultipartForm.File["file"]) != 1 {
		return ctx.ApiError(http.StatusBadRequest, "common::invalid_body")
	}

	file, err := storage.Upload(r.Context(), storage.UploadOptions{
		Category: storage.CategoryAttachment, UserID: user.ID, ProjectID: project.ID,
		Name: header.Filename, Reader: uploaded, Size: header.Size,
	})
	if err != nil {
		logrus.WithContext(r.Context()).WithError(err).Error("Failed to upload attachment")
		return ctx.ApiServerError()
	}
	return ctx.ApiSuccess(db.ToAttachment(file, project.UID))
}

// Attachment
// @Summary Read an attachment
// @Description Read a file uploaded in this project. Project membership is checked on every request.
// @Param projectUID path string true "Project UID"
// @Param fileUID path string true "Attachment UID"
// @Param download query boolean false "Force file download"
// @Success 200 {file} file
// @Failure 401 {string} string "Not signed in"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or attachment not found"
// @Failure 500 {string} string "Internal server error"
// @ID getAttachment
// @Router /projects/{projectUID}/attachments/{fileUID} [get]
func (schemalessRoute) Attachment(ctx context.Context, project *db.Project) error {
	c := ctx.Request().Context()
	file, err := db.Files.GetByUID(c, ctx.Param("fileUID"))
	if err != nil {
		if errors.Is(err, db.ErrFileNotFound) {
			return ctx.ApiError(http.StatusNotFound, "attachment::not_found")
		}
		logrus.WithContext(c).WithError(err).Error("Failed to get attachment")
		return ctx.ApiServerError()
	}
	if file.Category != string(storage.CategoryAttachment) || file.ProjectID != project.ID {
		return ctx.ApiError(http.StatusNotFound, "attachment::not_found")
	}
	object, err := storage.OpenFile(c, file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ctx.ApiError(http.StatusNotFound, "attachment::not_found")
		}
		logrus.WithContext(c).WithError(err).Error("Failed to open attachment")
		return ctx.ApiServerError()
	}
	defer func() { _ = object.Close() }()

	// Serve only inert image formats inline; active content is always downloaded.
	disposition := "attachment"
	switch file.ContentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/avif", "image/bmp":
		if ctx.Query("download") != "1" && ctx.Query("download") != "true" {
			disposition = "inline"
		}
	}
	w := ctx.ResponseWriter()
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": file.Name}))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", `"`+file.UID+`"`)
	http.ServeContent(w, ctx.Request().Request, file.Name, file.CreatedAt, object)
	return nil
}
