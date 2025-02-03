package chat

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

func NewCreatedEvent(ctx context.Context, data CreateDTO, result Chat) (*CreatedEvent, error) {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	return &CreatedEvent{
		User:   u,
		Data:   data,
		Result: result,
	}, nil
}

func NewUpdatedEvent(ctx context.Context, result Chat) (*UpdatedEvent, error) {
	u, _ := composables.UseUser(ctx)
	return &UpdatedEvent{
		User:   u,
		Result: result,
	}, nil
}

func NewDeletedEvent(ctx context.Context, result Chat) (*DeletedEvent, error) {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return nil, err
	}
	return &DeletedEvent{
		User:   u,
		Result: result,
	}, nil
}

type CreatedEvent struct {
	User   user.User
	Data   CreateDTO
	Result Chat
}

type UpdatedEvent struct {
	User   user.User
	Result Chat
}

type DeletedEvent struct {
	User   user.User
	Result Chat
}
