package payment

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/entities/session"
	"github.com/iota-agency/iota-erp/internal/domain/entities/user"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

func NewCreatedEvent(ctx context.Context, data CreateDTO) (*Created, error) {
	ev := &Created{
		Data: data,
	}
	if u, err := composables.UseUser(ctx); err == nil {
		ev.Sender = *u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		ev.Session = *sess
	}
	return ev, nil
}

func NewUpdatedEvent(ctx context.Context, data UpdateDTO) (*Updated, error) {
	ev := &Updated{
		Data: data,
	}
	if u, err := composables.UseUser(ctx); err == nil {
		ev.Sender = *u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		ev.Session = *sess
	}
	return ev, nil
}

func NewDeletedEvent(ctx context.Context) (*Deleted, error) {
	ev := &Deleted{}
	if u, err := composables.UseUser(ctx); err == nil {
		ev.Sender = *u
	}
	if sess, err := composables.UseSession(ctx); err == nil {
		ev.Session = *sess
	}
	return ev, nil
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
