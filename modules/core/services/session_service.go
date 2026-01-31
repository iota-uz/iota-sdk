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

func (s *SessionService) GetAll(ctx context.Context) ([]session.Session, error) {
	return s.repo.GetAll(ctx)
}

func (s *SessionService) GetByToken(ctx context.Context, id string) (session.Session, error) {
	return s.repo.GetByToken(ctx, id)
}

func (s *SessionService) GetPaginated(
	ctx context.Context, params *session.FindParams,
) ([]session.Session, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *SessionService) Create(ctx context.Context, data *session.CreateDTO) error {
	entity := data.ToEntity()
	var createdSession session.Session
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Create(txCtx, entity); err != nil {
			return err
		}
		createdSession = entity
		return nil
	})
	if err != nil {
		return err
	}
	createdEvent, err := session.NewCreatedEvent(createdSession)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *SessionService) Update(ctx context.Context, data session.Session) error {
	updatedEvent, err := session.NewUpdatedEvent(data)
	if err != nil {
		return err
	}
	var updatedSession session.Session
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Update(txCtx, data); err != nil {
			return err
		}
		if updated, err := s.repo.GetByToken(txCtx, data.Token()); err != nil {
			return err
		} else {
			updatedSession = updated
		}
		return nil
	})
	if err != nil {
		return err
	}
	updatedEvent.Result = updatedSession
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *SessionService) Delete(ctx context.Context, token string) error {
	var deletedSession session.Session
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		if ses, err := s.repo.GetByToken(txCtx, token); err != nil {
			return err
		} else {
			deletedSession = ses
		}
		return s.repo.Delete(txCtx, token)
	})
	if err != nil {
		return err
	}
	deletedEvent, err := session.NewDeletedEvent(deletedSession)
	if err != nil {
		return err
	}
	s.publisher.Publish(deletedEvent)
	return nil
}

func (s *SessionService) DeleteByUserId(ctx context.Context, userId uint) ([]session.Session, error) {
	var deletedSessions []session.Session
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		if sessions, err := s.repo.DeleteByUserId(txCtx, userId); err != nil {
			return err
		} else {
			deletedSessions = sessions
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, sess := range deletedSessions {
		deletedEvent, err := session.NewDeletedEvent(sess)
		if err != nil {
			return nil, err
		}
		s.publisher.Publish(deletedEvent)
	}
	return deletedSessions, nil
}

func (s *SessionService) GetByUserID(ctx context.Context, userID uint) ([]session.Session, error) {
	return s.repo.GetByUserID(ctx, userID)
}

func (s *SessionService) TerminateSession(ctx context.Context, token string) error {
	var deletedSession session.Session
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		if ses, err := s.repo.GetByToken(txCtx, token); err != nil {
			return err
		} else {
			deletedSession = ses
		}
		return s.repo.Delete(txCtx, token)
	})
	if err != nil {
		return err
	}
	deletedEvent, err := session.NewDeletedEvent(deletedSession)
	if err != nil {
		return err
	}
	s.publisher.Publish(deletedEvent)
	return nil
}

func (s *SessionService) TerminateOtherSessions(ctx context.Context, userID uint, currentToken string) (int, error) {
	var deletedSessions []session.Session
	var count int
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		// Get all sessions that will be deleted (for event publishing)
		allSessions, err := s.repo.GetByUserID(txCtx, userID)
		if err != nil {
			return err
		}
		// Filter sessions to be deleted (all except current token)
		for _, sess := range allSessions {
			if sess.Token() != currentToken {
				deletedSessions = append(deletedSessions, sess)
			}
		}
		// Delete all sessions except the current one
		deletedCount, err := s.repo.DeleteAllExceptToken(txCtx, userID, currentToken)
		if err != nil {
			return err
		}
		count = deletedCount
		return nil
	})
	if err != nil {
		return 0, err
	}
	// Publish delete events for all deleted sessions
	for _, sess := range deletedSessions {
		deletedEvent, err := session.NewDeletedEvent(sess)
		if err != nil {
			return count, err
		}
		s.publisher.Publish(deletedEvent)
	}
	return count, nil
}
