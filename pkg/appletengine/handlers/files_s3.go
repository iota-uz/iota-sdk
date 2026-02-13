package handlers

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type S3FilesConfig struct {
	Bucket          string
	Region          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
}

type s3API interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type S3FilesStore struct {
	pool   *pgxpool.Pool
	bucket string
	client s3API
}

func NewS3FilesStore(pool *pgxpool.Pool, cfg S3FilesConfig) (*S3FilesStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.AccessKeyID = strings.TrimSpace(cfg.AccessKeyID)
	cfg.SecretAccessKey = strings.TrimSpace(cfg.SecretAccessKey)
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("s3 region is required")
	}
	if cfg.AccessKeyID == "" {
		return nil, fmt.Errorf("s3 access key is required")
	}
	if cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("s3 secret key is required")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = cfg.ForcePathStyle
		if cfg.Endpoint != "" {
			options.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	return &S3FilesStore{
		pool:   pool,
		bucket: cfg.Bucket,
		client: client,
	}, nil
}

func (s *S3FilesStore) Store(ctx context.Context, name, contentType string, data []byte) (map[string]any, error) {
	tenantID, appletID, err := tenantAndAppletFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("s3 files.store: %w", err)
	}
	id := uuid.NewString()
	safeName := sanitizeFileName(name)
	if safeName == "" {
		safeName = "file.bin"
	}
	key := strings.Join([]string{tenantID, appletID, id + "-" + safeName}, "/")
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}); err != nil {
		return nil, fmt.Errorf("s3 files.store put object: %w", err)
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO applets.files(tenant_id, applet_id, file_id, file_name, content_type, size_bytes, storage_path)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at
	`, tenantID, appletID, id, safeName, contentType, len(data), key)
	var createdAt time.Time
	if err := row.Scan(&createdAt); err != nil {
		_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})
		return nil, fmt.Errorf("s3 files.store metadata insert: %w", err)
	}

	return map[string]any{
		"id":          id,
		"name":        safeName,
		"contentType": contentType,
		"size":        len(data),
		"path":        fmt.Sprintf("s3://%s/%s", s.bucket, key),
		"createdAt":   createdAt.UTC().Format(time.RFC3339Nano),
	}, nil
}

func (s *S3FilesStore) Get(ctx context.Context, id string) (map[string]any, bool, error) {
	tenantID, appletID, err := tenantAndAppletFromContext(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("s3 files.get: %w", err)
	}
	row := s.pool.QueryRow(ctx, `
		SELECT file_name, content_type, size_bytes, storage_path, created_at
		FROM applets.files
		WHERE tenant_id = $1 AND applet_id = $2 AND file_id = $3
	`, tenantID, appletID, id)
	var (
		fileName    string
		contentType string
		sizeBytes   int
		storagePath string
		createdAt   time.Time
	)
	if err := row.Scan(&fileName, &contentType, &sizeBytes, &storagePath, &createdAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("s3 files.get: %w", err)
	}
	return map[string]any{
		"id":          id,
		"name":        fileName,
		"contentType": contentType,
		"size":        sizeBytes,
		"path":        fmt.Sprintf("s3://%s/%s", s.bucket, storagePath),
		"createdAt":   createdAt.UTC().Format(time.RFC3339Nano),
	}, true, nil
}

func (s *S3FilesStore) Delete(ctx context.Context, id string) (bool, error) {
	tenantID, appletID, err := tenantAndAppletFromContext(ctx)
	if err != nil {
		return false, fmt.Errorf("s3 files.delete: %w", err)
	}
	row := s.pool.QueryRow(ctx, `
		SELECT storage_path
		FROM applets.files
		WHERE tenant_id = $1 AND applet_id = $2 AND file_id = $3
	`, tenantID, appletID, id)
	var storagePath string
	if err := row.Scan(&storagePath); err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("s3 files.delete select: %w", err)
	}

	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(storagePath),
	})
	if err != nil {
		if !strings.Contains(err.Error(), "NoSuchKey") && !strings.Contains(err.Error(), "NotFound") {
			return false, fmt.Errorf("s3 files.delete object: %w", err)
		}
	}
	commandTag, err := s.pool.Exec(ctx, `
		DELETE FROM applets.files
		WHERE tenant_id = $1 AND applet_id = $2 AND file_id = $3
	`, tenantID, appletID, id)
	if err != nil {
		return false, fmt.Errorf("s3 files.delete metadata: %w", err)
	}
	return commandTag.RowsAffected() > 0, nil
}
