package product

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"

	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func NewCreatedEvent(ctx context.Context, data CreateDTO, result Product) (*CreatedEvent, error) {
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
		Result:  result,
	}, nil
}

func NewUpdatedEvent(ctx context.Context, data UpdateDTO, result Product) (*UpdatedEvent, error) {
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
		Result:  result,
	}, nil
}

func NewDeletedEvent(ctx context.Context, result Product) (*DeletedEvent, error) {
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
	Data    CreateDTO
	Result  Product
}

type UpdatedEvent struct {
	Sender  user.User
	Session session.Session
	Data    UpdateDTO
	Result  Product
}

type DeletedEvent struct {
	Sender  user.User
	Session session.Session
	Result  Product
}
