package debt

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func NewDebtCreatedEvent(ctx context.Context, data Debt, result Debt) (*Created, error) {
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

func NewDebtUpdatedEvent(ctx context.Context, data Debt, result Debt) (*Updated, error) {
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

func NewDebtDeletedEvent(ctx context.Context, result Debt) (*Deleted, error) {
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

func NewDebtSettledEvent(ctx context.Context, result Debt) (*Settled, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := composables.UseSession(ctx)
	if err != nil {
		return nil, err
	}
	return &Settled{
		Session: *sess,
		Sender:  sender,
		Result:  result,
	}, nil
}

func NewDebtWrittenOffEvent(ctx context.Context, result Debt) (*WrittenOff, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess, err := composables.UseSession(ctx)
	if err != nil {
		return nil, err
	}
	return &WrittenOff{
		Session: *sess,
		Sender:  sender,
		Result:  result,
	}, nil
}

type Created struct {
	Sender  user.User
	Session session.Session
	Data    Debt
	Result  Debt
}

type Updated struct {
	Sender  user.User
	Session session.Session
	Data    Debt
	Result  Debt
}

type Deleted struct {
	Sender  user.User
	Session session.Session
	Result  Debt
}

type Settled struct {
	Sender  user.User
	Session session.Session
	Result  Debt
}

type WrittenOff struct {
	Sender  user.User
	Session session.Session
	Result  Debt
}
