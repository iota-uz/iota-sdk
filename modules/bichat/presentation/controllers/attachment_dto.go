package controllers

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const maxDecodedAttachmentBytes int64 = 20 * 1024 * 1024 // Keep in sync with attachment service limit.

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
			expectedDecodedSize := int64(base64.StdEncoding.DecodedLen(len(base64Payload)))
			if expectedDecodedSize > maxDecodedAttachmentBytes {
				return nil, serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachments[%d] exceeds max allowed size", i))
			}

			data, err := decodeBase64Bounded(base64Payload, maxDecodedAttachmentBytes)
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

func decodeBase64Bounded(payload string, maxBytes int64) ([]byte, error) {
	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(payload))
	data, err := io.ReadAll(io.LimitReader(decoder, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("decoded payload exceeds %d bytes", maxBytes)
	}
	return data, nil
}
