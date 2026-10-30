package uploadsconfig_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/env"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/stretchr/testify/require"
)

func TestStorageConfig_EnvNamesMapToFields(t *testing.T) {
	t.Parallel()
	dotenv := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(dotenv, []byte(`UPLOAD_STORAGE_BACKEND=S3
UPLOAD_S3_ENDPOINT=minio.internal:9000
UPLOAD_S3_BUCKET=granite-uploads
UPLOAD_S3_REGION=eu-central-1
UPLOAD_S3_ACCESS_KEY=access
UPLOAD_S3_SECRET_KEY=secret
UPLOAD_S3_SECURE=true
UPLOAD_S3_FORCE_PATH_STYLE=false
UPLOAD_S3_PRESIGN_TTL_SECONDS=60
`), 0o600))
	src, err := config.Build(env.New(dotenv))
	require.NoError(t, err)

	cfg, err := config.Register[uploadsconfig.StorageConfig](config.NewRegistry(src))
	require.NoError(t, err)

	require.Equal(t, uploadsconfig.BackendS3, cfg.Backend())
	require.Equal(t, "minio.internal:9000", cfg.S3.Endpoint)
	require.Equal(t, "granite-uploads", cfg.S3.Bucket)
	require.Equal(t, "eu-central-1", cfg.S3.Region)
	require.Equal(t, "access", cfg.S3.Access.Key)
	require.Equal(t, "secret", cfg.S3.Secret.Key)
	require.True(t, cfg.S3.Secure)
	require.False(t, cfg.S3.ForcePathStyle())
	require.Equal(t, time.Minute, cfg.S3.PresignTTL())
	require.True(t, cfg.S3Configured())
}

func TestStorageConfig_DefaultsKeepFilesystem(t *testing.T) {
	t.Parallel()
	cfg, err := config.Register[uploadsconfig.StorageConfig](config.NewRegistry(buildSource(t, nil)))
	require.NoError(t, err)

	require.Equal(t, uploadsconfig.BackendFS, cfg.Backend())
	require.False(t, cfg.S3Configured())
	require.True(t, cfg.S3.ForcePathStyle())
	require.Equal(t, "us-east-1", cfg.S3.Region)
	require.Equal(t, 5*time.Minute, cfg.S3.PresignTTL())
}

func TestStorageConfig_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		values  map[string]any
		wantErr string
	}{
		{name: "fs without s3", values: map[string]any{"upload.storage.backend": "fs"}},
		{name: "fs with s3 reads", values: map[string]any{
			"upload.storage.backend": "fs", "upload.s3.bucket": "b", "upload.s3.access.key": "a", "upload.s3.secret.key": "s",
		}},
		{name: "s3 primary", values: map[string]any{
			"upload.storage.backend": "s3", "upload.s3.bucket": "b", "upload.s3.access.key": "a", "upload.s3.secret.key": "s",
		}},
		{name: "s3 primary without settings", values: map[string]any{"upload.storage.backend": "s3", "upload.s3.bucket": "b"}, wantErr: "UPLOAD_S3_ACCESS_KEY, UPLOAD_S3_SECRET_KEY"},
		{name: "fs with partial s3", values: map[string]any{
			"upload.storage.backend": "fs", "upload.s3.bucket": "b", "upload.s3.endpoint": "minio:9000",
		}, wantErr: "partial S3 upload configuration: missing UPLOAD_S3_ACCESS_KEY, UPLOAD_S3_SECRET_KEY"},
		{name: "fs with s3 endpoint only", values: map[string]any{"upload.storage.backend": "fs", "upload.s3.endpoint": "minio:9000"}},
		{name: "unknown backend", values: map[string]any{"upload.storage.backend": "gcs"}, wantErr: "unsupported UPLOAD_STORAGE_BACKEND"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Register[uploadsconfig.StorageConfig](config.NewRegistry(buildSource(t, tt.values)))
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
