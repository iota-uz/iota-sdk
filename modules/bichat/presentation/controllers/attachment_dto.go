package controllers

import (
	"context"
	"fmt"

	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const maxAttachmentCount = 10

// AttachmentUploadDTO is the upload-reference shape accepted by stream/chat endpoints.
type AttachmentUploadDTO struct {
	UploadID *int64 `json:"uploadId"`
}

func convertAttachmentDTOs(ctx context.Context, uploads []AttachmentUploadDTO) ([]domain.Attachment, error) {
	const op serrors.Op = "controllers.convertAttachmentDTOs"

	if len(uploads) == 0 {
		return nil, nil
	}
	if len(uploads) > maxAttachmentCount {
		return nil, serrors.E(op, serrors.KindValidation, fmt.Sprintf("too many attachments: %d (max: %d)", len(uploads), maxAttachmentCount))
	}

	uploadRepo := corepersistence.NewUploadRepository()
	result := make([]domain.Attachment, 0, len(uploads))

	for i, uploadRef := range uploads {
		if uploadRef.UploadID == nil || *uploadRef.UploadID <= 0 {
			return nil, serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachments[%d].uploadId is required", i))
		}

		found, err := uploadRepo.GetByIDs(ctx, []uint{uint(*uploadRef.UploadID)})
		if err != nil {
			return nil, serrors.E(op, err)
		}
		if len(found) == 0 {
			return nil, serrors.E(op, serrors.KindValidation, fmt.Sprintf("attachments[%d].uploadId not found", i))
		}

		entity := found[0]
		mimeType := ""
		if entity.Mimetype() != nil {
			mimeType = entity.Mimetype().String()
		}
		result = append(result, domain.NewAttachment(
			domain.WithUploadID(int64(entity.ID())),
			domain.WithFileName(entity.Name()),
			domain.WithMimeType(mimeType),
			domain.WithSizeBytes(int64(entity.Size().Bytes())),
			domain.WithFilePath(entity.URL().String()),
		))
	}

	return result, nil
}
