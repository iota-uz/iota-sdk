package services

import (
	"context"
	role2 "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/pkg/event"
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

func (s *RoleService) GetByID(ctx context.Context, id uint) (*role2.Role, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RoleService) GetPaginated(ctx context.Context, params *role.FindParams) ([]*role2.Role, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *RoleService) Create(ctx context.Context, data *role2.Role) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("role.created", data)
	return tx.Commit(ctx)
}

func (s *RoleService) Update(ctx context.Context, data *role2.Role) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("role.updated", data)
	return tx.Commit(ctx)
}

func (s *RoleService) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.publisher.Publish("role.deleted", id)
	return tx.Commit(ctx)
}
