package billing

import (
	"context"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
)

func NewCreatedEvent(_ context.Context, result Transaction) (*CreatedEvent, error) {
	return &CreatedEvent{
		Result: result,
	}, nil
}

func NewUpdatedEvent(_ context.Context, result Transaction) (*UpdatedEvent, error) {
	return &UpdatedEvent{
		Result: result,
	}, nil
}

func NewDeletedEvent(_ context.Context, result Transaction) (*DeletedEvent, error) {
	return &DeletedEvent{
		Result: result,
	}, nil
}

type CreatedEvent struct {
	Result Transaction
}

type UpdatedEvent struct {
	Data   Transaction
	Result Transaction
}

type DeletedEvent struct {
	Result Transaction
}

type StatusChangedEvent struct {
	TransactionID uuid.UUID
	Data          Status
	Result        Status
}

type AmountChangedEvent struct {
	TransactionID uuid.UUID
	Data          Amount
	Result        Amount
}

type DetailsChangedEvent struct {
	TransactionID uuid.UUID
	Data          details.Details
	Result        details.Details
}
