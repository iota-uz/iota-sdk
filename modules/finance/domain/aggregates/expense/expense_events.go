package expense

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"

	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func NewCreatedEvent(ctx context.Context, result Expense) (*CreatedEvent, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := composables.UseSession(ctx)
	if err != nil {
		return nil, err
	}
	return &CreatedEvent{
		Sender:  sender,
		Session: *sess,
		Result:  result,
	}, nil
}

func NewUpdatedEvent(ctx context.Context, result Expense) (*UpdatedEvent, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := composables.UseSession(ctx)
	if err != nil {
		return nil, err
	}
	return &UpdatedEvent{
		Sender:  sender,
		Session: *sess,
		Result:  result,
	}, nil
}

func NewDeletedEvent(ctx context.Context, result Expense) (*DeletedEvent, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := composables.UseSession(ctx)
	if err != nil {
		return nil, err
	}
	return &DeletedEvent{
		Sender:  sender,
		Session: *sess,
		Result:  result,
	}, nil
}

type CreatedEvent struct {
	Sender  user.User
	Session session.Session
	Result  Expense
}

type UpdatedEvent struct {
	Sender  user.User
	Session session.Session
	Result  Expense
}

type DeletedEvent struct {
	Sender  user.User
	Session session.Session
	Result  Expense
}
