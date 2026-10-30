package upload_test

import (
	"bytes"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/stretchr/testify/require"
)

func TestParseS3Path(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		path       string
		wantBucket string
		wantKey    string
		wantErr    bool
	}{
		{name: "public key", path: "s3://uploads/static/abc.pdf", wantBucket: "uploads", wantKey: "static/abc.pdf"},
		{name: "private key", path: "s3://uploads/static/.private/edo/abc.pdf", wantBucket: "uploads", wantKey: "static/.private/edo/abc.pdf"},
		{name: "filesystem path", path: "static/abc.pdf", wantErr: true},
		{name: "missing key", path: "s3://uploads", wantErr: true},
		{name: "empty key", path: "s3://uploads/", wantErr: true},
		{name: "missing bucket", path: "s3:///static/abc.pdf", wantErr: true},
		{name: "traversal", path: "s3://uploads/static/../secret.pdf", wantErr: true},
		{name: "double slash", path: "s3://uploads/static//abc.pdf", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			bucket, key, err := upload.ParseS3Path(tt.path)
			if tt.wantErr {
				require.ErrorIs(t, err, upload.ErrInvalidS3Path)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantBucket, bucket)
			require.Equal(t, tt.wantKey, key)
			require.Equal(t, tt.path, upload.S3Path(bucket, key))
			require.Equal(t, tt.wantKey, upload.LayoutPath(tt.path))
		})
	}
}

func TestLocatePath(t *testing.T) {
	t.Parallel()
	require.Equal(t, "static/a.pdf", upload.LocatePath("", "static/a.pdf"))
	require.Equal(t, "s3://uploads/static/a.pdf", upload.LocatePath("s3://uploads", "static/a.pdf"))
	require.Equal(t, "s3://uploads/var/uploads/a.pdf", upload.LocatePath("s3://uploads/", "/var/uploads/a.pdf"))
}

func TestCreateDTO_StorageLocationMirrorsFilesystemLayout(t *testing.T) {
	t.Parallel()
	content := []byte("%PDF-1.7\nlocated")

	local, _, err := (&upload.CreateDTO{File: bytes.NewReader(content), Name: "a.pdf", Size: len(content), Slug: "slug"}).ToEntity()
	require.NoError(t, err)
	require.Equal(t, "static/slug.pdf", local.Path())
	require.Equal(t, "/static/slug.pdf", local.PreviewURL())

	located, _, err := (&upload.CreateDTO{File: bytes.NewReader(content), Name: "a.pdf", Size: len(content), Slug: "slug", StorageLocation: "s3://uploads"}).ToEntity()
	require.NoError(t, err)
	require.Equal(t, "s3://uploads/static/slug.pdf", located.Path())

	located.SetID(42)
	require.Equal(t, "/uploads/42", located.PreviewURL())
	require.Equal(t, "http://localhost/uploads/42", located.URL().String())

	located.SetSlug("renamed")
	require.Equal(t, "s3://uploads/static/renamed.pdf", located.Path())
}
