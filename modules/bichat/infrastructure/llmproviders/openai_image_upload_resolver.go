package llmproviders

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	coreupload "github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	openai "github.com/openai/openai-go/v3"
)

// OpenAIImageUploadRecord contains upload metadata needed by input mapper.
type OpenAIImageUploadRecord struct {
	Name     string
	MimeType string
	Path     string
}

// OpenAIImageUploadLookup resolves upload-backed image metadata by upload ID.
type OpenAIImageUploadLookup interface {
	FindImageUpload(ctx context.Context, uploadID int64) (*OpenAIImageUploadRecord, error)
}

type coreUploadByIDs interface {
	GetByIDs(ctx context.Context, ids []uint) ([]coreupload.Upload, error)
}

type coreOpenAIImageUploadLookup struct {
	repo coreUploadByIDs
}

func newCoreOpenAIImageUploadLookup() OpenAIImageUploadLookup {
	return &coreOpenAIImageUploadLookup{repo: corepersistence.NewUploadRepository()}
}

func (l *coreOpenAIImageUploadLookup) FindImageUpload(ctx context.Context, uploadID int64) (*OpenAIImageUploadRecord, error) {
	if uploadID <= 0 {
		return nil, fmt.Errorf("upload id must be positive")
	}
	uploads, err := l.repo.GetByIDs(ctx, []uint{uint(uploadID)})
	if err != nil {
		return nil, err
	}
	if len(uploads) == 0 {
		return nil, nil
	}

	entity := uploads[0]
	record := &OpenAIImageUploadRecord{
		Name: strings.TrimSpace(entity.Name()),
		Path: strings.TrimSpace(entity.Path()),
	}
	if entity.Mimetype() != nil {
		record.MimeType = strings.TrimSpace(entity.Mimetype().String())
	}

	return record, nil
}

func (m *OpenAIModel) resolveImageUploadFileID(
	ctx context.Context,
	uploadID int64,
	fallbackName string,
	fallbackMimeType string,
) string {
	if m == nil || m.client == nil || uploadID <= 0 {
		return ""
	}

	m.mu.RLock()
	lookup := m.imageUploadResolver
	m.mu.RUnlock()
	if lookup == nil {
		return ""
	}

	record, err := lookup.FindImageUpload(ctx, uploadID)
	if err != nil {
		m.logger.Warn(ctx, "failed to resolve image upload for OpenAI input_file", map[string]any{
			"upload_id": uploadID,
			"error":     err.Error(),
		})
		return ""
	}
	if record == nil {
		m.logger.Warn(ctx, "image upload id not found for OpenAI input_file", map[string]any{
			"upload_id": uploadID,
		})
		return ""
	}

	filename := strings.TrimSpace(record.Name)
	if filename == "" {
		filename = strings.TrimSpace(fallbackName)
	}
	if filename == "" {
		filename = "image"
	}

	mimeType := strings.TrimSpace(record.MimeType)
	if mimeType == "" {
		mimeType = strings.TrimSpace(fallbackMimeType)
	}

	data, readErr := os.ReadFile(record.Path)
	if readErr != nil {
		m.logger.Warn(ctx, "failed to read upload-backed image for OpenAI input_file", map[string]any{
			"upload_id": uploadID,
			"path":      record.Path,
			"error":     readErr.Error(),
		})
		return ""
	}
	if len(data) == 0 {
		return ""
	}

	uploaded, uploadErr := m.client.Files.New(ctx, openai.FileNewParams{
		File:    openai.File(bytes.NewReader(data), filename, mimeType),
		Purpose: openai.FilePurposeVision,
	})
	if uploadErr != nil {
		m.logger.Warn(ctx, "failed to upload image to OpenAI files for input_image.file_id", map[string]any{
			"upload_id": uploadID,
			"filename":  filename,
			"mime_type": mimeType,
			"error":     uploadErr.Error(),
		})
		return ""
	}

	return strings.TrimSpace(uploaded.ID)
}

func isLikelyInaccessibleImageURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed == nil {
		return false
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return false
	}

	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}
