package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type SessionService struct {
	repo      session.Repository
	publisher eventbus.EventBus
}

func NewSessionService(repo session.Repository, publisher eventbus.EventBus) *SessionService {
	return &SessionService{
		repo:      repo,
		publisher: publisher,
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

func (s *SessionService) GetPaginated(
	ctx context.Context, params *session.FindParams,
) ([]*session.Session, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *SessionService) Create(ctx context.Context, data *session.CreateDTO) error {
	entity := data.ToEntity()
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent, err := session.NewCreatedEvent(*data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *SessionService) Update(ctx context.Context, data *session.Session) error {
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("session.updated", data)
	return nil
}

func (s *SessionService) Delete(ctx context.Context, token string) error {
	if err := s.repo.Delete(ctx, token); err != nil {
		return err
	}
	s.publisher.Publish("session.deleted", token)
	return nil
}

func (s *SessionService) DeleteByUserId(ctx context.Context, userId uint) ([]*session.Session, error) {
	var err error
	var deletedSessions []*session.Session
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err != nil {
			return err
		}
		sessions, err := s.repo.DeleteByUserId(txCtx, userId)
		if err != nil {
			return err
		}
		deletedSessions = sessions
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, sess := range deletedSessions {
		s.publisher.Publish("session.deleted", sess.Token)
	}
	return deletedSessions, nil
}
