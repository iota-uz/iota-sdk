// Package services provides this package.
package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

// GroupService TODO: refactor it
// GroupService provides operations for managing groups
type GroupService struct {
	repo      group.Repository
	publisher eventbus.EventBus
	policy    *PrivilegeGrantPolicy
}

// NewGroupService creates a new group service instance
func NewGroupService(repo group.Repository, publisher eventbus.EventBus, policy *PrivilegeGrantPolicy) *GroupService {
	return &GroupService{
		repo:      repo,
		publisher: publisher,
		policy:    policy,
	}
}

// Count returns the total number of groups
func (s *GroupService) Count(ctx context.Context, params *group.FindParams) (int64, error) {
	return s.repo.Count(ctx, params)
}

// GetPaginated returns a paginated list of groups
func (s *GroupService) GetPaginated(ctx context.Context, params *group.FindParams) ([]group.Group, error) {
	if err := composables.CanUser(ctx, permissions.GroupRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, params)
}

// GetPaginatedWithTotal returns a paginated list of groups with total count
func (s *GroupService) GetPaginatedWithTotal(ctx context.Context, params *group.FindParams) ([]group.Group, int64, error) {
	if err := composables.CanUser(ctx, permissions.GroupRead); err != nil {
		return nil, 0, err
	}
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
	if err := composables.CanUser(ctx, permissions.GroupRead); err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// GetAll returns all groups
func (s *GroupService) GetAll(ctx context.Context) ([]group.Group, error) {
	if err := composables.CanUser(ctx, permissions.GroupRead); err != nil {
		return nil, err
	}
	return s.repo.GetPaginated(ctx, &group.FindParams{
		Limit: 1000, // Use a high limit to fetch all groups
	})
}

// Create creates a new group
func (s *GroupService) Create(ctx context.Context, g group.Group) (group.Group, error) {
	if err := composables.CanUser(ctx, permissions.GroupCreate); err != nil {
		return nil, err
	}

	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var savedGroup group.Group
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		g, err = s.policy.AuthorizeGroupCreate(txCtx, g)
		if err != nil {
			return err
		}
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
	if err := composables.CanUser(ctx, permissions.GroupUpdate); err != nil {
		return nil, err
	}

	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var oldGroup group.Group
	var updatedGroup group.Group

	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		oldGroup, g, err = s.policy.AuthorizeGroupUpdate(txCtx, g)
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
	if err := composables.CanUser(ctx, permissions.GroupDelete); err != nil {
		return err
	}

	actor, err := composables.UseUser(ctx)
	if err != nil {
		return err
	}

	var g group.Group
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		g, err = s.policy.AuthorizeGroupDelete(txCtx, id)
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
	if err := composables.CanUser(ctx, permissions.GroupUpdate); err != nil {
		return nil, err
	}

	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var savedGroup group.Group
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		g, userToAdd, err := s.policy.AuthorizeGroupMembership(txCtx, groupID, userToAdd.ID(), true)
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
	if err := composables.CanUser(ctx, permissions.GroupUpdate); err != nil {
		return nil, err
	}

	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var savedGroup group.Group
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		g, userToRemove, err := s.policy.AuthorizeGroupMembership(txCtx, groupID, userToRemove.ID(), false)
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
	if err := composables.CanUser(ctx, permissions.GroupUpdate); err != nil {
		return nil, err
	}

	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var g group.Group
	var savedGroup group.Group
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		g, roleToAssign, err = s.policy.AuthorizeGroupRoleChange(txCtx, groupID, roleToAssign.ID(), true)
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
	if err := composables.CanUser(ctx, permissions.GroupUpdate); err != nil {
		return nil, err
	}

	actor, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}

	var g group.Group
	var savedGroup group.Group
	err = composables.InTx(ctx, func(txCtx context.Context) error {
		var err error
		g, roleToRemove, err = s.policy.AuthorizeGroupRoleChange(txCtx, groupID, roleToRemove.ID(), false)
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
