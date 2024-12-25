package services

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/role"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type RoleService struct {
	repo      role.Repository
	publisher event.Publisher
}

func NewRoleService(repo role.Repository, publisher event.Publisher) *RoleService {
	return &RoleService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *RoleService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *RoleService) GetAll(ctx context.Context) ([]*role.Role, error) {
	return s.repo.GetAll(ctx)
}

func (s *RoleService) GetByID(ctx context.Context, id uint) (*role.Role, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RoleService) GetPaginated(ctx context.Context, params *role.FindParams) ([]*role.Role, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *RoleService) Create(ctx context.Context, data *role.Role) error {
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("role.created", data)
	return nil
}

func (s *RoleService) Update(ctx context.Context, data *role.Role) error {
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("role.updated", data)
	return nil
}

func (s *RoleService) Delete(ctx context.Context, id uint) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.publisher.Publish("role.deleted", id)
	return nil
}
