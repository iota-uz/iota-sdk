package group

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
)

type CreatedEvent struct {
	Group     Group
	Timestamp time.Time
	Actor     user.User
}

func NewCreatedEvent(group Group, actor user.User) *CreatedEvent {
	return &CreatedEvent{
		Group:     group,
		Timestamp: time.Now(),
		Actor:     actor,
	}
}

type UpdatedEvent struct {
	Group     Group
	OldGroup  Group
	Timestamp time.Time
	Actor     user.User
}

func NewUpdatedEvent(oldGroup, newGroup Group, actor user.User) *UpdatedEvent {
	return &UpdatedEvent{
		Group:     newGroup,
		OldGroup:  oldGroup,
		Timestamp: time.Now(),
		Actor:     actor,
	}
}

type DeletedEvent struct {
	Group     Group
	Timestamp time.Time
	Actor     user.User
}

func NewDeletedEvent(group Group, actor user.User) *DeletedEvent {
	return &DeletedEvent{
		Group:     group,
		Timestamp: time.Now(),
		Actor:     actor,
	}
}

type UserAddedEvent struct {
	Group     Group
	AddedUser user.User
	Timestamp time.Time
	Actor     user.User
}

func NewUserAddedEvent(group Group, addedUser user.User, actor user.User) *UserAddedEvent {
	return &UserAddedEvent{
		Group:     group,
		AddedUser: addedUser,
		Timestamp: time.Now(),
		Actor:     actor,
	}
}

type UserRemovedEvent struct {
	Group       Group
	RemovedUser user.User
	Timestamp   time.Time
	Actor       user.User
}

func NewUserRemovedEvent(group Group, removedUser user.User, actor user.User) *UserRemovedEvent {
	return &UserRemovedEvent{
		Group:       group,
		RemovedUser: removedUser,
		Timestamp:   time.Now(),
		Actor:       actor,
	}
}

