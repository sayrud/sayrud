package storage

import (
	"context"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/thanhpk/randstr"
)

// tmpPrefix is the name prefix of the files being written, they are skipped when iterating.
const tmpPrefix = ".tmp-"

var _ ObjectStorage = (*LocalStorage)(nil)

// LocalStorage stores the objects in a local directory, all the file operations are confined to it by os.Root.
type LocalStorage struct {
	root *os.Root
}

// NewLocalStorage creates a local storage rooted at dir, the directory is created if it does not exist.
func NewLocalStorage(dir string) (*LocalStorage, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, errors.Wrap(err, "create storage directory")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, errors.Wrap(err, "open storage directory")
	}
	return &LocalStorage{root: root}, nil
}

func (s *LocalStorage) Type() Type { return TypeLocal }

func (s *LocalStorage) Open(_ context.Context, p string) (Object, error) {
	p, err := cleanPath(p)
	if err != nil {
		return nil, err
	}
	f, err := s.root.Open(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, notExist("open", p)
		}
		return nil, errors.Wrap(err, "open")
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, errors.Wrap(err, "stat")
	}
	if !info.Mode().IsRegular() {
		_ = f.Close()
		return nil, notExist("open", p)
	}
	return f, nil
}

func (s *LocalStorage) Save(ctx context.Context, p string, r io.Reader, size int64) (int64, error) {
	p, err := cleanPath(p)
	if err != nil {
		return 0, err
	}
	dir := path.Dir(p)
	if dir != "." {
		if err := s.root.MkdirAll(dir, 0o755); err != nil {
			return 0, errors.Wrap(err, "create directory")
		}
	}

	// Write to a temporary file then rename it, so readers never see a partially written object.
	tmp := path.Join(dir, tmpPrefix+randstr.Hex(8))
	f, err := s.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return 0, errors.Wrap(err, "create temporary file")
	}
	src := io.Reader(&ctxReader{ctx: ctx, r: r})
	if size >= 0 {
		src = io.LimitReader(src, size+1)
	}
	n, err := io.Copy(f, src)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil && size >= 0 && n != size {
		err = errors.Newf("size mismatch: got %d bytes, want %d", n, size)
	}
	if err == nil {
		err = s.root.Rename(tmp, p)
	}
	if err != nil {
		_ = s.root.Remove(tmp)
		return 0, errors.Wrap(err, "save")
	}
	return n, nil
}

func (s *LocalStorage) Stat(_ context.Context, p string) (os.FileInfo, error) {
	p, err := cleanPath(p)
	if err != nil {
		return nil, err
	}
	info, err := s.root.Stat(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, notExist("stat", p)
		}
		return nil, errors.Wrap(err, "stat")
	}
	if !info.Mode().IsRegular() {
		return nil, notExist("stat", p)
	}
	return info, nil
}

func (s *LocalStorage) Delete(_ context.Context, p string) error {
	p, err := cleanPath(p)
	if err != nil {
		return err
	}
	if err := s.root.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return errors.Wrap(err, "delete")
	}
	return nil
}

// URL always returns ErrURLNotSupported, the business routes serve the local objects by Open.
func (s *LocalStorage) URL(context.Context, string, string) (*url.URL, error) {
	return nil, ErrURLNotSupported
}

func (s *LocalStorage) IterateObjects(ctx context.Context, prefix string, fn func(path string, obj Object) error) error {
	prefix, err := cleanPrefix(prefix)
	if err != nil {
		return err
	}
	start := "."
	if i := strings.LastIndex(prefix, "/"); i > 0 {
		start = prefix[:i]
	}
	return fs.WalkDir(s.root.FS(), start, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == start && errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			if p != start && !strings.HasPrefix(p+"/", prefix) {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || strings.HasPrefix(d.Name(), tmpPrefix) || !strings.HasPrefix(p, prefix) {
			return nil
		}
		obj, err := s.Open(ctx, p)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		defer func() { _ = obj.Close() }()
		return fn(p, obj)
	})
}
