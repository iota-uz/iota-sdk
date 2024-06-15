package services

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/authlog"
)

type AuthLogService struct {
	repo authlog.Repository
	app  *Application
}

func NewAuthLogService(repo authlog.Repository, app *Application) *AuthLogService {
	return &AuthLogService{
		repo: repo,
		app:  app,
	}
}

func (s *AuthLogService) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *AuthLogService) GetAll(ctx context.Context) ([]*authlog.AuthenticationLog, error) {
	return s.repo.GetAll(ctx)
}

func (s *AuthLogService) GetByID(ctx context.Context, id int64) (*authlog.AuthenticationLog, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AuthLogService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*authlog.AuthenticationLog, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *AuthLogService) Create(ctx context.Context, data *authlog.AuthenticationLog) error {
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("authlog.created", data)
	return nil
}

func (s *AuthLogService) Update(ctx context.Context, data *authlog.AuthenticationLog) error {
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("authlog.updated", data)
	return nil
}

func (s *AuthLogService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("authlog.deleted", id)
	return nil
}
