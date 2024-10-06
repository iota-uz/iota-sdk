package user

import (
	"context"
	"errors"
	"github.com/iota-agency/iota-erp/internal/domain/entities/session"
	"github.com/iota-agency/iota-erp/pkg/constants"
)

func NewCreatedEvent(ctx context.Context, data User) (*CreatedEvent, error) {
	sender, ok := ctx.Value(constants.UserKey).(*User)
	if !ok {
		return nil, errors.New("no user found")
	}
	sess, ok := ctx.Value(constants.SessionKey).(*session.Session)
	if !ok {
		return nil, errors.New("no session found")
	}
	return &CreatedEvent{
		Sender:  *sender,
		Session: *sess,
		Data:    data,
	}, nil
}

func NewUpdatedEvent(ctx context.Context, data User) (*UpdatedEvent, error) {
	sender, ok := ctx.Value(constants.UserKey).(*User)
	if !ok {
		return nil, errors.New("no user found")
	}
	sess, ok := ctx.Value(constants.SessionKey).(*session.Session)
	if !ok {
		return nil, errors.New("no session found")
	}
	return &UpdatedEvent{
		Sender:  *sender,
		Session: *sess,
		Data:    data,
	}, nil
}

func NewDeletedEvent(ctx context.Context) (*DeletedEvent, error) {
	sender, ok := ctx.Value(constants.UserKey).(*User)
	if !ok {
		return nil, errors.New("no user found")
	}
	sess, ok := ctx.Value(constants.SessionKey).(*session.Session)
	if !ok {
		return nil, errors.New("no session found")
	}
	return &DeletedEvent{
		Sender:  *sender,
		Session: *sess,
	}, nil
}

type CreatedEvent struct {
	Sender  User
	Session session.Session
	Data    User
	Result  User
}

type UpdatedEvent struct {
	Sender  User
	Session session.Session
	Data    User
	Result  User
}

type DeletedEvent struct {
	Sender  User
	Session session.Session
	Result  User
}
