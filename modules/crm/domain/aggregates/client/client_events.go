package client

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

// NewCreatedEvent creates a new CreatedEvent for a client.
func NewCreatedEvent(ctx context.Context, data Client) (*CreatedEvent, error) {
	sender, ok := ctx.Value(constants.UserKey).(user.User)
	if !ok {
		return nil, errors.New("no user found in context")
	}
	sess, ok := ctx.Value(constants.SessionKey).(*session.Session)
	if !ok {
		return nil, errors.New("no session found in context")
	}
	return &CreatedEvent{
		Sender:  sender,
		Session: *sess,
		Data:    data,
	}, nil
}

// NewUpdatedEvent creates a new UpdatedEvent for a client.
func NewUpdatedEvent(ctx context.Context, data Client) (*UpdatedEvent, error) {
	sender, ok := ctx.Value(constants.UserKey).(user.User)
	if !ok {
		return nil, errors.New("no user found in context")
	}
	sess, ok := ctx.Value(constants.SessionKey).(*session.Session)
	if !ok {
		return nil, errors.New("no session found in context")
	}
	return &UpdatedEvent{
		Sender:  sender,
		Session: *sess,
		Data:    data,
	}, nil
}

// NewDeletedEvent creates a new DeletedEvent for a client.
func NewDeletedEvent(ctx context.Context, data Client) (*DeletedEvent, error) {
	sender, ok := ctx.Value(constants.UserKey).(user.User)
	if !ok {
		return nil, errors.New("no user found in context")
	}
	sess, ok := ctx.Value(constants.SessionKey).(*session.Session)
	if !ok {
		return nil, errors.New("no session found in context")
	}
	return &DeletedEvent{
		Sender:  sender,
		Session: *sess,
		Data:    data,
	}, nil
}

// CreatedEvent represents the event of a client being created.
type CreatedEvent struct {
	Sender  user.User
	Session session.Session
	Data    Client
	Result  Client
}

// UpdatedEvent represents the event of a client being updated.
type UpdatedEvent struct {
	Sender  user.User
	Session session.Session
	Data    Client
	Result  Client
}

// DeletedEvent represents the event of a client being deleted.
type DeletedEvent struct {
	Sender  user.User
	Session session.Session
	Data    Client
	Result  Client
}
