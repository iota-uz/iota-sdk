package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type SessionService struct {
	repo      user.SessionRepository
	publisher eventbus.EventBus
}

func NewSessionService(repo user.SessionRepository, publisher eventbus.EventBus) *SessionService {
	return &SessionService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *SessionService) GetCount(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *SessionService) GetAll(ctx context.Context) ([]*user.Session, error) {
	return s.repo.GetAll(ctx)
}

func (s *SessionService) GetByToken(ctx context.Context, id user.SessionID) (*user.Session, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *SessionService) GetPaginated(
	ctx context.Context, params *user.SessionFindParams,
) ([]*user.Session, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *SessionService) Create(ctx context.Context, data *user.CreateSessionDTO) error {
	entity := data.ToEntity()
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	return nil
}

func (s *SessionService) Update(ctx context.Context, data *user.Session) error {
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	return nil
}

func (s *SessionService) Delete(ctx context.Context, token user.SessionID) error {
	if err := s.repo.Delete(ctx, token); err != nil {
		return err
	}
	return nil
}
