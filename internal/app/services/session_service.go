package services

import (
	"context"

	"github.com/iota-agency/iota-erp/internal/domain/entities/authlog"
	"github.com/iota-agency/iota-erp/internal/domain/entities/session"
)

type SessionService struct {
	repo session.Repository
	app  *Application
}

func NewSessionService(repo session.Repository, app *Application) *SessionService {
	return &SessionService{
		repo: repo,
		app:  app,
	}
}

func (s *SessionService) GetCount(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *SessionService) GetAll(ctx context.Context) ([]*session.Session, error) {
	return s.repo.GetAll(ctx)
}

func (s *SessionService) GetByToken(ctx context.Context, id string) (*session.Session, error) {
	return s.repo.GetByToken(ctx, id)
}

func (s *SessionService) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*session.Session, error) {
	return s.repo.GetPaginated(ctx, limit, offset, sortBy)
}

func (s *SessionService) Create(ctx context.Context, data *session.Session) error {
	if err := s.repo.Create(ctx, data); err != nil {
		return err
	}
	log := &authlog.AuthenticationLog{
		UserID:    data.UserID,
		IP:        data.IP,
		UserAgent: data.UserAgent,
	}
	if err := s.app.AuthLogService.Create(ctx, log); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("session.created", data)
	return nil
}

func (s *SessionService) Update(ctx context.Context, data *session.Session) error {
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("session.updated", data)
	return nil
}

func (s *SessionService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.app.EventPublisher.Publish("session.deleted", id)
	return nil
}
