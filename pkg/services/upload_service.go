package services

import (
	"context"
	"errors"

	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/upload"
	"github.com/iota-agency/iota-sdk/pkg/event"
	"gorm.io/gorm"
)

type UploadService struct {
	repo      upload.Repository
	storage   upload.Storage
	publisher event.Publisher
}

func NewUploadService(
	repo upload.Repository,
	storage upload.Storage,
	publisher event.Publisher,
) *UploadService {
	return &UploadService{
		repo:      repo,
		publisher: publisher,
		storage:   storage,
	}
}

func (s *UploadService) GetByID(ctx context.Context, id uint) (*upload.Upload, error) {
	if err := composables.CanUser(ctx, permission.UploadRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *UploadService) GetByHash(ctx context.Context, hash string) (*upload.Upload, error) {
	if err := composables.CanUser(ctx, permission.UploadRead); err != nil {
		return nil, err
	}
	return s.repo.GetByHash(ctx, hash)
}

func (s *UploadService) GetAll(ctx context.Context) ([]*upload.Upload, error) {
	if err := composables.CanUser(ctx, permission.UploadRead); err != nil {
		return nil, err
	}
	return s.repo.GetAll(ctx)
}

func (s *UploadService) Create(ctx context.Context, data *upload.CreateDTO) (*upload.Upload, error) {
	if err := composables.CanUser(ctx, permission.UploadCreate); err != nil {
		return nil, err
	}
	entity, bytes, err := data.ToEntity()
	if err != nil {
		return nil, err
	}
	up, err := s.repo.GetByHash(ctx, entity.Hash)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if up != nil {
		return up, nil
	}
	if err := s.storage.Save(ctx, entity.Hash, bytes); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, entity); err != nil {
		return nil, err
	}
	createdEvent, err := upload.NewCreatedEvent(ctx, *data, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(createdEvent)
	return entity, nil
}

func (s *UploadService) CreateMany(ctx context.Context, data []*upload.CreateDTO) ([]*upload.Upload, error) {
	if err := composables.CanUser(ctx, permission.UploadCreate); err != nil {
		return nil, err
	}
	entities := make([]*upload.Upload, 0, len(data))
	for _, d := range data {
		entity, err := s.Create(ctx, d)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}
	return entities, nil
}

func (s *UploadService) Update(ctx context.Context, id uint, data *upload.UpdateDTO) error {
	if err := composables.CanUser(ctx, permission.UploadUpdate); err != nil {
		return err
	}
	entity, err := data.ToEntity(id)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, entity); err != nil {
		return err
	}
	updatedEvent, err := upload.NewUpdatedEvent(ctx, *data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *UploadService) Delete(ctx context.Context, id uint) (*upload.Upload, error) {
	if err := composables.CanUser(ctx, permission.UploadDelete); err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	deletedEvent, err := upload.NewDeletedEvent(ctx, *entity)
	if err != nil {
		return nil, err
	}
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
