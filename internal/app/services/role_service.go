package services

import (
	"context"
	role2 "github.com/iota-agency/iota-erp/internal/domain/aggregates/role"
	"github.com/iota-agency/iota-erp/sdk/event"
)

type RoleService struct {
	repo      role2.Repository
	publisher event.Publisher
}

func NewRoleService(repo role2.Repository, publisher event.Publisher) *RoleService {
	return &RoleService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *RoleService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *RoleService) GetAll(ctx context.Context) ([]*role2.Role, error) {
	return s.repo.GetAll(ctx)
}

func (s *RoleService) GetByID(ctx context.Context, id int64) (*role2.Role, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RoleService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*role2.Role, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *RoleService) Create(ctx context.Context, data *role2.Role) error {
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("role.created", data)
	return nil
}

func (s *RoleService) Update(ctx context.Context, data *role2.Role) error {
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("role.updated", data)
	return nil
}

func (s *RoleService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.publisher.Publish("role.deleted", id)
	return nil
}
