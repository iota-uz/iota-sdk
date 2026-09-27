// Package userposition provides this package.
package userposition

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
)

type CreatedEvent struct {
	Position  UserPosition
	Timestamp time.Time
	Actor     user.User
}

func NewCreatedEvent(position UserPosition, actor user.User) *CreatedEvent {
	return &CreatedEvent{
		Position:  position,
		Timestamp: time.Now(),
		Actor:     actor,
	}
}

type UpdatedEvent struct {
	Position    UserPosition
	OldPosition UserPosition
	Timestamp   time.Time
	Actor       user.User
}

func NewUpdatedEvent(oldPosition, newPosition UserPosition, actor user.User) *UpdatedEvent {
	return &UpdatedEvent{
		Position:    newPosition,
		OldPosition: oldPosition,
		Timestamp:   time.Now(),
		Actor:       actor,
	}
}

type DeletedEvent struct {
	Position  UserPosition
	Timestamp time.Time
	Actor     user.User
}

func NewDeletedEvent(position UserPosition, actor user.User) *DeletedEvent {
	return &DeletedEvent{
		Position:  position,
		Timestamp: time.Now(),
		Actor:     actor,
	}
}
