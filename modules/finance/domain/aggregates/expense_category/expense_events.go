package category

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func NewCreatedEvent(ctx context.Context, data ExpenseCategory) (*CreatedEvent, error) {
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
		Data:    data,
	}, nil
}

func NewUpdatedEvent(ctx context.Context, data ExpenseCategory) (*UpdatedEvent, error) {
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
		Data:    data,
	}, nil
}

func NewDeletedEvent(ctx context.Context) (*DeletedEvent, error) {
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
	}, nil
}

type CreatedEvent struct {
	Sender  user.User
	Session session.Session
	Data    ExpenseCategory
	Result  ExpenseCategory
}

type UpdatedEvent struct {
	Sender  user.User
	Session session.Session
	Data    ExpenseCategory
	Result  ExpenseCategory
}

type DeletedEvent struct {
	Sender  user.User
	Session session.Session
	Result  ExpenseCategory
}
