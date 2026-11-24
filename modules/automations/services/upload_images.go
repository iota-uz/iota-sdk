package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/openai"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// UploadReader is what UploadImages reads of the uploads module (*services.UploadService of core).
type UploadReader interface {
	GetByID(ctx context.Context, id uint) (upload.Upload, error)
	OpenReader(ctx context.Context, storedPath string) (io.ReadCloser, upload.ObjectInfo, error)
}

// UploadImages serves the files of the tenant's uploads to the LLM node's images param
// (openai.ImageSource). An upload id is the numeric id of the uploads module; an upload of another tenant
// is not found, whatever the id.
type UploadImages struct {
	// Uploads resolves the reader when it is first needed; an error is "uploads are not available".
	Uploads func() (UploadReader, error)
}

var _ openai.ImageSource = UploadImages{}

// Image reads the upload of the tenant: at most maxBytes, or openai.ErrImageTooLarge.
func (u UploadImages) Image(ctx context.Context, tenantID uuid.UUID, uploadID string, maxBytes int64) (openai.Image, error) {
	if u.Uploads == nil {
		return openai.Image{}, openai.ErrImagesUnavailable
	}
	reader, err := u.Uploads()
	if err != nil || reader == nil {
		return openai.Image{}, openai.ErrImagesUnavailable
	}
	n, err := strconv.ParseUint(uploadID, 10, 32)
	if err != nil || n == 0 {
		return openai.Image{}, openai.ErrImageNotFound
	}
	ctx = composables.WithTenantID(ctx, tenantID)
	up, err := reader.GetByID(ctx, uint(n))
	switch {
	case errors.Is(err, persistence.ErrUploadNotFound):
		return openai.Image{}, openai.ErrImageNotFound
	case err != nil:
		return openai.Image{}, fmt.Errorf("read upload %d: %w", n, err)
	case up == nil || up.TenantID() != tenantID:
		// The repository finds an upload by id alone; the tenant's uploads are the ones it owns.
		return openai.Image{}, openai.ErrImageNotFound
	case int64(up.Size().Bytes()) > maxBytes:
		return openai.Image{}, openai.ErrImageTooLarge
	}
	body, _, err := reader.OpenReader(ctx, up.Path())
	if err != nil {
		return openai.Image{}, fmt.Errorf("open upload %d: %w", n, err)
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return openai.Image{}, fmt.Errorf("read upload %d: %w", n, err)
	}
	if int64(len(data)) > maxBytes {
		return openai.Image{}, openai.ErrImageTooLarge
	}
	img := openai.Image{Data: data}
	if mt := up.Mimetype(); mt != nil {
		img.MediaType = mt.String()
	}
	return img, nil
}
