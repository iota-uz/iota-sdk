package dialogue

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"

	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/session"
)

func NewCreatedEvent(ctx context.Context, data Dialogue, result Dialogue) (*CreatedEvent, error) {
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

func NewUpdatedEvent(ctx context.Context, data Dialogue, result Dialogue) (*UpdatedEvent, error) {
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

func NewDeletedEvent(ctx context.Context, result Dialogue) (*DeletedEvent, error) {
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
