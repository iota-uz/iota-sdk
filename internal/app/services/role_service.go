package services

import (
	"context"

	"github.com/iota-agency/iota-erp/internal/domain/entities/role"
)

type RoleService struct {
	repo role.Repository
	app  *Application
}

func NewRoleService(repo role.Repository, app *Application) *RoleService {
	return &RoleService{
		repo: repo,
		app:  app,
	}
}

func (s *RoleService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *RoleService) GetAll(ctx context.Context) ([]*role.Role, error) {
	return s.repo.GetAll(ctx)
}

func (s *RoleService) GetByID(ctx context.Context, id int64) (*role.Role, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *RoleService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*role.Role, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *RoleService) Create(ctx context.Context, data *role.Role) error {
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("role.created", data)
	return nil
}

func (s *RoleService) Update(ctx context.Context, data *role.Role) error {
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("role.updated", data)
	return nil
}

func (s *RoleService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("role.deleted", id)
	return nil
}
