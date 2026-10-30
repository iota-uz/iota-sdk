package persistence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gabriel-vasile/mimetype"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// S3API is the subset of the S3 client used by S3Storage.
type S3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

// S3Presigner is the subset of the S3 presign client used by S3Storage.
type S3Presigner interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

// S3Storage stores uploads in one private S3-compatible bucket and addresses
// objects by "s3://<bucket>/<key>" paths.
type S3Storage struct {
	bucket  string
	client  S3API
	presign S3Presigner
}

// NewS3Storage connects to the bucket described by cfg.
func NewS3Storage(cfg uploadsconfig.S3Config) (*S3Storage, error) {
	const op serrors.Op = "persistence.NewS3Storage"
	if !cfg.IsConfigured() {
		return nil, serrors.Wrap(op, fmt.Errorf("s3 upload storage requires %s", cfg.MissingFields()))
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}
	endpoint, err := s3Endpoint(cfg.Endpoint, cfg.Secure)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	client := s3.NewFromConfig(aws.Config{
		Region:                     region,
		Credentials:                credentials.NewStaticCredentialsProvider(strings.TrimSpace(cfg.Access.Key), strings.TrimSpace(cfg.Secret.Key), ""),
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	}, func(o *s3.Options) {
		o.UsePathStyle = cfg.ForcePathStyle()
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	return NewS3StorageWithClient(strings.TrimSpace(cfg.Bucket), client, s3.NewPresignClient(client)), nil
}

// NewS3StorageWithClient builds an S3Storage over an existing client, e.g. a test double.
func NewS3StorageWithClient(bucket string, client S3API, presign S3Presigner) *S3Storage {
	return &S3Storage{bucket: bucket, client: client, presign: presign}
}

// Bucket returns the bucket new objects are written to.
func (s *S3Storage) Bucket() string { return s.bucket }

// StorageLocation implements upload.LocatingStorage.
func (s *S3Storage) StorageLocation() string { return upload.S3PathScheme + s.bucket }

func (s *S3Storage) Open(ctx context.Context, fileName string) ([]byte, error) {
	const op serrors.Op = "persistence.S3Storage.Open"
	body, _, err := s.OpenReader(ctx, fileName)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return data, nil
}

func (s *S3Storage) OpenReader(ctx context.Context, fileName string) (io.ReadCloser, upload.ObjectInfo, error) {
	const op serrors.Op = "persistence.S3Storage.OpenReader"
	key, err := s.key(fileName)
	if err != nil {
		return nil, upload.ObjectInfo{}, serrors.Wrap(op, err)
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		if isS3NotFound(err) {
			err = fmt.Errorf("%w: %w", fs.ErrNotExist, err)
		}
		return nil, upload.ObjectInfo{}, serrors.Wrap(op, err)
	}
	return out.Body, upload.ObjectInfo{
		Size:        aws.ToInt64(out.ContentLength),
		ContentType: aws.ToString(out.ContentType),
		ModTime:     aws.ToTime(out.LastModified),
	}, nil
}

func (s *S3Storage) Save(ctx context.Context, fileName string, data []byte) error {
	const op serrors.Op = "persistence.S3Storage.Save"
	key, err := s.key(fileName)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String(mimetype.Detect(data).String()),
	})
	if err != nil {
		return serrors.Wrap(op, err)
	}
	return nil
}

func (s *S3Storage) Rename(ctx context.Context, oldPath, newPath string) error {
	const op serrors.Op = "persistence.S3Storage.Rename"
	sourceKey, err := s.key(oldPath)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	targetKey, err := s.key(newPath)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if sourceKey == targetKey {
		return nil
	}
	_, err = s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(s.bucket + "/" + (&url.URL{Path: sourceKey}).EscapedPath()),
		Key:        aws.String(targetKey),
	})
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if err := s.Delete(ctx, oldPath); err != nil {
		return serrors.Wrap(op, err)
	}
	return nil
}

func (s *S3Storage) Delete(ctx context.Context, fileName string) error {
	const op serrors.Op = "persistence.S3Storage.Delete"
	key, err := s.key(fileName)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}); err != nil && !isS3NotFound(err) {
		return serrors.Wrap(op, err)
	}
	return nil
}

func (s *S3Storage) PresignGetURL(ctx context.Context, fileName string, ttl time.Duration) (string, error) {
	const op serrors.Op = "persistence.S3Storage.PresignGetURL"
	key, err := s.key(fileName)
	if err != nil {
		return "", serrors.Wrap(op, err)
	}
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}, func(o *s3.PresignOptions) {
		o.Expires = ttl
	})
	if err != nil {
		return "", serrors.Wrap(op, err)
	}
	return req.URL, nil
}

// key accepts only "s3://<bucket>/<key>" paths for the configured bucket.
func (s *S3Storage) key(fileName string) (string, error) {
	bucket, key, err := upload.ParseS3Path(fileName)
	if err != nil {
		return "", fmt.Errorf("%w: %q", err, fileName)
	}
	if bucket != s.bucket {
		return "", fmt.Errorf("%w: bucket %q is not the configured upload bucket", upload.ErrInvalidS3Path, bucket)
	}
	return key, nil
}

func s3Endpoint(endpoint string, secure bool) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", nil
	}
	if !strings.Contains(endpoint, "://") {
		scheme := "http://"
		if secure {
			scheme = "https://"
		}
		endpoint = scheme + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("invalid UPLOAD_S3_ENDPOINT %q", endpoint)
	}
	return endpoint, nil
}

// isS3NotFound reports a missing object. A missing bucket is a configuration
// error and must not read as a missing file.
func isS3NotFound(err error) bool {
	var noSuchBucket *s3types.NoSuchBucket
	if errors.As(err, &noSuchBucket) {
		return false
	}
	var codeErr interface{ ErrorCode() string }
	if errors.As(err, &codeErr) && codeErr.ErrorCode() == "NoSuchBucket" {
		return false
	}
	var noSuchKey *s3types.NoSuchKey
	var notFound *s3types.NotFound
	if errors.As(err, &noSuchKey) || errors.As(err, &notFound) {
		return true
	}
	var apiErr interface{ ErrorCode() string }
	if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound") {
		return true
	}
	var respErr interface{ HTTPStatusCode() int }
	return errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound
}
