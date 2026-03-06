package persistence

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type s3Client interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type s3PresignClient interface {
	PresignGetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*v4.PresignedHTTPRequest, error)
}

type S3Storage struct {
	bucket  string
	client  s3Client
	presign s3PresignClient
}

func NewS3Storage() (*S3Storage, error) {
	const op serrors.Op = "persistence.NewS3Storage"

	conf := configuration.Use()

	bucket := strings.TrimSpace(conf.UploadS3Bucket)
	region := strings.TrimSpace(conf.UploadS3Region)
	endpoint := normalizeS3Endpoint(strings.TrimSpace(conf.UploadS3Endpoint), conf.UploadS3Secure)
	accessKey := strings.TrimSpace(conf.UploadS3AccessKey)
	secretKey := strings.TrimSpace(conf.UploadS3SecretKey)

	if bucket == "" {
		return nil, serrors.E(op, serrors.Invalid, "UPLOAD_S3_BUCKET is required")
	}
	if region == "" {
		region = "us-east-1"
	}
	if accessKey == "" || secretKey == "" {
		return nil, serrors.E(op, serrors.Invalid, "UPLOAD_S3_ACCESS_KEY and UPLOAD_S3_SECRET_KEY are required")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return nil, serrors.E(op, fmt.Errorf("load aws config: %w", err))
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = conf.UploadS3ForcePathStyle
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})

	return &S3Storage{
		bucket:  bucket,
		client:  client,
		presign: s3.NewPresignClient(client),
	}, nil
}

func (s *S3Storage) Open(ctx context.Context, fileName string) ([]byte, error) {
	const op serrors.Op = "persistence.S3Storage.Open"
	key, err := normalizeS3Key(fileName)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	res, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return body, nil
}

func (s *S3Storage) Save(ctx context.Context, fileName string, b []byte) error {
	const op serrors.Op = "persistence.S3Storage.Save"
	key, err := normalizeS3Key(fileName)
	if err != nil {
		return serrors.E(op, err)
	}
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(b),
	})
	if err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *S3Storage) Rename(ctx context.Context, oldPath, newPath string) error {
	const op serrors.Op = "persistence.S3Storage.Rename"
	sourceKey, err := normalizeS3Key(oldPath)
	if err != nil {
		return serrors.E(op, err)
	}
	targetKey, err := normalizeS3Key(newPath)
	if err != nil {
		return serrors.E(op, err)
	}

	_, err = s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(s.bucket + "/" + sourceKey),
		Key:        aws.String(targetKey),
	})
	if err != nil {
		return serrors.E(op, err)
	}
	if err := s.Delete(ctx, sourceKey); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *S3Storage) Delete(ctx context.Context, fileName string) error {
	const op serrors.Op = "persistence.S3Storage.Delete"
	key, err := normalizeS3Key(fileName)
	if err != nil {
		return serrors.E(op, err)
	}
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *S3Storage) PresignGetURL(ctx context.Context, fileName string, ttl time.Duration) (string, error) {
	const op serrors.Op = "persistence.S3Storage.PresignGetURL"
	key, err := normalizeS3Key(fileName)
	if err != nil {
		return "", serrors.E(op, err)
	}
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(po *s3.PresignOptions) {
		po.Expires = ttl
	})
	if err != nil {
		return "", serrors.E(op, err)
	}
	return req.URL, nil
}

func normalizeS3Key(fileName string) (string, error) {
	cleaned := strings.TrimSpace(fileName)
	cleaned = strings.TrimPrefix(cleaned, "/")
	cleaned = path.Clean("/" + cleaned)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("invalid s3 object key for %q", fileName)
	}
	return cleaned, nil
}

func normalizeS3Endpoint(endpoint string, secure bool) string {
	if endpoint == "" {
		return ""
	}
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	scheme := "http://"
	if secure {
		scheme = "https://"
	}
	return scheme + endpoint
}
