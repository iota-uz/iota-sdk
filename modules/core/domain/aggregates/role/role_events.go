package role

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/constants"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
)

func NewCreatedEvent(ctx context.Context, data Role) (*CreatedEvent, error) {
	var sess *session.Session
	if sessionValue, ok := ctx.Value(constants.SessionKey).(*session.Session); ok {
		sess = sessionValue
	}
	return &CreatedEvent{
		Session: sess,
		Data:    data,
	}, nil
}

func NewUpdatedEvent(ctx context.Context, data Role) (*UpdatedEvent, error) {
	var sess *session.Session
	if sessionValue, ok := ctx.Value(constants.SessionKey).(*session.Session); ok {
		sess = sessionValue
	}
	return &UpdatedEvent{
		Session: sess,
		Data:    data,
	}, nil
}

func NewDeletedEvent(ctx context.Context) (*DeletedEvent, error) {
	var sess *session.Session
	if sessionValue, ok := ctx.Value(constants.SessionKey).(*session.Session); ok {
		sess = sessionValue
	}
	return &DeletedEvent{
		Session: sess,
	}, nil
}

type CreatedEvent struct {
	Session *session.Session
	Data    Role
	Result  Role
}

type UpdatedEvent struct {
	Session *session.Session
	Data    Role
	Result  Role
}

type DeletedEvent struct {
	Session *session.Session
	Result  Role
}
