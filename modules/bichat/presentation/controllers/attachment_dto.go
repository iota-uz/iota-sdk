package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// AttachmentUploadDTO is the transport shape accepted by stream/chat endpoints.
// It supports both:
//   - upload path (base64Data present)
//   - persisted reference path (url/filePath present)
type AttachmentUploadDTO struct {
	Filename   string `json:"filename"`
	MimeType   string `json:"mimeType,omitempty"`
	SizeBytes  *int64 `json:"sizeBytes"`
	Base64Data string `json:"base64Data,omitempty"`
	URL        string `json:"url,omitempty"`
	FilePath   string `json:"filePath,omitempty"` // backward compatibility alias
}

func convertAttachmentDTOs(
	ctx context.Context,
	attachmentService bichatservices.AttachmentService,
	uploads []AttachmentUploadDTO,
	tenantID uuid.UUID,
	userID uuid.UUID,
) ([]domain.Attachment, error) {
	const op serrors.Op = "controllers.convertAttachmentDTOs"

	if len(uploads) == 0 {
		return nil, nil
	}
	if attachmentService == nil {
		return nil, serrors.E(op, serrors.Internal, "attachment service is not configured")
	}

	result := make([]domain.Attachment, 0, len(uploads))
	logger := configuration.Use().Logger()

	for i, upload := range uploads {
		filename := strings.TrimSpace(upload.Filename)
		if filename == "" {
			return nil, serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachments[%d].filename is required", i))
		}

		if upload.SizeBytes == nil {
			return nil, serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachments[%d].sizeBytes is required", i))
		}

		if *upload.SizeBytes < 0 {
			return nil, serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachments[%d].sizeBytes must be non-negative", i))
		}

		base64Payload := strings.TrimSpace(upload.Base64Data)
		if base64Payload != "" {
			data, err := base64.StdEncoding.DecodeString(base64Payload)
			if err != nil {
				return nil, serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachments[%d].base64Data is invalid", i))
			}

			authoritativeSize := int64(len(data))
			if *upload.SizeBytes != 0 && *upload.SizeBytes != authoritativeSize {
				logger.WithFields(map[string]interface{}{
					"index":           i,
					"filename":        filename,
					"declared_size":   *upload.SizeBytes,
					"decoded_size":    authoritativeSize,
					"controller_path": "bichat.attachments",
				}).Warn("Attachment size mismatch; using decoded size")
			}

			attachment, err := attachmentService.ValidateAndSave(
				ctx,
				filename,
				upload.MimeType,
				authoritativeSize,
				bytes.NewReader(data),
				tenantID,
				userID,
			)
			if err != nil {
				return nil, serrors.E(op, err)
			}

			result = append(result, attachment)
			continue
		}

		persistedURL := strings.TrimSpace(upload.URL)
		if persistedURL == "" {
			persistedURL = strings.TrimSpace(upload.FilePath)
		}
		if persistedURL == "" {
			return nil, serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachments[%d].base64Data is required for upload", i))
		}

		result = append(result, domain.NewAttachment(
			domain.WithFileName(filename),
			domain.WithMimeType(strings.TrimSpace(upload.MimeType)),
			domain.WithSizeBytes(*upload.SizeBytes),
			domain.WithFilePath(persistedURL),
		))
	}

	return result, nil
}
