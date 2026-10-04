// Package storage stores the uploaded files on the local disk or S3-compatible object storage through the global instance Default.
// The business code usually calls Upload / Remove / FileURL / OpenFile, which also maintain the db.Files records.
package storage

import (
	"context"
	"io"
	"io/fs"
	"mime"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/wuhan005/sayrud/internal/conf"
)

// Type is the type of the storage medium.
type Type string

const (
	TypeLocal Type = "local"
	TypeS3    Type = "s3"
)

// Object is a single object in the storage, it must be closed after reading.
type Object interface {
	io.ReadSeekCloser
	Stat() (os.FileInfo, error)
}

// ObjectStorage is the unified interface of the storage media, paths are relative and separated by "/".
// The errors of the nonexistent objects satisfy errors.Is(err, fs.ErrNotExist).
type ObjectStorage interface {
	// Type returns the type of the storage medium.
	Type() Type
	// Open opens the object for reading.
	Open(ctx context.Context, path string) (Object, error)
	// Save writes the object and returns the number of bytes written, an existing object is overwritten.
	// The size is -1 if unknown, otherwise a mismatched length fails without leaving the object behind.
	Save(ctx context.Context, path string, r io.Reader, size int64) (int64, error)
	// Stat returns the information of the object.
	Stat(ctx context.Context, path string) (os.FileInfo, error)
	// Delete deletes the object, deleting a nonexistent object is not an error.
	Delete(ctx context.Context, path string) error
	// URL returns the presigned or CDN URL of the object for the client to download directly, the name is the download file name.
	// It returns ErrURLNotSupported if the medium can not be accessed directly, the caller then serves the content by Open.
	URL(ctx context.Context, path, name string) (*url.URL, error)
	// IterateObjects calls fn for every object whose path starts with prefix, it stops and returns the error returned by fn.
	// The obj is closed after fn returns.
	IterateObjects(ctx context.Context, prefix string, fn func(path string, obj Object) error) error
}

// Default is the global storage instance created by Init.
var Default ObjectStorage

// Init creates the global storage instance from conf.Storage.
func Init(ctx context.Context) error {
	s, err := New(ctx, conf.Storage)
	if err != nil {
		return err
	}
	Default = s
	return nil
}

// New creates a storage instance from the configuration.
func New(ctx context.Context, cfg conf.StorageConfig) (ObjectStorage, error) {
	switch Type(cfg.Type) {
	case TypeLocal, "":
		return NewLocalStorage(cfg.Local.Path)
	case TypeS3:
		return NewS3Storage(ctx, cfg.S3, cfg.URLExpiry)
	default:
		return nil, errors.Newf("unsupported storage type %q", cfg.Type)
	}
}

var (
	ErrInvalidPath     = errors.New("invalid storage path")
	ErrURLNotSupported = errors.New("storage does not support direct URLs")
)

// cleanPath normalizes the object path by removing the leading "/" and resolving "..", the result never escapes the root.
func cleanPath(p string) (string, error) {
	if strings.ContainsRune(p, 0) {
		return "", ErrInvalidPath
	}
	p = path.Clean("/" + strings.ReplaceAll(p, "\\", "/"))[1:]
	if p == "" {
		return "", ErrInvalidPath
	}
	return p, nil
}

// cleanPrefix normalizes the iteration prefix, the trailing "/" is kept to match a directory and empty means all.
func cleanPrefix(prefix string) (string, error) {
	if prefix == "" || prefix == "/" {
		return "", nil
	}
	p, err := cleanPath(prefix)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(prefix, "/") {
		p += "/"
	}
	return p, nil
}

func notExist(op, p string) error {
	return &fs.PathError{Op: op, Path: p, Err: fs.ErrNotExist}
}

// inlineTypes are the types displayed inline by browsers without running scripts, the others are downloaded as attachments.
var inlineTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true, "image/avif": true, "image/bmp": true,
	"video/mp4": true, "video/webm": true, "audio/mpeg": true, "audio/ogg": true, "audio/wav": true,
	"text/plain": true,
}

// ResponseHeaders returns the Content-Type and Content-Disposition of a download response by the file extension.
// Business routes serving the user uploaded content should also set "X-Content-Type-Options: nosniff" and a sandbox CSP.
func ResponseHeaders(p, name string) (contentType, disposition string) {
	ext := path.Ext(name)
	if ext == "" {
		ext = path.Ext(p)
	}
	contentType = mime.TypeByExtension(strings.ToLower(ext))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	mediaType, _, _ := mime.ParseMediaType(contentType)
	dispositionType := "attachment"
	if inlineTypes[mediaType] {
		dispositionType = "inline"
	}
	if name == "" {
		return contentType, dispositionType
	}
	return contentType, mime.FormatMediaType(dispositionType, map[string]string{"filename": name})
}

// objectInfo implements os.FileInfo for the objects in object storage.
type objectInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (i *objectInfo) Name() string       { return i.name }
func (i *objectInfo) Size() int64        { return i.size }
func (i *objectInfo) Mode() os.FileMode  { return 0o644 }
func (i *objectInfo) ModTime() time.Time { return i.modTime }
func (i *objectInfo) IsDir() bool        { return false }
func (i *objectInfo) Sys() interface{}   { return nil }

// ctxReader stops reading once ctx is canceled, so a disconnected client does not keep writing to the storage.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *ctxReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
