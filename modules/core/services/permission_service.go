package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type PermissionService struct {
	repo      permission.Repository
	publisher eventbus.EventBus
}

func NewPermissionService(repo permission.Repository, publisher eventbus.EventBus) *PermissionService {
	return &PermissionService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *PermissionService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *PermissionService) GetAll(ctx context.Context) ([]*permission.Permission, error) {
	return s.repo.GetAll(ctx)
}

func (s *PermissionService) GetByID(ctx context.Context, id string) (*permission.Permission, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *PermissionService) GetPaginated(ctx context.Context, params *permission.FindParams) ([]*permission.Permission, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *PermissionService) Save(ctx context.Context, data *permission.Permission) error {
	if err := s.repo.Save(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("permission.saved", data)
	return nil
}

func (s *PermissionService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.publisher.Publish("permission.deleted", id)
	return nil
}
