package services_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/openai"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type fakeUploads struct {
	byID  map[uint]upload.Upload
	files map[string][]byte
	seen  uuid.UUID
}

func (f *fakeUploads) GetByID(ctx context.Context, id uint) (upload.Upload, error) {
	f.seen, _ = composables.UseTenantID(ctx)
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, services.ErrUploadNotFound
}

func (f *fakeUploads) OpenReader(_ context.Context, path string) (io.ReadCloser, upload.ObjectInfo, error) {
	data, ok := f.files[path]
	if !ok {
		return nil, upload.ObjectInfo{}, errors.New("no file")
	}
	return io.NopCloser(bytes.NewReader(data)), upload.ObjectInfo{}, nil
}

func newUpload(id uint, tenant uuid.UUID, path string, size int, mt string) upload.Upload {
	return upload.NewWithID(id, tenant, "hash", path, "pic.png", "slug", size, mimetype.Lookup(mt), upload.UploadTypeImage, time.Now(), time.Now())
}

func TestUploadImages(t *testing.T) {
	tenant, other := uuid.New(), uuid.New()
	png := []byte("\x89PNG\r\n\x1a\nrest")
	f := &fakeUploads{
		byID: map[uint]upload.Upload{
			7:  newUpload(7, tenant, "a/7.png", len(png), "image/png"),
			8:  newUpload(8, other, "a/8.png", len(png), "image/png"),
			9:  newUpload(9, tenant, "a/9.png", 100, "image/png"),
			10: newUpload(10, tenant, "a/10.png", 4, "image/png"), // the size on record is smaller than the file
		},
		files: map[string][]byte{"a/7.png": png, "a/8.png": png, "a/9.png": bytes.Repeat([]byte{1}, 100), "a/10.png": bytes.Repeat([]byte{1}, 100)},
	}
	src := services.UploadImages{Uploads: func() (services.UploadReader, error) { return f, nil }}
	ctx := context.Background()

	img, err := src.Image(ctx, tenant, "7", 1024)
	require.NoError(t, err)
	assert.Equal(t, png, img.Data)
	assert.Equal(t, "image/png", img.MediaType)
	assert.Equal(t, tenant, f.seen, "the repository is asked as the tenant")

	_, err = src.Image(ctx, tenant, "8", 1024)
	require.ErrorIs(t, err, openai.ErrImageNotFound, "another tenant's upload is not found")
	_, err = src.Image(ctx, tenant, "404", 1024)
	require.ErrorIs(t, err, openai.ErrImageNotFound)
	for _, bad := range []string{"", "0", "-1", "abc", "1.5", "99999999999"} {
		_, err = src.Image(ctx, tenant, bad, 1024)
		require.ErrorIs(t, err, openai.ErrImageNotFound, bad)
	}
	_, err = src.Image(ctx, tenant, "9", 50)
	require.ErrorIs(t, err, openai.ErrImageTooLarge)
	_, err = src.Image(ctx, tenant, "10", 50)
	require.ErrorIs(t, err, openai.ErrImageTooLarge, "the file is read up to the limit, whatever the record says")

	none := services.UploadImages{}
	_, err = none.Image(ctx, tenant, "7", 1024)
	require.ErrorIs(t, err, openai.ErrImagesUnavailable)
	down := services.UploadImages{Uploads: func() (services.UploadReader, error) { return nil, errors.New("no uploads module") }}
	_, err = down.Image(ctx, tenant, "7", 1024)
	require.ErrorIs(t, err, openai.ErrImagesUnavailable)
}
