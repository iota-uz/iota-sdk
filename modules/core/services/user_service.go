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
	err := composables.CanUser(ctx, permissions.UserCreate)
	if err != nil {
		return err
	}

	createdEvent, err := user.NewCreatedEvent(ctx, data)
	if err != nil {
		return err
	}

	var createdUser user.User
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err = s.validator.ValidateCreate(txCtx, data); err != nil {
			return err
		}
		if created, err := s.repo.Create(txCtx, data); err != nil {
			return err
		} else {
			createdUser = created
		}
		return nil
	})
	if err != nil {
		return err
	}
	createdEvent.Result = createdUser

	s.publisher.Publish(createdEvent)
	for _, e := range data.Events() {
		s.publisher.Publish(e)
	}

	return err
}

func (s *UserService) UpdateLastAction(ctx context.Context, id uint) error {
	return s.repo.UpdateLastAction(ctx, id)
}

func (s *UserService) UpdateLastLogin(ctx context.Context, id uint) error {
	return s.repo.UpdateLastLogin(ctx, id)
}

func (s *UserService) Update(ctx context.Context, data user.User) error {
	err := composables.CanUser(ctx, permissions.UserUpdate)
	if err != nil {
		return err
	}

	if !data.CanUpdate() {
		return composables.ErrForbidden
	}

	updatedEvent, err := user.NewUpdatedEvent(ctx, data)
	if err != nil {
		return err
	}

	var updatedUser user.User
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err = s.validator.ValidateUpdate(txCtx, data); err != nil {
			return err
		}
		if err = s.repo.Update(txCtx, data); err != nil {
			return err
		}
		if userAfterUpdate, err := s.repo.GetByID(txCtx, data.ID()); err != nil {
			return err
		} else {
			updatedUser = userAfterUpdate
		}
		return nil
	})
	if err != nil {
		return err
	}

	updatedEvent.Result = updatedUser

	s.publisher.Publish(updatedEvent)
	for _, e := range data.Events() {
		s.publisher.Publish(e)
	}

	return nil
}

func (s *UserService) Delete(ctx context.Context, id uint) (user.User, error) {
	err := composables.CanUser(ctx, permissions.UserDelete)
	if err != nil {
		return nil, err
	}

	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !entity.CanDelete() {
		return nil, composables.ErrForbidden
	}

	deletedEvent, err := user.NewDeletedEvent(ctx)
	if err != nil {
		return nil, err
	}

	var deletedUser user.User
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Delete(txCtx, id); err != nil {
			return err
		} else {
			deletedUser = entity
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	deletedEvent.Result = deletedUser

	s.publisher.Publish(deletedEvent)

	return deletedUser, nil
}
