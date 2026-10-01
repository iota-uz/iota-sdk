package job

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// CreatedEvent is published when a job is enqueued. Sender and session may be
// nil: the enqueue path runs inside an HTTP request, but the SDK keeps the
// event constructible from worker contexts without them.
type CreatedEvent struct {
	Sender  user.User
	Session session.Session
	Result  Job
}

func NewCreatedEvent(ctx context.Context, result Job) (*CreatedEvent, error) {
	sender, _ := composables.UseUser(ctx)
	sess, _ := composables.UseSession(ctx)
	return &CreatedEvent{
		Sender:  sender,
		Session: sess,
		Result:  result,
	}, nil
}
