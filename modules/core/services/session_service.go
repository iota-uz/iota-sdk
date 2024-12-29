package services

import (
	"context"

	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/session"
	"github.com/iota-agency/iota-sdk/pkg/event"
)

type SessionService struct {
	repo      session2.Repository
	publisher event.Publisher
}

func NewSessionService(repo session2.Repository, publisher event.Publisher) *SessionService {
	return &SessionService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *SessionService) GetCount(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}

func (s *SessionService) GetAll(ctx context.Context) ([]*session2.Session, error) {
	return s.repo.GetAll(ctx)
}

func (s *SessionService) GetByToken(ctx context.Context, id string) (*session2.Session, error) {
	return s.repo.GetByToken(ctx, id)
}

func (s *SessionService) GetPaginated(
	ctx context.Context, params *session.FindParams,
) ([]*session2.Session, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *SessionService) Create(ctx context.Context, data *session2.CreateDTO) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	entity := data.ToEntity()
	if err := s.repo.Create(ctx, entity); err != nil {
		return err
	}
	createdEvent, err := session2.NewCreatedEvent(*data, *entity)
	if err != nil {
		return err
	}
	s.publisher.Publish(createdEvent)
	return tx.Commit(ctx)
}

func (s *SessionService) Update(ctx context.Context, data *session2.Session) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	s.publisher.Publish("session.updated", data)
	return tx.Commit(ctx)
}

func (s *SessionService) Delete(ctx context.Context, token string) error {
	tx, err := composables.UsePoolTx(ctx)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, token); err != nil {
		return err
	}
	s.publisher.Publish("session.deleted", token)
	return tx.Commit(ctx)
}
