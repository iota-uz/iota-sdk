package spotlight

import (
	"context"
	"errors"
)

type OutboxProcessor interface {
	PollAndProcess(ctx context.Context) error
}

var ErrNoOutboxDocument = errors.New("spotlight: no outbox document")
