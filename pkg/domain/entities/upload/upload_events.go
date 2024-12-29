// README: Commented out everything until I find a way to solve import cycles.
package upload

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/domain/entities/session"
)

// import (
// 	"context"

// 	"github.com/iota-uz/iota-sdk/pkg/composables"
// 	"github.com/iota-uz/iota-sdk/pkg/domain/aggregates/user"
// 	"github.com/iota-uz/iota-sdk/pkg/domain/entities/session"
// )

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
		Session: session.Session{},
		Data:    data,
		Result:  result,
	}, nil
}

func NewUpdatedEvent(ctx context.Context, data UpdateDTO, result Upload) (*UpdatedEvent, error) {
	// sender, err := composables.UseUser(ctx)
	// if err != nil {
	// 	return nil, err
	// }
	// sess, err := composables.UseSession(ctx)
	// if err != nil {
	// 	return nil, err
	// }
	return &UpdatedEvent{
		Session: session.Session{},
		Data:    data,
		Result:  result,
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
		Session: session.Session{},
		Result:  result,
	}, nil
}

type CreatedEvent struct {
	// Sender  user.User
	Session session.Session
	Data    CreateDTO
	Result  Upload
}

type UpdatedEvent struct {
	// Sender  user.User
	Session session.Session
	Data    UpdateDTO
	Result  Upload
}

type DeletedEvent struct {
	// Sender  user.User
	Session session.Session
	Result  Upload
}
