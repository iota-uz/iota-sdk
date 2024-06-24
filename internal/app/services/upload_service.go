package services

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/domain/upload"
	"io"
	"os"
	"path/filepath"
)

type UploadService struct {
	repo upload.Repository
	app  *Application
}

func NewUploadService(repo upload.Repository, app *Application) *UploadService {
	return &UploadService{
		repo: repo,
		app:  app,
	}
}

func (s *UploadService) GetUploadsCount(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *UploadService) GetUploads(ctx context.Context) ([]*upload.Upload, error) {
	return s.repo.GetAll(ctx)
}

func (s *UploadService) GetUploadByID(ctx context.Context, id int64) (*upload.Upload, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UploadService) GetUploadsPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*upload.Upload, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *UploadService) UploadFile(ctx context.Context, file io.ReadSeeker, upload *upload.Upload) error {
	// write file to disk
	conf := configuration.Use()
	fullPath := filepath.Join(conf.UploadsPath(), upload.Path)
	f, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, file); err != nil {
		return err
	}
	return s.CreateUpload(ctx, upload)
}

func (s *UploadService) CreateUpload(ctx context.Context, upload *upload.Upload) error {
	if err := s.repo.Create(ctx, upload); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("upload.created", upload)
	return nil
}

func (s *UploadService) UpdateUpload(ctx context.Context, upload *upload.Upload) error {
	if err := s.repo.Update(ctx, upload); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("upload.updated", upload)
	return nil
}

func (s *UploadService) DeleteUpload(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("upload.deleted", id)
	return nil
}
