package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type UserService struct {
	repo      user.Repository
	validator user.Validator
	publisher eventbus.EventBus
}

func NewUserService(repo user.Repository, validator user.Validator, publisher eventbus.EventBus) *UserService {
	return &UserService{
		repo:      repo,
		validator: validator,
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
	if err := composables.CanUser(ctx, permissions.UserRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

func (s *UserService) GetPaginated(ctx context.Context, params *user.FindParams) ([]user.User, error) {
	if err := composables.CanUser(ctx, permissions.UserRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

func (s *UserService) GetPaginatedWithTotal(ctx context.Context, params *user.FindParams) ([]user.User, int64, error) {
	if err := composables.CanUser(ctx, permissions.UserRead); err != nil {
		return nil, 0, err
	}
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
	if err := composables.CanUser(ctx, permissions.UserCreate); err != nil {
		return err
	}
	if err := s.validator.ValidateCreate(ctx, data); err != nil {
		return err
	}
	logger := composables.UseLogger(ctx)
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			logger.WithError(err).Error("failed to rollback transaction")
		}
	}()
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
	if err := composables.CanUser(ctx, permissions.UserUpdate); err != nil {
		return err
	}
	logger := composables.UseLogger(ctx)
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			logger.WithError(err).Error("failed to rollback transaction")
		}
	}()
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
	if err := composables.CanUser(ctx, permissions.UserDelete); err != nil {
		return nil, err
	}
	logger := composables.UseLogger(ctx)
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := tx.Rollback(ctx); err != nil {
			logger.WithError(err).Error("failed to rollback transaction")
		}
	}()
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
