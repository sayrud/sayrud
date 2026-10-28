package api

import (
	"bytes"
	gocontext "context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/collab"
	"github.com/wuhan005/sayrud/internal/context"
	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
	"github.com/wuhan005/sayrud/internal/dto"
	"github.com/wuhan005/sayrud/internal/form"
	"github.com/wuhan005/sayrud/internal/importer"
	"github.com/wuhan005/sayrud/internal/storage"
)

// Import is the routes of importing Excel / CSV files.
var Import importRoute

type importRoute struct{}

const (
	// importStaleAfter is the time after which a running import without progress is treated as interrupted, e.g. its server crashed.
	importStaleAfter = time.Minute
	// importHeartbeat renews the liveness of a running import while it waits for the table lock or inserts a chunk.
	importHeartbeat = 15 * time.Second
	// importExpiry is how long the uploaded files are kept for starting or retrying the import.
	importExpiry = 24 * time.Hour
)

// LimitUpload bounds the request body and removes multipart temporary files.
func (importRoute) LimitUpload(ctx context.Context) {
	r := ctx.Request().Request
	r.Body = http.MaxBytesReader(ctx.ResponseWriter(), r.Body, importer.MaxFileSize+(64<<10))
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	ctx.Next()
}

// Importer loads the import of the path, only its uploader can access it.
func (importRoute) Importer(ctx context.Context, user *db.User, project *db.Project) error {
	item, err := db.SLImports.GetByUID(ctx.Request().Context(), ctx.Param("importUID"))
	if err != nil {
		if errors.Is(err, db.ErrSLImportNotFound) {
			return ctx.ApiError(http.StatusNotFound, "import::not_found")
		}
		logrus.WithContext(ctx.Request().Context()).WithError(err).Error("Failed to get import")
		return ctx.ApiServerError()
	}
	if item.ProjectID != project.ID || item.UserID != user.ID {
		return ctx.ApiError(http.StatusNotFound, "import::not_found")
	}
	ctx.Map(item)
	return nil
}

// readError responds the error of reading the imported file.
func readError(ctx context.Context, err error) error {
	switch {
	case errors.Is(err, importer.ErrUnsupportedFormat):
		return ctx.ApiError(http.StatusBadRequest, "import::unsupported_format")
	case errors.Is(err, importer.ErrLegacyExcel):
		return ctx.ApiError(http.StatusBadRequest, "import::legacy_excel")
	case errors.Is(err, importer.ErrNoData):
		return ctx.ApiError(http.StatusBadRequest, "import::no_data")
	case errors.Is(err, importer.ErrTooManyRows):
		return ctx.ApiError(http.StatusBadRequest, "import::too_many_rows", importer.MaxRows)
	case errors.Is(err, importer.ErrTooManyColumns):
		return ctx.ApiError(http.StatusBadRequest, "import::too_many_columns", importer.MaxColumns)
	default:
		logrus.WithContext(ctx.Request().Context()).WithError(err).Warn("Failed to read imported file")
		return ctx.ApiError(http.StatusBadRequest, "import::read_failed")
	}
}

