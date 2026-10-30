package businessevent

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrDeliveryNotFound = errors.New("business event delivery not found")

// Delivery is the outbox entry that hands one stored event to one downstream consumer.
type Delivery struct {
	TenantID    uuid.UUID
	EventID     uuid.UUID
	Consumer    string
	OperationID string
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	AvailableAt time.Time
	DeliveredAt *time.Time
}

// FailureSummary describes undelivered events that have failed at least once, across tenants.
type FailureSummary struct {
	Count  int
	Oldest *Delivery
}

type DeliveryRepository interface {
	// Enqueue schedules the event for each consumer in the transaction carried by ctx;
	// scheduling an already scheduled pair again is a no-op.
	Enqueue(ctx context.Context, eventID uuid.UUID, consumers []string) error
	// Claim locks the next due delivery of any tenant in the transaction carried by ctx;
	// it returns nil when nothing is due.
	Claim(ctx context.Context) (*Delivery, error)
	Complete(ctx context.Context, d Delivery) error
	// Fail records the failed attempt and schedules the next one with a growing delay.
	Fail(ctx context.Context, d Delivery, reason string) error
	// Retry makes an undelivered delivery of the current tenant due now.
	Retry(ctx context.Context, eventID uuid.UUID, consumer string) error
	// ListFailed returns the current tenant's undelivered deliveries that failed at least once, oldest first.
	ListFailed(ctx context.Context, limit int) ([]Delivery, error)
	Failures(ctx context.Context) (FailureSummary, error)
}
