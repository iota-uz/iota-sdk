// Package debt provides this package.
package debt

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// sessionOrNil is the session of ctx, or nil when there is none: a call from
// MCP, an agent or a job has a user but no session, and the events built after
// the change committed must not fail for it.
func sessionOrNil(ctx context.Context) session.Session {
	sess, err := composables.UseSession(ctx)
	if err != nil {
		return nil
	}
	return sess
}

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
		Session: sess,
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
		Session: sess,
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
		Session: sess,
		Sender:  sender,
		Result:  result,
	}, nil
}

func NewDebtSettledEvent(ctx context.Context, result Debt) (*Settled, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess := sessionOrNil(ctx)
	return &Settled{
		Session: sess,
		Sender:  sender,
		Result:  result,
	}, nil
}

func NewDebtWrittenOffEvent(ctx context.Context, result Debt) (*WrittenOff, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess := sessionOrNil(ctx)
	return &WrittenOff{
		Session: sess,
		Sender:  sender,
		Result:  result,
	}, nil
}

func NewDebtCancelledEvent(ctx context.Context, result Debt) (*Cancelled, error) {
	sender, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	sess := sessionOrNil(ctx)
	return &Cancelled{
		Session: sess,
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

type Cancelled struct {
	Sender  user.User
	Session session.Session
	Result  Debt
}
