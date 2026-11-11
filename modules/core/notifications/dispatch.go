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
	Mode         NoticeMode
	RecipientIDs []uint
	// Addressees are the record and address recipients of an explicit send.
	// The job walks RecipientIDs first, then Addressees.
	Addressees  []Addressee
	Cursor      int
	Delivered   int
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	AvailableAt time.Time
	CompletedAt *time.Time
}

// DispatchRequest is one durable intent: the event, the rule that selects
// channels and level, and the recipients to walk.
type DispatchRequest struct {
	Event        Event
	Rule         Rule
	Mode         NoticeMode
	RecipientIDs []uint
	Addressees   []Addressee
}

// Total is the number of recipients the job walks.
func (j DispatchJob) Total() int { return len(j.RecipientIDs) + len(j.Addressees) }

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
	Enqueue(context.Context, DispatchRequest) (DispatchJob, error)
	Next(context.Context) (*DispatchJob, error)
	Advance(context.Context, uuid.UUID, int, int) error
	Fail(context.Context, uuid.UUID) error
	Retry(context.Context, uuid.UUID) error
	Stats(context.Context) (DispatchStats, error)
	List(context.Context) ([]DispatchSummary, error)
}
