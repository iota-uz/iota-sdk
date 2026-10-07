package notifications

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrDispatchNotFound = errors.New("notification dispatch not found")

type DispatchJob struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	Event        Event
	Rule         Rule
	RecipientIDs []uint
	Cursor       int
	Delivered    int
	Attempts     int
	LastError    string
	CreatedAt    time.Time
	AvailableAt  time.Time
	CompletedAt  *time.Time
}

type DispatchStats struct{ Pending, Retrying, Completed int64 }

type DispatchSummary struct {
	ID                                    uuid.UUID
	EventKey                              string
	Total, Processed, Delivered, Attempts int
	LastError                             string
	CreatedAt, AvailableAt                time.Time
	CompletedAt                           *time.Time
}

type DispatchRepository interface {
	Enqueue(context.Context, Event, Rule, []uint) (DispatchJob, error)
	Next(context.Context) (*DispatchJob, error)
	Advance(context.Context, uuid.UUID, int, int) error
	Fail(context.Context, uuid.UUID) error
	Retry(context.Context, uuid.UUID) error
	Stats(context.Context) (DispatchStats, error)
	List(context.Context) ([]DispatchSummary, error)
}