// Upload
// @Summary Upload a file to import
// @Tags Imports
// @Description Upload an Excel (.xlsx) or CSV file up to 20 MiB. The response previews the visible sheets with the inferred column types; the file is kept for 24 hours to start the import.
// @Accept multipart/form-data
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param file formData file true "Excel or CSV file"
// @Success 200 {object} dto.ImportUpload
// @Failure 400 {string} string "Unsupported, empty or invalid file"
// @Failure 401 {string} string "Not signed in"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project not found"
// @Failure 413 {string} string "File is too large"
// @Failure 500 {string} string "Internal server error"
// @ID uploadImport
// @Router /projects/{projectUID}/imports [post]
func (importRoute) Upload(ctx context.Context, hub *collab.Hub, user *db.User, project *db.Project) error {
	r := ctx.Request().Request
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return ctx.ApiError(http.StatusRequestEntityTooLarge, "import::too_large", importer.MaxFileSize>>20)
		}
		return ctx.ApiError(http.StatusBadRequest, "common::invalid_body")
	}
	uploaded, header, err := r.FormFile("file")
	if err != nil || len(r.MultipartForm.File["file"]) != 1 {
		return ctx.ApiError(http.StatusBadRequest, "common::invalid_body")
	}
	defer func() { _ = uploaded.Close() }()
	if header.Size > importer.MaxFileSize {
		return ctx.ApiError(http.StatusRequestEntityTooLarge, "import::too_large", importer.MaxFileSize>>20)
	}

	data, err := io.ReadAll(io.LimitReader(uploaded, importer.MaxFileSize+1))
	if err != nil {
		return ctx.ApiError(http.StatusBadRequest, "common::invalid_body")
	}
	sheets, err := importer.Read(header.Filename, data)
	if err != nil {
		return readError(ctx, err)
	}

	c := r.Context()
	file, err := storage.Upload(c, storage.UploadOptions{
		Category:  storage.CategoryImport,
		UserID:    user.ID,
		ProjectID: project.ID,
		Name:      header.Filename,
		Reader:    bytes.NewReader(data),
		Size:      int64(len(data)),
	})
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to save imported file")
		return ctx.ApiServerError()
	}
	item, err := db.SLImports.Create(c, db.CreateSLImportOptions{
		ProjectID: project.ID,
		UserID:    user.ID,
		FileID:    file.ID,
		FileName:  file.Name,
	})
	if err != nil {
		_ = storage.Remove(c, file)
		logrus.WithContext(c).WithError(err).Error("Failed to create import")
		return ctx.ApiServerError()
	}

	hub.RunBackground(c, removeExpiredImports)
	return ctx.ApiSuccess(&dto.ImportUpload{
		UID:      item.UID,
		FileName: item.FileName,
		Sheets:   dto.ToImportSheets(sheets),
	})
}

// removeExpiredImports deletes the imports and the files uploaded before the expiry.
func removeExpiredImports(ctx gocontext.Context) {
	items, err := db.SLImports.ListExpired(ctx, dbutil.Now().Add(-importExpiry))
	if err != nil {
		logrus.WithContext(ctx).WithError(err).Error("Failed to list expired imports")
		return
	}
	for _, item := range items {
		if file, err := db.Files.GetByID(ctx, item.FileID); err == nil {
			_ = storage.Remove(ctx, file)
		}
		if err := db.SLImports.DeleteByID(ctx, item.ID); err != nil {
			logrus.WithContext(ctx).WithError(err).Error("Failed to delete expired import")
		}
	}
}

// toImport returns the state of the import with the error translated in the language of the request.
func (importRoute) toImport(ctx context.Context, item *db.SLImport) *dto.Import {
	result := &dto.Import{
		UID:    item.UID,
		Status: item.Status,
		Done:   item.Done,
		Total:  item.Total,
	}
	importErr := item.Error.Data()
	if item.Status == db.SLImportRunning && item.UpdatedAt.Before(dbutil.Now().Add(-importStaleAfter)) {
		result.Status = db.SLImportFailed
		importErr = db.ImportError{Key: "import::interrupted"}
	}
	if result.Status == db.SLImportFailed && importErr.Key != "" {
		result.Error = ctx.Tr(importErr.Key, lo.ToAnySlice(importErr.Args)...)
	}
	if item.SLTableID != 0 {
		if table, err := db.SLTables.GetByID(ctx.Request().Context(), item.SLTableID); err == nil {
			result.TableUID = table.UID
		}
	}
	return result
}

// Get
// @Summary Get the state of an import
// @Tags Imports
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param importUID path string true "Import UID"
// @Success 200 {object} dto.Import
// @Failure 401 {string} string "Not signed in"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project or import not found"
// @Failure 500 {string} string "Internal server error"
// @ID getImport
// @Router /projects/{projectUID}/imports/{importUID} [get]
func (r importRoute) Get(ctx context.Context, item *db.SLImport) error {
	return ctx.ApiSuccess(r.toImport(ctx, item))
}

