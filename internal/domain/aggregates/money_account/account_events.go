package moneyaccount

import (
	"context"
	"github.com/iota-agency/iota-sdk/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/internal/domain/entities/session"
	"github.com/iota-agency/iota-sdk/pkg/composables"
)

func NewCreatedEvent(ctx context.Context, data CreateDTO, result Account) (*CreatedEvent, error) {
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

func NewUpdatedEvent(ctx context.Context, data UpdateDTO, result Account) (*UpdatedEvent, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := composables.UseSession(ctx)
	if err != nil {
		return nil, err
	}
	return &UpdatedEvent{
		Sender:  *sender,
		Session: *sess,
		Data:    data,
		Result:  result,
	}, nil
}

func NewDeletedEvent(ctx context.Context, result Account) (*DeletedEvent, error) {
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
	Sender  user.User
	Session session.Session
	Data    CreateDTO
	Result  Account
}

type UpdatedEvent struct {
	Sender  user.User
	Session session.Session
	Data    UpdateDTO
	Result  Account
}

type DeletedEvent struct {
	Sender  user.User
	Session session.Session
	Result  Account
}
