package payment_category

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func NewCreatedEvent(ctx context.Context, data PaymentCategory) (*CreatedEvent, error) {
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

func NewUpdatedEvent(ctx context.Context, data PaymentCategory) (*UpdatedEvent, error) {
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
	Data    PaymentCategory
	Result  PaymentCategory
}

type UpdatedEvent struct {
	Sender  user.User
	Session session.Session
	Data    PaymentCategory
	Result  PaymentCategory
}

type DeletedEvent struct {
	Sender  user.User
	Session session.Session
	Result  PaymentCategory
}
