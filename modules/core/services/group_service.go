package services

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

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

// GetByID returns a group by its ID
func (s *GroupService) GetByID(ctx context.Context, id group.GroupID) (group.Group, error) {
	return s.repo.GetByID(ctx, id)
}

// Create creates a new group
func (s *GroupService) Create(ctx context.Context, g group.Group, actor user.User) (group.Group, error) {
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	txCtx := composables.WithTx(ctx, tx)
	
	savedGroup, err := s.repo.Save(txCtx, g)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	evt := group.NewCreatedEvent(savedGroup, actor)
	s.publisher.Publish("group.created", evt)

	return savedGroup, nil
}

// Update updates an existing group
func (s *GroupService) Update(ctx context.Context, g group.Group, actor user.User) (group.Group, error) {
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	txCtx := composables.WithTx(ctx, tx)
	
	oldGroup, err := s.repo.GetByID(txCtx, g.ID())
	if err != nil {
		return nil, err
	}

	updatedGroup, err := s.repo.Save(txCtx, g)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	evt := group.NewUpdatedEvent(oldGroup, updatedGroup, actor)
	s.publisher.Publish("group.updated", evt)

	return updatedGroup, nil
}

// Delete removes a group by its ID
func (s *GroupService) Delete(ctx context.Context, id group.GroupID, actor user.User) error {
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	txCtx := composables.WithTx(ctx, tx)
	
	g, err := s.repo.GetByID(txCtx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(txCtx, id); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	evt := group.NewDeletedEvent(g, actor)
	s.publisher.Publish("group.deleted", evt)

	return nil
}

// AddUser adds a user to a group
func (s *GroupService) AddUser(ctx context.Context, groupID group.GroupID, userToAdd user.User, actor user.User) (group.Group, error) {
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	txCtx := composables.WithTx(ctx, tx)
	
	g, err := s.repo.GetByID(txCtx, groupID)
	if err != nil {
		return nil, err
	}

	updatedGroup := g.AddUser(userToAdd)
	
	savedGroup, err := s.repo.Save(txCtx, updatedGroup)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	evt := group.NewUserAddedEvent(savedGroup, userToAdd, actor)
	s.publisher.Publish("group.user.added", evt)

	return savedGroup, nil
}

// RemoveUser removes a user from a group
func (s *GroupService) RemoveUser(ctx context.Context, groupID group.GroupID, userToRemove user.User, actor user.User) (group.Group, error) {
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	txCtx := composables.WithTx(ctx, tx)
	
	g, err := s.repo.GetByID(txCtx, groupID)
	if err != nil {
		return nil, err
	}

	updatedGroup := g.RemoveUser(userToRemove)
	
	savedGroup, err := s.repo.Save(txCtx, updatedGroup)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	evt := group.NewUserRemovedEvent(savedGroup, userToRemove, actor)
	s.publisher.Publish("group.user.removed", evt)

	return savedGroup, nil
}

// AssignRole assigns a role to a group
func (s *GroupService) AssignRole(ctx context.Context, groupID group.GroupID, roleToAssign role.Role, actor user.User) (group.Group, error) {
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	txCtx := composables.WithTx(ctx, tx)
	
	g, err := s.repo.GetByID(txCtx, groupID)
	if err != nil {
		return nil, err
	}

	updatedGroup := g.AssignRole(roleToAssign)
	
	savedGroup, err := s.repo.Save(txCtx, updatedGroup)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	evt := group.NewUpdatedEvent(g, savedGroup, actor)
	s.publisher.Publish("group.role.assigned", evt)

	return savedGroup, nil
}

// RemoveRole removes a role from a group
func (s *GroupService) RemoveRole(ctx context.Context, groupID group.GroupID, roleToRemove role.Role, actor user.User) (group.Group, error) {
	tx, err := composables.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	txCtx := composables.WithTx(ctx, tx)
	
	g, err := s.repo.GetByID(txCtx, groupID)
	if err != nil {
		return nil, err
	}

	updatedGroup := g.RemoveRole(roleToRemove)
	
	savedGroup, err := s.repo.Save(txCtx, updatedGroup)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	evt := group.NewUpdatedEvent(g, savedGroup, actor)
	s.publisher.Publish("group.role.removed", evt)

	return savedGroup, nil
}