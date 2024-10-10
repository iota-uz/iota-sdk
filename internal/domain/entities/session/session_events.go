package session

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/entities/user"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

func NewCreatedEvent(ctx context.Context, data CreateDTO, result Session) (*CreatedEvent, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := composables.UseSession(ctx)
	if err != nil {
		return nil, err
	}
	return &CreatedEvent{
		Sender:  *sender,
		Session: *sess,
		Data:    data,
		Result:  result,
	}, nil
}

func NewDeletedEvent(ctx context.Context, result Session) (*DeletedEvent, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := composables.UseSession(ctx)
	if err != nil {
		return nil, err
	}
	return &DeletedEvent{
		Sender:  *sender,
		Session: *sess,
		Result:  result,
	}, nil
}

type CreatedEvent struct {
	Session Session
	Sender  user.User
	Data    CreateDTO
	Result  Session
}

type DeletedEvent struct {
	Session Session
	Sender  user.User
	Result  Session
}
