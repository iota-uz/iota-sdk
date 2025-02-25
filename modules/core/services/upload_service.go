package services

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type UploadService struct {
	repo      upload.Repository
	storage   upload.Storage
	publisher eventbus.EventBus
}

func NewUploadService(
	repo upload.Repository,
	storage upload.Storage,
	publisher eventbus.EventBus,
) *UploadService {
	return &UploadService{
		repo:      repo,
		publisher: publisher,
		storage:   storage,
	}
}

func (s *UploadService) GetByID(ctx context.Context, id uint) (upload.Upload, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UploadService) GetByHash(ctx context.Context, hash string) (upload.Upload, error) {
	return s.repo.GetByHash(ctx, hash)
}

func (s *UploadService) GetAll(ctx context.Context) ([]upload.Upload, error) {
	return s.repo.GetAll(ctx)
}

func (s *UploadService) GetPaginated(ctx context.Context, params *upload.FindParams) ([]upload.Upload, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *UploadService) Create(ctx context.Context, data *upload.CreateDTO) (upload.Upload, error) {
	entity, bytes, err := data.ToEntity()
	if err != nil {
		return nil, err
	}
	up, err := s.repo.GetByHash(ctx, entity.Hash())
	if err != nil && !errors.Is(err, persistence.ErrUploadNotFound) {
		return nil, err
	}
	if up != nil {
		return up, nil
	}
	if err := s.storage.Save(ctx, entity.Path(), bytes); err != nil {
		return nil, err
	}
	createdEntity, err := s.repo.Create(ctx, entity)
	if err != nil {
		return nil, err
	}
	createdEvent, err := upload.NewCreatedEvent(ctx, *data, createdEntity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(createdEvent)
	return createdEntity, nil
}

func (s *UploadService) CreateMany(ctx context.Context, data []*upload.CreateDTO) ([]upload.Upload, error) {
	entities := make([]upload.Upload, 0, len(data))
	for _, d := range data {
		entity, err := s.Create(ctx, d)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}
	return entities, nil
}

func (s *UploadService) Delete(ctx context.Context, id uint) (upload.Upload, error) {
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := upload.NewDeletedEvent(ctx, entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
