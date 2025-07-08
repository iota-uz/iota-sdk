package controllers

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	importpkg "github.com/iota-uz/iota-sdk/pkg/import"
)

// UploadServiceAdapter adapts the core upload service to the import interface
type UploadServiceAdapter struct {
	coreService *coreservices.UploadService
}

// NewUploadServiceAdapter creates a new adapter
func NewUploadServiceAdapter(coreService *coreservices.UploadService) *UploadServiceAdapter {
	return &UploadServiceAdapter{
		coreService: coreService,
	}
}

// GetByID implements importpkg.UploadService
func (a *UploadServiceAdapter) GetByID(ctx context.Context, id uint) (importpkg.UploadFile, error) {
	upload, err := a.coreService.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &UploadFileAdapter{upload: upload}, nil
}

// UploadFileAdapter adapts upload.Upload to importpkg.UploadFile
type UploadFileAdapter struct {
	upload upload.Upload
}

// Path implements importpkg.UploadFile
func (a *UploadFileAdapter) Path() string {
	return a.upload.Path()
}
