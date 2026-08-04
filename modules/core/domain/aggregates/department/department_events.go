// Package department provides this package.
package department

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
)

type CreatedEvent struct {
	Department Department
	Timestamp  time.Time
	Actor      user.User
}

func NewCreatedEvent(department Department, actor user.User) *CreatedEvent {
	return &CreatedEvent{
		Department: department,
		Timestamp:  time.Now(),
		Actor:      actor,
	}
}

type UpdatedEvent struct {
	Department    Department
	OldDepartment Department
	Timestamp     time.Time
	Actor         user.User
}

func NewUpdatedEvent(oldDepartment, newDepartment Department, actor user.User) *UpdatedEvent {
	return &UpdatedEvent{
		Department:    newDepartment,
		OldDepartment: oldDepartment,
		Timestamp:     time.Now(),
		Actor:         actor,
	}
}

type DeletedEvent struct {
	Department Department
	Timestamp  time.Time
	Actor      user.User
}

func NewDeletedEvent(department Department, actor user.User) *DeletedEvent {
	return &DeletedEvent{
		Department: department,
		Timestamp:  time.Now(),
		Actor:      actor,
	}
}
