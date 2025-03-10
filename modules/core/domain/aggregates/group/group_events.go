package group

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
)

// CreatedEvent is triggered when a new group is created
type CreatedEvent struct {
	Group     Group
	Timestamp time.Time
	Actor     user.User
}

// NewCreatedEvent creates a new group created event
func NewCreatedEvent(group Group, actor user.User) *CreatedEvent {
	return &CreatedEvent{
		Group:     group,
		Timestamp: time.Now(),
		Actor:     actor,
	}
}

// UpdatedEvent is triggered when a group is updated
type UpdatedEvent struct {
	Group     Group
	OldGroup  Group
	Timestamp time.Time
	Actor     user.User
}

// NewUpdatedEvent creates a new group updated event
func NewUpdatedEvent(oldGroup, newGroup Group, actor user.User) *UpdatedEvent {
	return &UpdatedEvent{
		Group:     newGroup,
		OldGroup:  oldGroup,
		Timestamp: time.Now(),
		Actor:     actor,
	}
}

// DeletedEvent is triggered when a group is deleted
type DeletedEvent struct {
	Group     Group
	Timestamp time.Time
	Actor     user.User
}

// NewDeletedEvent creates a new group deleted event
func NewDeletedEvent(group Group, actor user.User) *DeletedEvent {
	return &DeletedEvent{
		Group:     group,
		Timestamp: time.Now(),
		Actor:     actor,
	}
}

// UserAddedEvent is triggered when a user is added to a group
type UserAddedEvent struct {
	Group     Group
	AddedUser user.User
	Timestamp time.Time
	Actor     user.User
}

// NewUserAddedEvent creates a new user added to group event
func NewUserAddedEvent(group Group, addedUser user.User, actor user.User) *UserAddedEvent {
	return &UserAddedEvent{
		Group:     group,
		AddedUser: addedUser,
		Timestamp: time.Now(),
		Actor:     actor,
	}
}

// UserRemovedEvent is triggered when a user is removed from a group
type UserRemovedEvent struct {
	Group       Group
	RemovedUser user.User
	Timestamp   time.Time
	Actor       user.User
}

// NewUserRemovedEvent creates a new user removed from group event
func NewUserRemovedEvent(group Group, removedUser user.User, actor user.User) *UserRemovedEvent {
	return &UserRemovedEvent{
		Group:       group,
		RemovedUser: removedUser,
		Timestamp:   time.Now(),
		Actor:       actor,
	}
}