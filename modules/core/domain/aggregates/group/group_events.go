package group

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
)

// Event represents a base structure for group events
type Event struct {
	Group     Group
	Timestamp time.Time
	Actor     user.User
}

// CreatedEvent is triggered when a new group is created
type CreatedEvent struct {
	Event
}

// NewCreatedEvent creates a new group created event
func NewCreatedEvent(group Group, actor user.User) *CreatedEvent {
	return &CreatedEvent{
		Event: Event{
			Group:     group,
			Timestamp: time.Now(),
			Actor:     actor,
		},
	}
}

// UpdatedEvent is triggered when a group is updated
type UpdatedEvent struct {
	Event
	OldGroup Group
}

// NewUpdatedEvent creates a new group updated event
func NewUpdatedEvent(oldGroup, newGroup Group, actor user.User) *UpdatedEvent {
	return &UpdatedEvent{
		Event: Event{
			Group:     newGroup,
			Timestamp: time.Now(),
			Actor:     actor,
		},
		OldGroup: oldGroup,
	}
}

// DeletedEvent is triggered when a group is deleted
type DeletedEvent struct {
	Event
}

// NewDeletedEvent creates a new group deleted event
func NewDeletedEvent(group Group, actor user.User) *DeletedEvent {
	return &DeletedEvent{
		Event: Event{
			Group:     group,
			Timestamp: time.Now(),
			Actor:     actor,
		},
	}
}

// UserAddedEvent is triggered when a user is added to a group
type UserAddedEvent struct {
	Event
	AddedUser user.User
}

// NewUserAddedEvent creates a new user added to group event
func NewUserAddedEvent(group Group, addedUser user.User, actor user.User) *UserAddedEvent {
	return &UserAddedEvent{
		Event: Event{
			Group:     group,
			Timestamp: time.Now(),
			Actor:     actor,
		},
		AddedUser: addedUser,
	}
}

// UserRemovedEvent is triggered when a user is removed from a group
type UserRemovedEvent struct {
	Event
	RemovedUser user.User
}

// NewUserRemovedEvent creates a new user removed from group event
func NewUserRemovedEvent(group Group, removedUser user.User, actor user.User) *UserRemovedEvent {
	return &UserRemovedEvent{
		Event: Event{
			Group:     group,
			Timestamp: time.Now(),
			Actor:     actor,
		},
		RemovedUser: removedUser,
	}
}