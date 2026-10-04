package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/cockroachdb/errors"
	"github.com/sirupsen/logrus"

	"github.com/wuhan005/sayrud/internal/conf"
)

var _ ObjectStorage = (*S3Storage)(nil)

// S3Storage stores the objects in S3 or S3-compatible object storage.
type S3Storage struct {
	client    *s3.Client
	presign   *s3.PresignClient
	bucket    string
	basePath  string
	publicURL *url.URL
	expiry    time.Duration
}

// NewS3Storage creates an S3 storage, expiry is the lifetime of the presigned URLs. A failed connectivity check is only logged.
func NewS3Storage(ctx context.Context, cfg conf.S3StorageConfig, expiry time.Duration) (*S3Storage, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("storage.s3.bucket is required")
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	// Only calculate checksums when required, the S3-compatible services may not support the new default checksums.
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		config.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired),
	}
	if cfg.AccessKeyID != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "load aws config")
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.UsePathStyle
	})

	s := &S3Storage{
		client:  client,
		presign: s3.NewPresignClient(client),
		bucket:  cfg.Bucket,
		expiry:  expiry,
	}
	if s.expiry <= 0 {
		s.expiry = time.Hour
	}
	if base := strings.Trim(cfg.BasePath, "/"); base != "" {
		s.basePath = base + "/"
	}
	if cfg.PublicURL != "" {
		u, err := url.Parse(cfg.PublicURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, errors.Newf("invalid storage.s3.public_url %q", cfg.PublicURL)
		}
		s.publicURL = u
	}

	if _, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)}); err != nil {
		logrus.WithError(err).Warnf("Failed to access S3 bucket %q", s.bucket)
	}
	return s, nil
}

func (s *S3Storage) Type() Type { return TypeS3 }

func (s *S3Storage) key(p string) (string, error) {
	p, err := cleanPath(p)
	if err != nil {
		return "", err
	}
	return s.basePath + p, nil
}

func (s *S3Storage) Open(ctx context.Context, p string) (Object, error) {
	info, err := s.Stat(ctx, p)
	if err != nil {
		return nil, err
	}
	key, _ := s.key(p)
	return &s3Object{ctx: ctx, s: s, key: key, info: info.(*objectInfo)}, nil
}

func (s *S3Storage) Save(ctx context.Context, p string, r io.Reader, size int64) (int64, error) {
	key, err := s.key(p)
	if err != nil {
		return 0, err
	}
	// PutObject requires Content-Length, buffer the content of unknown size in a temporary file.
	if size < 0 {
		tmp, err := os.CreateTemp("", "sayrud-upload-*")
		if err != nil {
			return 0, errors.Wrap(err, "create temporary file")
		}
		defer func() {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}()
		if size, err = io.Copy(tmp, &ctxReader{ctx: ctx, r: r}); err != nil {
			return 0, errors.Wrap(err, "buffer upload")
		}
		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			return 0, errors.Wrap(err, "rewind temporary file")
		}
		r = tmp
	}

	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          r,
		ContentLength: aws.Int64(size),
	}
	if contentType := mime.TypeByExtension(strings.ToLower(path.Ext(key))); contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	// UNSIGNED-PAYLOAD lets unseekable streams such as request bodies upload without hashing first, a length mismatch fails in the HTTP transport.
	_, err = s.client.PutObject(ctx, input, s3.WithAPIOptions(v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware))
	if err != nil {
		return 0, errors.Wrap(err, "put object")
	}
	return size, nil
}

func (s *S3Storage) Stat(ctx context.Context, p string) (os.FileInfo, error) {
	key, err := s.key(p)
	if err != nil {
		return nil, err
	}
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return nil, notExist("stat", p)
		}
		return nil, errors.Wrap(err, "head object")
	}
	return &objectInfo{name: path.Base(key), size: aws.ToInt64(out.ContentLength), modTime: aws.ToTime(out.LastModified)}, nil
}

func (s *S3Storage) Delete(ctx context.Context, p string) error {
	key, err := s.key(p)
	if err != nil {
		return err
	}
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}); err != nil && !isNotFound(err) {
		return errors.Wrap(err, "delete object")
	}
	return nil
}

func (s *S3Storage) URL(ctx context.Context, p, name string) (*url.URL, error) {
	key, err := s.key(p)
	if err != nil {
		return nil, err
	}
	if s.publicURL != nil {
		return s.publicURL.JoinPath(key), nil
	}
	contentType, disposition := ResponseHeaders(key, name)
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(key),
		ResponseContentType:        aws.String(contentType),
		ResponseContentDisposition: aws.String(disposition),
	}, s3.WithPresignExpires(s.expiry))
	if err != nil {
		return nil, errors.Wrap(err, "presign get object")
	}
	return url.Parse(req.URL)
}

func (s *S3Storage) IterateObjects(ctx context.Context, prefix string, fn func(path string, obj Object) error) error {
	prefix, err := cleanPrefix(prefix)
	if err != nil {
		return err
	}
	pages := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(s.basePath + prefix),
	})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return errors.Wrap(err, "list objects")
		}
		for _, item := range page.Contents {
			key := aws.ToString(item.Key)
			if strings.HasSuffix(key, "/") {
				continue
			}
			obj := &s3Object{ctx: ctx, s: s, key: key, info: &objectInfo{
				name:    path.Base(key),
				size:    aws.ToInt64(item.Size),
				modTime: aws.ToTime(item.LastModified),
			}}
			err := fn(strings.TrimPrefix(key, s.basePath), obj)
			_ = obj.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func isNotFound(err error) bool {
	var re interface{ HTTPStatusCode() int }
	return errors.As(err, &re) && re.HTTPStatusCode() == http.StatusNotFound
}

// s3Object sends a ranged GET from the current offset on the first Read, and again after Seek.
type s3Object struct {
	ctx    context.Context
	s      *S3Storage
	key    string
	info   *objectInfo
	offset int64
	body   io.ReadCloser
}

func (o *s3Object) Read(p []byte) (int, error) {
	if o.offset >= o.info.size {
		return 0, io.EOF
	}
	if o.body == nil {
		out, err := o.s.client.GetObject(o.ctx, &s3.GetObjectInput{
			Bucket: aws.String(o.s.bucket),
			Key:    aws.String(o.key),
			Range:  aws.String(fmt.Sprintf("bytes=%d-", o.offset)),
		})
		if err != nil {
			if isNotFound(err) {
				return 0, notExist("read", o.key)
			}
			return 0, errors.Wrap(err, "get object")
		}
		o.body = out.Body
	}
	n, err := o.body.Read(p)
	o.offset += int64(n)
	return n, err
}

func (o *s3Object) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = o.offset + offset
	case io.SeekEnd:
		abs = o.info.size + offset
	default:
		return 0, errors.New("invalid whence")
	}
	if abs < 0 {
		return 0, errors.New("negative position")
	}
	if abs != o.offset && o.body != nil {
		_ = o.body.Close()
		o.body = nil
	}
	o.offset = abs
	return abs, nil
}

func (o *s3Object) Close() error {
	if o.body == nil {
		return nil
	}
	err := o.body.Close()
	o.body = nil
	return err
}

func (o *s3Object) Stat() (os.FileInfo, error) {
	return o.info, nil
}
