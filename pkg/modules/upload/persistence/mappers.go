package persistence

import (
	"github.com/iota-agency/iota-sdk/pkg/modules/upload/domain/entities/upload"
	"github.com/iota-agency/iota-sdk/pkg/modules/upload/persistence/models"
)

func toDBUpload(upload *upload.Upload) *models.Upload {
	return &models.Upload{
		ID:        upload.ID,
		URL:       upload.URL,
		Name:      upload.Name,
		Type:      upload.Type,
		Size:      upload.Size,
		CreatedAt: upload.CreatedAt,
		UpdatedAt: upload.UpdatedAt,
	}
}

func toDomainUpload(dbUpload *models.Upload) (*upload.Upload, error) {
	return &upload.Upload{
		ID:        dbUpload.ID,
		Type:      dbUpload.Type,
		URL:       dbUpload.URL,
		Size:      dbUpload.Size,
		Name:      dbUpload.Name,
		CreatedAt: dbUpload.CreatedAt,
		UpdatedAt: dbUpload.UpdatedAt,
	}, nil
}
