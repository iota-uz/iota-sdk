// Package upload README: Commented out everything until I find a way to solve import cycles.
package upload

import (
	"context"
)

func NewCreatedEvent(ctx context.Context, data CreateDTO, result Upload) (*CreatedEvent, error) {
	// sender, err := composables.UseUser(ctx)
	// if err != nil {
	// 	return nil, err
	// }
	// sess, err := composables.UseSession(ctx)
	// if err != nil {
	// 	return nil, err
	// }
	return &CreatedEvent{
		Data:   data,
		Result: result,
	}, nil
}

func NewDeletedEvent(ctx context.Context, result Upload) (*DeletedEvent, error) {
	// sender, err := composables.UseUser(ctx)
	// if err != nil {
	// 	return nil, err
	// }
	// sess, err := composables.UseSession(ctx)
	// if err != nil {
	// 	return nil, err
	// }
	return &DeletedEvent{
		Result: result,
	}, nil
}

type CreatedEvent struct {
	// Sender  user.User
	Data   CreateDTO
	Result Upload
}

type DeletedEvent struct {
	// Sender  user.User
	Result Upload
}
