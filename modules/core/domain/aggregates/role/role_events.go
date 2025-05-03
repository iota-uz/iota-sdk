package role

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/constants"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
)

func NewCreatedEvent(ctx context.Context, data Role) (*CreatedEvent, error) {
	sess, ok := ctx.Value(constants.SessionKey).(*session.Session)
	if !ok {
		return nil, errors.New("no session found")
	}
	return &CreatedEvent{
		Session: *sess,
		Data:    data,
	}, nil
}

func NewUpdatedEvent(ctx context.Context, data Role) (*UpdatedEvent, error) {
	sess, ok := ctx.Value(constants.SessionKey).(*session.Session)
	if !ok {
		return nil, errors.New("no session found")
	}
	return &UpdatedEvent{
		Session: *sess,
		Data:    data,
	}, nil
}

func NewDeletedEvent(ctx context.Context) (*DeletedEvent, error) {
	sess, ok := ctx.Value(constants.SessionKey).(*session.Session)
	if !ok {
		return nil, errors.New("no session found")
	}
	return &DeletedEvent{
		Session: *sess,
	}, nil
}

type CreatedEvent struct {
	Session session.Session
	Data    Role
	Result  Role
}

type UpdatedEvent struct {
	Session session.Session
	Data    Role
	Result  Role
}

type DeletedEvent struct {
	Session session.Session
	Result  Role
}
