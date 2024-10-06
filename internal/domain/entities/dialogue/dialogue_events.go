package dialogue

import (
	"context"

	"github.com/iota-agency/iota-erp/internal/domain/entities/session"
	"github.com/iota-agency/iota-erp/internal/domain/entities/user"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

func NewCreatedEvent(ctx context.Context, data Dialogue) (*CreatedEvent, error) {
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
	}, nil
}

func NewUpdatedEvent(ctx context.Context, data Dialogue) (*UpdatedEvent, error) {
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
		Sender:  *sender,
		Session: *sess,
	}, nil
}

type CreatedEvent struct {
	Data    Dialogue
	Result  Dialogue
	Sender  user.User
	Session session.Session
}

type UpdatedEvent struct {
	Data    Dialogue
	Result  Dialogue
	Sender  user.User
	Session session.Session
}

type DeletedEvent struct {
	Result  Dialogue
	Sender  user.User
	Session session.Session
}
