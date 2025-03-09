package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type UserService struct {
	repo      user.Repository
	publisher eventbus.EventBus
}

func NewUserService(repo user.Repository, publisher eventbus.EventBus) *UserService {
	return &UserService{
		repo:      repo,
		publisher: publisher,
	}
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (user.User, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *UserService) Count(ctx context.Context, params *user.FindParams) (int64, error) {
	return s.repo.Count(ctx, params)
}

func (s *UserService) GetAll(ctx context.Context) ([]user.User, error) {
	return s.repo.GetAll(ctx)
}

func (s *UserService) GetByID(ctx context.Context, id uint) (user.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) GetPaginated(ctx context.Context, params *user.FindParams) ([]user.User, error) {
	return s.repo.GetPaginated(ctx, params)
}

func (s *UserService) GetPaginatedWithTotal(ctx context.Context, params *user.FindParams) ([]user.User, int64, error) {
	us, err := s.repo.GetPaginated(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	return us, total, nil
}

func (s *UserService) Create(ctx context.Context, data user.User) error {
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	createdEvent, err := user.NewCreatedEvent(ctx, data)
	if err != nil {
		return err
	}
	data, err = data.SetPassword(data.Password())
	if err != nil {
		return err
	}
	created, err := s.repo.Create(ctx, data)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	createdEvent.Result = created
	s.publisher.Publish(createdEvent)
	return nil
}

func (s *UserService) UpdateLastAction(ctx context.Context, id uint) error {
	return s.repo.UpdateLastAction(ctx, id)
}

func (s *UserService) UpdateLastLogin(ctx context.Context, id uint) error {
	return s.repo.UpdateLastLogin(ctx, id)
}

func (s *UserService) Update(ctx context.Context, data user.User) error {
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	updatedEvent, err := user.NewUpdatedEvent(ctx, data)
	if err != nil {
		return err
	}
	if data.Password() != "" {
		data, err = data.SetPassword(data.Password())
		if err != nil {
			return err
		}
	}
	if err := s.repo.Update(ctx, data); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	updatedEvent.Result = data
	s.publisher.Publish(updatedEvent)
	return nil
}

func (s *UserService) Delete(ctx context.Context, id uint) (user.User, error) {
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	deletedEvent, err := user.NewDeletedEvent(ctx)
	if err != nil {
		return nil, err
	}
	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	deletedEvent.Result = entity
	s.publisher.Publish(deletedEvent)
	return entity, nil
}
