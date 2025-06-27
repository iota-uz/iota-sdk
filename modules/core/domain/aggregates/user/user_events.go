package user

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"

	"github.com/iota-uz/iota-sdk/pkg/constants"
)

func NewCreatedEvent(ctx context.Context, data User) *CreatedEvent {
	var sender User
	var sess *session.Session

	if user, ok := ctx.Value(constants.UserKey).(User); ok {
		sender = user
	}

	if sessionValue, ok := ctx.Value(constants.SessionKey).(*session.Session); ok {
		sess = sessionValue
	}

	return &CreatedEvent{
		Sender:  sender,
		Session: sess,
		Data:    data,
	}
}

func NewUpdatedEvent(ctx context.Context, data User) *UpdatedEvent {
	var sender User
	var sess *session.Session

	if user, ok := ctx.Value(constants.UserKey).(User); ok {
		sender = user
	}

	if sessionValue, ok := ctx.Value(constants.SessionKey).(*session.Session); ok {
		sess = sessionValue
	}

	return &UpdatedEvent{
		Sender:  sender,
		Session: sess,
		Data:    data,
	}
}

func NewDeletedEvent(ctx context.Context) *DeletedEvent {
	var sender User
	var sess *session.Session

	if user, ok := ctx.Value(constants.UserKey).(User); ok {
		sender = user
	}

	if sessionValue, ok := ctx.Value(constants.SessionKey).(*session.Session); ok {
		sess = sessionValue
	}

	return &DeletedEvent{
		Sender:  sender,
		Session: sess,
	}
}

type CreatedEvent struct {
	Sender  User
	Session *session.Session
	Data    User
	Result  User
}

type UpdatedEvent struct {
	Sender  User
	Session *session.Session
	Data    User
	Result  User
}

type UpdatedPasswordEvent struct {
	UserID uint
}

type DeletedEvent struct {
	Sender  User
	Session *session.Session
	Result  User
}
