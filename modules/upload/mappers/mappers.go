package mappers

import (
	"strconv"
	"time"

	"github.com/iota-agency/iota-sdk/modules/upload/domain/entities/upload"
	"github.com/iota-agency/iota-sdk/modules/upload/viewmodels"
)

func UploadToViewModel(entity *upload.Upload) *viewmodels.Upload {
	return &viewmodels.Upload{
		ID:        entity.ID,
		URL:       entity.URL,
		Name:      entity.Name,
		Type:      entity.Type,
		Size:      strconv.Itoa(entity.Size),
		CreatedAt: entity.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entity.UpdatedAt.Format(time.RFC3339),
	}
}
