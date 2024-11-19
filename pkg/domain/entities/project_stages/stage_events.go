package project_stages

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"

	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/session"
)

func NewCreatedEvent(ctx context.Context, data CreateDTO) (*CreatedEvent, error) {
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

func NewUpdatedEvent(ctx context.Context, data UpdateDTO) (*UpdatedEvent, error) {
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
	Sender  user.User
	Session session.Session
	Data    CreateDTO
	Result  ProjectStage
}

type UpdatedEvent struct {
	Sender  user.User
	Session session.Session
	Data    UpdateDTO
	Result  ProjectStage
}

type DeletedEvent struct {
	Sender  user.User
	Session session.Session
	Result  ProjectStage
}
