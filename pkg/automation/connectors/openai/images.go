package openai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/internal/aichat"
)

// Limits of the images param.
const (
	MaxImages     = 8
	MaxImageBytes = 5 << 20
)

// Image is the file of an upload.
type Image struct {
	// MediaType is the file's type; empty: it is read from the content.
	MediaType string
	Data      []byte
}

// ErrImageNotFound: the tenant has no such upload. ErrImageTooLarge: the file is over the limit passed to
// Image.
// ErrImagesUnavailable: this application does not serve uploads to automations.
var (
	ErrImageNotFound     = errors.New("openai: no such upload")
	ErrImageTooLarge     = errors.New("openai: the upload is too large")
	ErrImagesUnavailable = errors.New("openai: uploads are not available")
)

// ImageSource is where the uploads an ai.llm node's images param names are read from: the files of the
// tenant, by upload id. Image returns ErrImageNotFound for an id the tenant does not have (another
// tenant's id is not found) and ErrImageTooLarge for a file over maxBytes, without reading it whole.
type ImageSource interface {
	Image(ctx context.Context, tenantID uuid.UUID, uploadID string, maxBytes int64) (Image, error)
}

// imageTypes are the media types the provider takes.
var imageTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true}

// loadImages reads the uploads of a call. A failure is a CallError-able message.
func loadImages(ctx context.Context, src ImageSource, tenant uuid.UUID, ids []string) ([]aichat.Image, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > MaxImages {
		return nil, fmt.Errorf("at most %d images", MaxImages)
	}
	if src == nil {
		return nil, errors.New("images are not available: this application does not serve uploads to automations")
	}
	out := make([]aichat.Image, 0, len(ids))
	for _, id := range ids {
		img, err := src.Image(ctx, tenant, id, MaxImageBytes)
		switch {
		case errors.Is(err, ErrImagesUnavailable):
			return nil, errors.New("images are not available: this application does not serve uploads to automations")
		case errors.Is(err, ErrImageNotFound):
			return nil, fmt.Errorf("image %s was not found", id)
		case errors.Is(err, ErrImageTooLarge):
			return nil, fmt.Errorf("image %s is over %d MB", id, MaxImageBytes>>20)
		case err != nil:
			return nil, &readError{err: err}
		case len(img.Data) == 0:
			return nil, fmt.Errorf("image %s is empty", id)
		case len(img.Data) > MaxImageBytes:
			return nil, fmt.Errorf("image %s is over %d MB", id, MaxImageBytes>>20)
		}
		// The content decides, not the label: a file that is not an image of a supported format is refused, and
		// so is one whose label names another format.
		detected := http.DetectContentType(img.Data)
		if !imageTypes[detected] {
			return nil, fmt.Errorf("image %s is not a PNG, JPEG, WebP or GIF", id)
		}
		declared := img.MediaType
		if i := strings.Index(declared, ";"); i >= 0 {
			declared = declared[:i]
		}
		if declared = strings.ToLower(strings.TrimSpace(declared)); imageTypes[declared] && declared != detected {
			return nil, fmt.Errorf("image %s is labelled %s but is %s", id, declared, detected)
		}
		mt := detected
		out = append(out, aichat.Image{MediaType: mt, Data: img.Data})
	}
	return out, nil
}

// readError is a failure of the source itself, not of the upload: the engine repeats the call.
type readError struct{ err error }

func (e *readError) Error() string { return "the upload could not be read" }
func (e *readError) Unwrap() error { return e.err }
