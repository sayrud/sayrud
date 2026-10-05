package storage

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/db"
	"github.com/wuhan005/sayrud/internal/dbutil"
)

// Category is the business usage of a file, it is also the first directory of the storage path.
type Category string

const (
	CategoryAvatar     Category = "avatars"
	CategoryAttachment Category = "attachments"
)

func (c Category) valid() bool {
	return c == CategoryAvatar || c == CategoryAttachment
}

// NameMaxLength is the maximum number of characters of the saved original file name.
const NameMaxLength = 255

// UploadOptions are the options of uploading a file, the size limit and type validation are up to the business code.
type UploadOptions struct {
	Category Category
	// UserID is the uploader.
	UserID int64
	// ProjectID scopes attachments to a project; avatars leave it at zero.
	ProjectID int64
	// Name is the original file name, only its last element is kept and the control characters are removed.
	Name   string
	Reader io.Reader
	// Size is the number of bytes, -1 if unknown.
	Size int64
}

// Upload writes the file to the global storage and creates its db.Files record, the object is deleted if creating the record fails.
func Upload(ctx context.Context, opts UploadOptions) (*db.File, error) {
	if !opts.Category.valid() {
		return nil, errors.Newf("invalid file category %q", opts.Category)
	}
	uid := db.NewFileUID()
	name := sanitizeName(opts.Name)
	if name == "" {
		name = uid
	}
	p := path.Join(string(opts.Category), dbutil.Now().Format("2006/01"), uid+extension(name))

	br := bufio.NewReader(opts.Reader)
	head, err := br.Peek(512)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, errors.Wrap(err, "read file header")
	}
	contentType := http.DetectContentType(head)
	hash := sha256.New()
	size, err := Default.Save(ctx, p, io.TeeReader(br, hash), opts.Size)
	if err != nil {
		return nil, errors.Wrap(err, "save object")
	}

	file, err := db.Files.Create(ctx, db.CreateFileOptions{
		UID:         uid,
		Category:    string(opts.Category),
		Storage:     string(Default.Type()),
		Path:        p,
		Name:        name,
		Size:        size,
		ContentType: contentType,
		SHA256:      hex.EncodeToString(hash.Sum(nil)),
		UserID:      opts.UserID,
		ProjectID:   opts.ProjectID,
	})
	if err != nil {
		if delErr := Default.Delete(context.WithoutCancel(ctx), p); delErr != nil {
			logrus.WithContext(ctx).WithError(delErr).WithField("path", p).Warn("Failed to delete orphan storage object")
		}
		return nil, errors.Wrap(err, "create file record")
	}
	return file, nil
}

// Remove deletes the file record and its object. A failed object deletion is only logged, the leftovers can be cleaned up by IterateObjects.
func Remove(ctx context.Context, file *db.File) error {
	if err := db.Files.DeleteByID(ctx, file.ID); err != nil {
		return err
	}
	if err := Default.Delete(context.WithoutCancel(ctx), file.Path); err != nil {
		logrus.WithContext(ctx).WithError(err).WithField("path", file.Path).Warn("Failed to delete storage object")
	}
	return nil
}

// FileURL returns the presigned or CDN URL of the file downloaded with its original name.
// It returns ErrURLNotSupported for the local storage, the business route then serves the content by OpenFile.
func FileURL(ctx context.Context, file *db.File) (*url.URL, error) {
	return Default.URL(ctx, file.Path, file.Name)
}

// OpenFile opens the content of the file for reading, it must be closed after reading.
func OpenFile(ctx context.Context, file *db.File) (Object, error) {
	return Default.Open(ctx, file.Path)
}

func sanitizeName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if r == utf8.RuneError || unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "." || name == ".." {
		return ""
	}
	if utf8.RuneCountInString(name) > NameMaxLength {
		ext := extension(name)
		name = string([]rune(name)[:NameMaxLength-len(ext)]) + ext
	}
	return name
}

var extensionPattern = regexp.MustCompile(`^\.[a-z0-9]{1,16}$`)

// extension returns the lowercase file extension, or empty if it is unusual, so arbitrary characters never get into the storage path.
func extension(name string) string {
	ext := strings.ToLower(path.Ext(name))
	if !extensionPattern.MatchString(ext) {
		return ""
	}
	return ext
}
