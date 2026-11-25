package automations

import (
	"context"
	"errors"
	"io"

	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
)

// coreUploads is the core uploads service as the UploadReader of the LLM node's images: the persistence's
// not-found error becomes the reader's.
type coreUploads struct{ svc *coreservices.UploadService }

func (c coreUploads) GetByID(ctx context.Context, id uint) (upload.Upload, error) {
	u, err := c.svc.GetByID(ctx, id)
	if errors.Is(err, persistence.ErrUploadNotFound) {
		return nil, services.ErrUploadNotFound
	}
	return u, err
}

func (c coreUploads) OpenReader(ctx context.Context, storedPath string) (io.ReadCloser, upload.ObjectInfo, error) {
	return c.svc.OpenReader(ctx, storedPath)
}
