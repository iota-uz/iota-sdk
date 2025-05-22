package billing

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"time"
)

type Status string
type Currency string

const (
	Created           Status = "created"
	Pending           Status = "pending"
	Completed         Status = "completed"
	Failed            Status = "failed"
	Canceled          Status = "canceled"
	Refunded          Status = "refunded"
	PartiallyRefunded Status = "partially-refunded"
	Expired           Status = "expired"
)

const (
	UZS Currency = "UZS"
	USD Currency = "USD"
	EUR Currency = "EUR"
	RUB Currency = "RUB"
)

// ---- Interfaces ----

type Transaction interface {
	ID() uuid.UUID

	TenantID() uuid.UUID

	Status() Status
	Amount() Amount

	Gateway() Gateway
	Details() details.Details

	CreatedAt() time.Time
	UpdatedAt() time.Time

	Events() []interface{}

	SetTenantID(tenantID uuid.UUID) Transaction
	SetStatus(status Status) Transaction
	SetAmount(quantity float64, currency Currency) Transaction
	SetDetails(details details.Details) Transaction
}

type Amount interface {
	Quantity() float64
	Currency() Currency
}
