package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

// GroupService TODO: refactor it
// GroupService provides operations for managing groups
type GroupService struct {
	repo      group.Repository
	publisher eventbus.EventBus
}

// NewGroupService creates a new group service instance
func NewGroupService(repo group.Repository, publisher eventbus.EventBus) *GroupService {
	return &GroupService{
		repo:      repo,
		publisher: publisher,
	}
}

// Count returns the total number of groups
func (s *GroupService) Count(ctx context.Context, params *group.FindParams) (int64, error) {
	return s.repo.Count(ctx, params)
}

// GetPaginated returns a paginated list of groups
func (s *GroupService) GetPaginated(ctx context.Context, params *group.FindParams) ([]group.Group, error) {
	return s.repo.GetPaginated(ctx, params)
}

// GetPaginatedWithTotal returns a paginated list of groups with total count
func (s *GroupService) GetPaginatedWithTotal(ctx context.Context, params *group.FindParams) ([]group.Group, int64, error) {
	groups, err := s.repo.GetPaginated(ctx, params)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.repo.Count(ctx, params)
	if err != nil {
		return nil, 0, err
	}

	return groups, total, nil
}

// GetByID returns a group by its ID
func (s *GroupService) GetByID(ctx context.Context, id uuid.UUID) (group.Group, error) {
	return s.repo.GetByID(ctx, id)
}

// GetAll returns all groups
func (s *GroupService) GetAll(ctx context.Context) ([]group.Group, error) {
	return s.repo.GetPaginated(ctx, &group.FindParams{
		Limit: 1000, // Use a high limit to fetch all groups
	})
}

// Create creates a new group
func (s *GroupService) Create(ctx context.Context, g group.Group) (group.Group, error) {
	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var savedGroup group.Group
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		savedGroup, err = s.repo.Save(txCtx, g)
		return err
	})
	if err != nil {
		return nil, err
	}

	evt := group.NewCreatedEvent(savedGroup, actor)
	s.publisher.Publish(evt)

	return savedGroup, nil
}

// Update updates an existing group
func (s *GroupService) Update(ctx context.Context, g group.Group) (group.Group, error) {
	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	if !g.CanUpdate() {
		return nil, composables.ErrForbidden
	}

	var oldGroup group.Group
	var updatedGroup group.Group

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		oldGroup, err = s.repo.GetByID(txCtx, g.ID())
		if err != nil {
			return err
		}

		updatedGroup, err = s.repo.Save(txCtx, g)
		return err
	})
	if err != nil {
		return nil, err
	}

	evt := group.NewUpdatedEvent(oldGroup, updatedGroup, actor)
	s.publisher.Publish(evt)

	return updatedGroup, nil
}

// Delete removes a group by its ID
func (s *GroupService) Delete(ctx context.Context, id uuid.UUID) error {
	actor, err := composables.UseUser(ctx)
	if err != nil {
		return err
	}

	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !entity.CanDelete() {
		return composables.ErrForbidden
	}

	var g group.Group
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		g, err = s.repo.GetByID(txCtx, id)
		if err != nil {
			return err
		}

		return s.repo.Delete(txCtx, id)
	})
	if err != nil {
		return err
	}

	evt := group.NewDeletedEvent(g, actor)
	s.publisher.Publish(evt)

	return nil
}

// AddUser adds a user to a group
func (s *GroupService) AddUser(ctx context.Context, groupID uuid.UUID, userToAdd user.User) (group.Group, error) {
	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var savedGroup group.Group
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		g, err := s.repo.GetByID(txCtx, groupID)
		if err != nil {
			return err
		}

		updatedGroup := g.AddUser(userToAdd)

		savedGroup, err = s.repo.Save(txCtx, updatedGroup)
		return err
	})
	if err != nil {
		return nil, err
	}

	evt := group.NewUserAddedEvent(savedGroup, userToAdd, actor)
	s.publisher.Publish(evt)

	return savedGroup, nil
}

// RemoveUser removes a user from a group
func (s *GroupService) RemoveUser(ctx context.Context, groupID uuid.UUID, userToRemove user.User) (group.Group, error) {
	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var savedGroup group.Group
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		g, err := s.repo.GetByID(txCtx, groupID)
		if err != nil {
			return err
		}

		updatedGroup := g.RemoveUser(userToRemove)

		savedGroup, err = s.repo.Save(txCtx, updatedGroup)
		return err
	})
	if err != nil {
		return nil, err
	}

	evt := group.NewUserRemovedEvent(savedGroup, userToRemove, actor)
	s.publisher.Publish(evt)

	return savedGroup, nil
}

// AssignRole assigns a role to a group
func (s *GroupService) AssignRole(ctx context.Context, groupID uuid.UUID, roleToAssign role.Role) (group.Group, error) {
	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var g group.Group
	var savedGroup group.Group
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		g, err = s.repo.GetByID(txCtx, groupID)
		if err != nil {
			return err
		}

		updatedGroup := g.AssignRole(roleToAssign)

		savedGroup, err = s.repo.Save(txCtx, updatedGroup)
		return err
	})
	if err != nil {
		return nil, err
	}

	evt := group.NewUpdatedEvent(g, savedGroup, actor)
	s.publisher.Publish(evt)

	return savedGroup, nil
}

// RemoveRole removes a role from a group
func (s *GroupService) RemoveRole(ctx context.Context, groupID uuid.UUID, roleToRemove role.Role) (group.Group, error) {
	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var g group.Group
	var savedGroup group.Group
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		g, err = s.repo.GetByID(txCtx, groupID)
		if err != nil {
			return err
		}

		updatedGroup := g.RemoveRole(roleToRemove)

		savedGroup, err = s.repo.Save(txCtx, updatedGroup)
		return err
	})
	if err != nil {
		return nil, err
	}

	evt := group.NewUpdatedEvent(g, savedGroup, actor)
	s.publisher.Publish(evt)

	return savedGroup, nil
}
