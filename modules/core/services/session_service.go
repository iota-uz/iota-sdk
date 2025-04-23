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
	var err error
	var createdSession *session.Session
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err != nil {
			return err
		}
		entity := data.ToEntity()
		if err := s.repo.Create(txCtx, entity); err != nil {
			return err
		}
		createdSession = entity
		return nil
	})
	createdEvent, err := session.NewCreatedEvent(*createdSession)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *SessionService) Update(ctx context.Context, data *session.Session) error {
	var err error

	updatedEvent, err := session.NewUpdatedEvent(*data)
	if err != nil {
		return err
	}

	var updatedSession *session.Session
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err != nil {
			return err
		}
		if err := s.repo.Update(txCtx, data); err != nil {
			return err
		}
		userAfterUpdate, err := s.repo.GetByToken(txCtx, data.Token)
		if err != nil {
			return err
		}
		updatedSession = userAfterUpdate
		return nil
	})
	if err != nil {
		return err
	}

	updatedEvent.Result = *updatedSession
	s.publisher.Publish(updatedEvent)

	return nil
}

func (s *SessionService) Delete(ctx context.Context, token string) error {
	var err error
	var deletedSession *session.Session
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err != nil {
			return err
		}
		ses, err := s.repo.GetByToken(txCtx, token)
		if err != nil {
			return err
		}
		err = s.repo.Delete(txCtx, token)
		if err != nil {
			return err
		}
		deletedSession = ses
		return nil
	})
	if err != nil {
		return err
	}
	deletedEvent, err := session.NewDeletedEvent(*deletedSession)
	if err != nil {
		return err
	}
	s.publisher.Publish(deletedEvent)
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
		deletedEvent, err := session.NewDeletedEvent(*sess)
		if err != nil {
			return nil, err
		}
		s.publisher.Publish(deletedEvent)
	}
	return deletedSessions, nil
}