// Start
// @Summary Start an import
// @Tags Imports
// @Description Import a sheet of the uploaded file in the background, into the table or a new table. The fields, options and records are written in a single transaction, so a failed import changes nothing. Poll the import for the progress.
// @Accept json
// @Produce json
// @Param projectUID path string true "Project UID"
// @Param importUID path string true "Import UID"
// @Param data body form.StartImport true "Sheet and column mapping"
// @Success 200 {object} dto.Import
// @Failure 400 {string} string "Invalid sheet, columns or table name"
// @Failure 401 {string} string "Not signed in"
// @Failure 403 {string} string "Permission denied"
// @Failure 404 {string} string "Project, import, table or file not found"
// @Failure 409 {string} string "The import is running or has succeeded"
// @Failure 500 {string} string "Internal server error"
// @ID startImport
// @Router /projects/{projectUID}/imports/{importUID}/start [post]
func (r importRoute) Start(ctx context.Context, hub *collab.Hub, project *db.Project, item *db.SLImport, f form.StartImport) error {
	c := ctx.Request().Context()
	if item.Status == db.SLImportSucceeded || (item.Status == db.SLImportRunning && !item.UpdatedAt.Before(dbutil.Now().Add(-importStaleAfter))) {
		return ctx.ApiError(http.StatusConflict, "import::already_started")
	}

	file, err := db.Files.GetByID(c, item.FileID)
	if err != nil {
		if errors.Is(err, db.ErrFileNotFound) {
			return ctx.ApiError(http.StatusNotFound, "import::expired")
		}
		logrus.WithContext(c).WithError(err).Error("Failed to get imported file")
		return ctx.ApiServerError()
	}
	sheets, err := readImportFile(c, file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ctx.ApiError(http.StatusNotFound, "import::expired")
		}
		return readError(ctx, err)
	}
	if f.Sheet < 0 || f.Sheet >= len(sheets) || len(f.Columns) == 0 {
		return ctx.ApiError(http.StatusBadRequest, "import::invalid_columns")
	}
	sheet := sheets[f.Sheet]
	rows := sheet.Rows
	if f.HeaderRow {
		rows = rows[1:]
	}

	columns := make([]collab.ImportColumn, 0, len(f.Columns))
	for _, col := range f.Columns {
		column := collab.ImportColumn{
			Index:    col.Index,
			FieldUID: col.FieldUID,
			Label:    strings.TrimSpace(col.Label),
			Type:     db.SLFieldType(col.Type),
		}
		invalidNew := col.FieldUID == "" && (column.Label == "" || !lo.Contains(collab.ImportableFieldTypes, column.Type))
		if col.Index < 0 || col.Index >= sheet.Width || invalidNew {
			return ctx.ApiError(http.StatusBadRequest, "import::invalid_columns")
		}
		columns = append(columns, column)
	}

	job := importJob{
		file: file,
		opts: collab.ImportOptions{
			Rows:     rows,
			Columns:  columns,
			Location: time.Local,
		},
	}
	if f.Timezone != "" {
		if loc, err := time.LoadLocation(f.Timezone); err == nil {
			job.opts.Location = loc
		}
	}
	if f.TableUID != "" {
		table, err := db.SLTables.GetByUID(c, f.TableUID)
		if err != nil || table.ProjectID != project.ID {
			return ctx.ApiError(http.StatusNotFound, "table::not_found")
		}
		job.table = table
	} else {
		job.tableName = strings.TrimSpace(f.TableName)
		job.opts.ViewName = strings.TrimSpace(f.ViewName)
		if job.tableName == "" {
			return ctx.ApiError(http.StatusBadRequest, "import::table_name_required")
		}
		if job.opts.ViewName == "" {
			job.opts.ViewName = "Grid"
		}
	}

	started, err := db.SLImports.Start(c, item.ID, dbutil.Now().Add(-importStaleAfter))
	if err != nil {
		logrus.WithContext(c).WithError(err).Error("Failed to start import")
		return ctx.ApiServerError()
	}
	if !started {
		return ctx.ApiError(http.StatusConflict, "import::already_started")
	}

	hub.RunBackground(c, func(ctx gocontext.Context) { job.run(ctx, hub, project, item) })
	result := &dto.Import{
		UID:    item.UID,
		Status: db.SLImportRunning,
		Total:  len(rows),
	}
	if job.table != nil {
		result.TableUID = job.table.UID
	}
	return ctx.ApiSuccess(result)
}

