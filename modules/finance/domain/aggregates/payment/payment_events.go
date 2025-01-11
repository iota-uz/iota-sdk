package payment

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func NewCreatedEvent(ctx context.Context, data CreateDTO, result Payment) (*Created, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := composables.UseSession(ctx)
	if err != nil {
		return nil, err
	}
	ev := &Created{
		Data:    data,
		Result:  result,
		Sender:  sender,
		Session: *sess,
	}
	return ev, nil
}

func NewUpdatedEvent(ctx context.Context, data UpdateDTO, result Payment) (*Updated, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := composables.UseSession(ctx)
	if err != nil {
		return nil, err
	}
	return &Updated{
		Data:    data,
		Sender:  sender,
		Session: *sess,
		Result:  result,
	}, nil
}

func NewDeletedEvent(ctx context.Context, result Payment) (*Deleted, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := composables.UseSession(ctx)
	if err != nil {
		return nil, err
	}
	return &Deleted{
		Session: *sess,
		Sender:  sender,
		Result:  result,
	}, nil
}

type Created struct {
	Sender  user.User
	Session session.Session
	Data    CreateDTO
	Result  Payment
}

type Updated struct {
	Sender  user.User
	Session session.Session
	Data    UpdateDTO
	Result  Payment
}

type Deleted struct {
	Sender  user.User
	Session session.Session
	Result  Payment
}
