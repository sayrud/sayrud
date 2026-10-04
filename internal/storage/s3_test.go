package storage

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thanhpk/randstr"

	"github.com/wuhan005/sayrud/internal/conf"
)

// newTestS3 connects to the S3-compatible service (e.g. a local MinIO) of S3_TEST_ENDPOINT, the test is skipped if it is not set.
func newTestS3(t *testing.T) (*S3Storage, conf.S3StorageConfig) {
	t.Helper()
	endpoint := os.Getenv("S3_TEST_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_TEST_ENDPOINT is not set")
	}
	cfg := conf.S3StorageConfig{
		Endpoint:        endpoint,
		Region:          "us-east-1",
		Bucket:          "sayrud-test",
		AccessKeyID:     os.Getenv("S3_TEST_ACCESS_KEY"),
		SecretAccessKey: os.Getenv("S3_TEST_SECRET_KEY"),
		UsePathStyle:    true,
		BasePath:        "/run-" + strings.ToLower(randstr.String(8)) + "/",
	}
	ctx := context.Background()
	s, err := NewS3Storage(ctx, cfg, 10*time.Minute)
	require.NoError(t, err)
	_, _ = s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(cfg.Bucket)})
	return s, cfg
}

// unseekable hides the Seek of the underlying reader, like an HTTP request body.
type unseekable struct{ io.Reader }

func TestS3Storage(t *testing.T) {
	ctx := context.Background()
	s, cfg := newTestS3(t)

	n, err := s.Save(ctx, "avatars/2026/10/a.png", unseekable{strings.NewReader("hello world")}, 11)
	require.NoError(t, err)
	assert.Equal(t, int64(11), n)
	n, err = s.Save(ctx, "attachments/2026/10/b.txt", unseekable{strings.NewReader("unknown size")}, -1)
	require.NoError(t, err)
	assert.Equal(t, int64(12), n)

	info, err := s.Stat(ctx, "avatars/2026/10/a.png")
	require.NoError(t, err)
	assert.Equal(t, int64(11), info.Size())
	assert.Equal(t, "a.png", info.Name())
	assert.False(t, info.ModTime().IsZero())

	head, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(cfg.Bucket), Key: aws.String(s.basePath + "avatars/2026/10/a.png")})
	require.NoError(t, err)
	assert.Equal(t, "image/png", aws.ToString(head.ContentType))

	obj, err := s.Open(ctx, "avatars/2026/10/a.png")
	require.NoError(t, err)
	buf := make([]byte, 5)
	_, err = io.ReadFull(obj, buf)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(buf))
	_, err = obj.Seek(-5, io.SeekEnd)
	require.NoError(t, err)
	rest, err := io.ReadAll(obj)
	require.NoError(t, err)
	assert.Equal(t, "world", string(rest))
	require.NoError(t, obj.Close())

	// A mismatched length fails without leaving the object behind.
	_, err = s.Save(ctx, "attachments/short.txt", unseekable{strings.NewReader("abc")}, 4)
	require.Error(t, err)
	_, err = s.Stat(ctx, "attachments/short.txt")
	assert.ErrorIs(t, err, fs.ErrNotExist)

	u, err := s.URL(ctx, "avatars/2026/10/a.png", "头像.png")
	require.NoError(t, err)
	resp, err := http.Get(u.String())
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, string(body))
	assert.Equal(t, "hello world", string(body))
	assert.Equal(t, "image/png", resp.Header.Get("Content-Type"))
	assert.Equal(t, "inline; filename*=utf-8''%E5%A4%B4%E5%83%8F.png", resp.Header.Get("Content-Disposition"))

	var paths []string
	require.NoError(t, s.IterateObjects(ctx, "", func(p string, obj Object) error {
		info, err := obj.Stat()
		require.NoError(t, err)
		b, err := io.ReadAll(obj)
		require.NoError(t, err)
		assert.Equal(t, info.Size(), int64(len(b)))
		paths = append(paths, p)
		return nil
	}))
	sort.Strings(paths)
	assert.Equal(t, []string{"attachments/2026/10/b.txt", "avatars/2026/10/a.png"}, paths)
	paths = nil
	require.NoError(t, s.IterateObjects(ctx, "avatars/", func(p string, _ Object) error {
		paths = append(paths, p)
		return nil
	}))
	assert.Equal(t, []string{"avatars/2026/10/a.png"}, paths)

	for _, p := range []string{"avatars/2026/10/a.png", "attachments/2026/10/b.txt"} {
		require.NoError(t, s.Delete(ctx, p))
		require.NoError(t, s.Delete(ctx, p))
		_, err = s.Stat(ctx, p)
		assert.ErrorIs(t, err, fs.ErrNotExist)
		_, err = s.Open(ctx, p)
		assert.ErrorIs(t, err, fs.ErrNotExist)
	}
}

func TestS3Storage_PublicURL(t *testing.T) {
	_, cfg := newTestS3(t)
	cfg.BasePath = "files"
	cfg.PublicURL = "https://cdn.example.com/sayrud/"
	s, err := NewS3Storage(context.Background(), cfg, time.Hour)
	require.NoError(t, err)
	u, err := s.URL(context.Background(), "avatars/a.png", "a.png")
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/sayrud/files/avatars/a.png", u.String())

	cfg.PublicURL = "cdn.example.com"
	_, err = NewS3Storage(context.Background(), cfg, time.Hour)
	assert.Error(t, err)
}