// readImportFile reads the sheets of the uploaded file from the storage.
func readImportFile(ctx gocontext.Context, file *db.File) ([]*importer.Sheet, error) {
	object, err := storage.OpenFile(ctx, file)
	if err != nil {
		return nil, err
	}
	defer func() { _ = object.Close() }()
	data, err := io.ReadAll(io.LimitReader(object, importer.MaxFileSize+1))
	if err != nil {
		return nil, errors.Wrap(err, "read file")
	}
	return importer.Read(file.Name, data)
}

// importJob writes the rows of an import, into the table or a new table named tableName.
type importJob struct {
	// file is the uploaded file, removed after the import succeeds.
	file *db.File
	// table is the existing target table, nil to create a table.
	table *db.SLTable
	// tableName is the name of the table to create.
	tableName string
	// opts are the rows and the column mapping to import.
	opts collab.ImportOptions
}

// run writes the import and saves its final state, the table created for a failed import is removed.
func (job importJob) run(ctx gocontext.Context, hub *collab.Hub, project *db.Project, item *db.SLImport) {
	// The final state is saved even if the server shuts down during the import.
	finishCtx := gocontext.WithoutCancel(ctx)

	var mu sync.Mutex
	done, total := 0, len(job.opts.Rows)
	job.opts.Progress = func(d, t int) {
		mu.Lock()
		done, total = d, t
		mu.Unlock()
		_ = db.SLImports.SetProgress(finishCtx, item.ID, d, t)
	}
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		ticker := time.NewTicker(importHeartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				mu.Lock()
				d, t := done, total
				mu.Unlock()
				_ = db.SLImports.SetProgress(finishCtx, item.ID, d, t)
			}
		}
	}()

	table, created := job.table, false
	if table != nil {
		_ = db.SLImports.SetTable(finishCtx, item.ID, table.ID)
	} else {
		var err error
		table, err = db.SLTables.Create(ctx, project.ID, db.CreateSLTableOptions{Name: job.tableName})
		if err != nil {
			_ = db.SLImports.Finish(finishCtx, item.ID, 0, importFailure(ctx, err))
			return
		}
		created = true
		_ = db.SLImports.SetTable(finishCtx, item.ID, table.ID)
		hub.NotifyProject(project.UID, collab.MessageTablesChanged)
	}

	n, err := hub.Import(ctx, project, table, job.opts)
	if err != nil {
		// Nothing is written by the failed transaction, so the table created for it is removed.
		if created {
			_ = db.SLTables.DeleteByID(finishCtx, table.ID)
			hub.NotifyProject(project.UID, collab.MessageTablesChanged)
		}
		_ = db.SLImports.Finish(finishCtx, item.ID, 0, importFailure(ctx, err))
		return
	}
	if err := db.SLImports.Finish(finishCtx, item.ID, n, nil); err != nil {
		logrus.WithContext(ctx).WithError(err).Error("Failed to finish import")
	}
	if err := storage.Remove(finishCtx, job.file); err != nil {
		logrus.WithContext(ctx).WithError(err).Warn("Failed to remove imported file")
	}
}

// importFailure returns the reason saved for the failed import.
func importFailure(ctx gocontext.Context, err error) *db.ImportError {
	var opErr *collab.OperationError
	switch {
	case errors.As(err, &opErr):
		return &db.ImportError{
			Key:  opErr.Key,
			Args: lo.Map(opErr.Args, func(a interface{}, _ int) string { return fmt.Sprint(a) }),
		}
	case errors.Is(err, db.ErrSLTableExists):
		return &db.ImportError{Key: "table::exists"}
	case ctx.Err() != nil:
		return &db.ImportError{Key: "import::interrupted"}
	default:
		logrus.WithContext(ctx).WithError(err).Error("Failed to import")
		return &db.ImportError{Key: "common::internal_error"}
	}
}
