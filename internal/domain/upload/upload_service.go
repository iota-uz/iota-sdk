package upload

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/infrastracture/event"
)

type Service struct {
	repo           Repository
	eventPublisher *event.Publisher
}

func NewService(repo Repository, publisher *event.Publisher) *Service {
	return &Service{
		repo:           repo,
		eventPublisher: publisher,
	}
}

func (s *Service) GetUploadsCount(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *Service) GetUploads(ctx context.Context) ([]*Upload, error) {
	return s.repo.GetAll(ctx)
}

func (s *Service) GetUploadByID(ctx context.Context, id int64) (*Upload, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetUploadsPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Upload, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *Service) CreateUpload(ctx context.Context, upload *Upload) error {
	if err := s.repo.Create(ctx, upload); err != nil {
		return err
	}
	s.eventPublisher.Publish("upload.created", upload)
	return nil
}

func (s *Service) UpdateUpload(ctx context.Context, upload *Upload) error {
	if err := s.repo.Update(ctx, upload); err != nil {
		return err
	}
	s.eventPublisher.Publish("upload.updated", upload)
	return nil
}

func (s *Service) DeleteUpload(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.eventPublisher.Publish("upload.deleted", id)
	return nil
}
