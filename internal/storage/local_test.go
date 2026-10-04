package storage

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLocal(t *testing.T) (*LocalStorage, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := NewLocalStorage(dir)
	require.NoError(t, err)
	return s, dir
}

func TestCleanPath(t *testing.T) {
	for in, want := range map[string]string{
		"a/b.png":       "a/b.png",
		"/a//b.png":     "a/b.png",
		"../../etc/x":   "etc/x",
		"a/../../b":     "b",
		`a\..\..\b.png`: "b.png",
	} {
		got, err := cleanPath(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "/", "..", "a/..", "a\x00b"} {
		_, err := cleanPath(in)
		assert.ErrorIs(t, err, ErrInvalidPath, in)
	}
}

func TestLocalStorage(t *testing.T) {
	ctx := context.Background()
	s, dir := newTestLocal(t)

	n, err := s.Save(ctx, "avatars/2026/10/a.png", strings.NewReader("hello"), 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), n)

	info, err := s.Stat(ctx, "avatars/2026/10/a.png")
	require.NoError(t, err)
	assert.Equal(t, int64(5), info.Size())
	assert.Equal(t, "a.png", info.Name())

	obj, err := s.Open(ctx, "/avatars/2026/10/a.png")
	require.NoError(t, err)
	_, err = obj.Seek(1, io.SeekStart)
	require.NoError(t, err)
	b, err := io.ReadAll(obj)
	require.NoError(t, err)
	assert.Equal(t, "ello", string(b))
	require.NoError(t, obj.Close())

	// Overwrite with an unknown size.
	n, err = s.Save(ctx, "avatars/2026/10/a.png", strings.NewReader("hi"), -1)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
	got, err := os.ReadFile(filepath.Join(dir, "avatars/2026/10/a.png"))
	require.NoError(t, err)
	assert.Equal(t, "hi", string(got))

	// Directories are not objects.
	_, err = s.Stat(ctx, "avatars/2026")
	assert.ErrorIs(t, err, fs.ErrNotExist)
	_, err = s.Open(ctx, "avatars/2026")
	assert.ErrorIs(t, err, fs.ErrNotExist)

	require.NoError(t, s.Delete(ctx, "avatars/2026/10/a.png"))
	require.NoError(t, s.Delete(ctx, "avatars/2026/10/a.png"))
	_, err = s.Stat(ctx, "avatars/2026/10/a.png")
	assert.ErrorIs(t, err, fs.ErrNotExist)
	_, err = s.Open(ctx, "avatars/2026/10/a.png")
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

func TestLocalStorage_SaveFailureLeavesNothing(t *testing.T) {
	ctx := context.Background()
	s, dir := newTestLocal(t)

	_, err := s.Save(ctx, "a/short.txt", strings.NewReader("abc"), 4)
	require.Error(t, err)
	_, err = s.Save(ctx, "a/long.txt", strings.NewReader("abcde"), 4)
	require.Error(t, err)
	_, err = s.Save(ctx, "a/broken.txt", iotest.ErrReader(io.ErrUnexpectedEOF), -1)
	require.Error(t, err)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = s.Save(canceled, "a/canceled.txt", strings.NewReader("abc"), 3)
	require.ErrorIs(t, err, context.Canceled)

	entries, err := os.ReadDir(filepath.Join(dir, "a"))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestLocalStorage_StaysInRoot(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()
	dir := filepath.Join(parent, "root")
	s, err := NewLocalStorage(dir)
	require.NoError(t, err)

	_, err = s.Save(ctx, "../escape.txt", strings.NewReader("x"), 1)
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(parent, "escape.txt"))
	assert.ErrorIs(t, err, fs.ErrNotExist)
	_, err = os.Stat(filepath.Join(dir, "escape.txt"))
	assert.NoError(t, err)

	// Symlinks pointing outside the root are not readable.
	require.NoError(t, os.WriteFile(filepath.Join(parent, "secret.txt"), []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(filepath.Join(parent, "secret.txt"), filepath.Join(dir, "link.txt")))
	_, err = s.Open(ctx, "link.txt")
	assert.Error(t, err)
}

func TestLocalStorage_IterateObjects(t *testing.T) {
	ctx := context.Background()
	s, dir := newTestLocal(t)
	for _, p := range []string{"avatars/2026/09/a.png", "avatars/2026/10/b.png", "attachments/2026/10/c.pdf", "avatarsx/d.png"} {
		_, err := s.Save(ctx, p, strings.NewReader(p), -1)
		require.NoError(t, err)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "avatars/2026/10", tmpPrefix+"x"), []byte("tmp"), 0o644))

	list := func(prefix string) []string {
		var paths []string
		require.NoError(t, s.IterateObjects(ctx, prefix, func(p string, obj Object) error {
			b, err := io.ReadAll(obj)
			require.NoError(t, err)
			assert.Equal(t, p, string(b))
			paths = append(paths, p)
			return nil
		}))
		sort.Strings(paths)
		return paths
	}
	assert.Equal(t, []string{"attachments/2026/10/c.pdf", "avatars/2026/09/a.png", "avatars/2026/10/b.png", "avatarsx/d.png"}, list(""))
	assert.Equal(t, []string{"avatars/2026/09/a.png", "avatars/2026/10/b.png"}, list("avatars/"))
	assert.Equal(t, []string{"avatars/2026/09/a.png", "avatars/2026/10/b.png", "avatarsx/d.png"}, list("avatars"))
	assert.Equal(t, []string{"avatars/2026/10/b.png"}, list("avatars/2026/1"))
	assert.Empty(t, list("missing/"))

	stop := assert.AnError
	count := 0
	err := s.IterateObjects(ctx, "", func(string, Object) error {
		count++
		return stop
	})
	assert.ErrorIs(t, err, stop)
	assert.Equal(t, 1, count)
}

func TestLocalStorage_URL(t *testing.T) {
	s, _ := newTestLocal(t)
	_, err := s.URL(context.Background(), "attachments/a.png", "a.png")
	assert.ErrorIs(t, err, ErrURLNotSupported)
}

func TestResponseHeaders(t *testing.T) {
	for _, tc := range []struct {
		path, name, contentType, disposition string
	}{
		{"a/x.png", "", "image/png", "inline"},
		{"a/x", "photo.JPG", "image/jpeg", "inline; filename=photo.JPG"},
		{"a/x.svg", "x.svg", "image/svg+xml", "attachment; filename=x.svg"},
		{"a/x", "", "application/octet-stream", "attachment"},
		{"a/x.pdf", "报告.pdf", "application/pdf", "attachment; filename*=utf-8''%E6%8A%A5%E5%91%8A.pdf"},
	} {
		contentType, disposition := ResponseHeaders(tc.path, tc.name)
		assert.Equal(t, tc.contentType, contentType, tc.path)
		assert.Equal(t, tc.disposition, disposition, tc.path)
	}
}
