package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type UserService struct {
	repo           user.Repository
	validator      user.Validator
	publisher      eventbus.EventBus
	sessionService *SessionService
}

func NewUserService(repo user.Repository, validator user.Validator, publisher eventbus.EventBus, sessionService *SessionService) *UserService {
	return &UserService{
		repo:           repo,
		validator:      validator,
		publisher:      publisher,
		sessionService: sessionService,
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

func (s *UserService) Create(ctx context.Context, data user.User) (user.User, error) {
	if err := composables.CanUser(ctx, permissions.UserCreate); err != nil {
		return nil, err
	}

	createdEvent := user.NewCreatedEvent(ctx, data)

	var createdUser user.User
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.validator.ValidateCreate(txCtx, data); err != nil {
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
		return nil, err
	}
	createdEvent.Result = createdUser

	s.publisher.Publish(createdEvent)
	for _, e := range data.Events() {
		s.publisher.Publish(e)
	}

	return createdUser, nil
}

func (s *UserService) UpdateLastAction(ctx context.Context, id uint) error {
	return s.repo.UpdateLastAction(ctx, id)
}

func (s *UserService) UpdateLastLogin(ctx context.Context, id uint) error {
	return s.repo.UpdateLastLogin(ctx, id)
}

func (s *UserService) Update(ctx context.Context, data user.User) (user.User, error) {
	if err := composables.CanUser(ctx, permissions.UserUpdate); err != nil {
		return nil, err
	}

	if !data.CanUpdate() {
		return nil, composables.ErrForbidden
	}

	return s.performUpdate(ctx, data)
}

func (s *UserService) UpdateSelf(ctx context.Context, data user.User) (user.User, error) {
	currentUser, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	if currentUser.ID() != data.ID() {
		return nil, composables.ErrForbidden
	}

	if !data.CanUpdate() {
		return nil, composables.ErrForbidden
	}

	// Preserve sensitive fields from current user to prevent privilege escalation
	data = data.
		SetRoles(currentUser.Roles()).
		SetPermissions(currentUser.Permissions()).
		SetGroupIDs(currentUser.GroupIDs())

	return s.performUpdate(ctx, data)
}

// performUpdate executes the common update logic for both Update and UpdateSelf
func (s *UserService) performUpdate(ctx context.Context, data user.User) (user.User, error) {
	updatedEvent := user.NewUpdatedEvent(ctx, data)

	var updatedUser user.User
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		if err := s.validator.ValidateUpdate(txCtx, data); err != nil {
			return err
		}
		if err := s.repo.Update(txCtx, data); err != nil {
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
		return nil, err
	}

	updatedEvent.Result = updatedUser

	s.publisher.Publish(updatedEvent)
	for _, e := range data.Events() {
		s.publisher.Publish(e)
	}

	return updatedUser, nil
}

func (s *UserService) CanUserBeDeleted(ctx context.Context, userID uint) (bool, error) {
	entity, err := s.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}

	if !entity.CanDelete() {
		return false, nil
	}

	tenantID := entity.TenantID()
	userCount, err := s.repo.CountByTenantID(ctx, tenantID)
	if err != nil {
		return false, err
	}

	return userCount > 1, nil
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

	tenantID := entity.TenantID()
	userCount, err := s.repo.CountByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	if userCount <= 1 {
		return nil, errors.New("cannot delete the last user in tenant")
	}

	deletedEvent := user.NewDeletedEvent(ctx)

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

func (s *UserService) BlockUser(ctx context.Context, userID uint, reason string) (user.User, error) {
	// Check permission
	if err := composables.CanUser(ctx, permissions.UserUpdateBlockStatus); err != nil {
		return nil, err
	}

	// Validate reason length
	reason = strings.TrimSpace(reason)
	if len(reason) < 3 {
		return nil, errors.New("block reason must be at least 3 characters")
	}
	if len(reason) > 1024 {
		return nil, errors.New("block reason must not exceed 1024 characters")
	}

	var blockedUser user.User
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		// Get user entity
		u, err := s.repo.GetByID(txCtx, userID)
		if err != nil {
			return err
		}

		// Business rules validation
		if !u.CanBeBlocked() {
			return errors.New("system users cannot be blocked")
		}
		if u.IsBlocked() {
			return errors.New("user is already blocked")
		}

		// Get current user for blockedBy
		actor, err := composables.UseUser(txCtx)
		if err != nil {
			return err
		}

		// Prevent users from blocking themselves
		if actor.ID() == userID {
			return errors.New("you cannot block yourself")
		}

		// Block user
		u = u.Block(reason, actor.ID())

		// Update in repository
		if err := s.repo.Update(txCtx, u); err != nil {
			return err
		}

		// Delete all active sessions for the blocked user
		if _, err := s.sessionService.DeleteByUserId(txCtx, userID); err != nil {
			return fmt.Errorf("failed to invalidate user sessions: %w", err)
		}

		// Reload user to get updated state
		blockedUser, err = s.repo.GetByID(txCtx, userID)
		return err
	})

	if err != nil {
		return nil, err
	}

	// Publish updated event for realtime updates
	updatedEvent := user.NewUpdatedEvent(ctx, blockedUser)
	updatedEvent.Result = blockedUser
	s.publisher.Publish(updatedEvent)

	return blockedUser, nil
}

func (s *UserService) UnblockUser(ctx context.Context, userID uint) (user.User, error) {
	// Check permission
	if err := composables.CanUser(ctx, permissions.UserUpdateBlockStatus); err != nil {
		return nil, err
	}

	var unblockedUser user.User
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		// Get user entity
		u, err := s.repo.GetByID(txCtx, userID)
		if err != nil {
			return err
		}

		// Validate is blocked
		if !u.IsBlocked() {
			return errors.New("user is not blocked")
		}

		// Unblock user
		u = u.Unblock()

		// Update in repository
		if err := s.repo.Update(txCtx, u); err != nil {
			return err
		}

		// Reload user to get updated state
		unblockedUser, err = s.repo.GetByID(txCtx, userID)
		return err
	})

	if err != nil {
		return nil, err
	}

	// Publish updated event for realtime updates
	updatedEvent := user.NewUpdatedEvent(ctx, unblockedUser)
	updatedEvent.Result = unblockedUser
	s.publisher.Publish(updatedEvent)

	return unblockedUser, nil
}
