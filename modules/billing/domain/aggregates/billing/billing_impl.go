package billing

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
)

type Option func(t *transaction)

// --- Option setters ---

func WithID(id uuid.UUID) Option {
	return func(t *transaction) {
		t.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(t *transaction) {
		t.tenantID = tenantID
	}
}

func WithStatus(status Status) Option {
	return func(t *transaction) {
		t.status = status
	}
}

func WithAmount(quantity float64, currency Currency) Option {
	return func(t *transaction) {
		t.amount = &amount{
			quantity: quantity,
			currency: currency,
		}
	}
}

func WithDetails(details details.Details) Option {
	return func(t *transaction) {
		t.details = details
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(t *transaction) {
		t.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(t *transaction) {
		t.updatedAt = updatedAt
	}
}

// ---- Implementation ----

type amount struct {
	quantity float64
	currency Currency
}

func (m *amount) Quantity() float64 {
	return m.quantity
}

func (m *amount) Currency() Currency {
	return m.currency
}

func New(
	quantity float64,
	currency Currency,
	gateway Gateway,
	details details.Details,
	opts ...Option,
) Transaction {
	t := &transaction{
		id:      uuid.Nil,
		status:  Created,
		gateway: gateway,
		amount: &amount{
			quantity: quantity,
			currency: currency,
		},
		details:   details,
		createdAt: time.Now(),
		updatedAt: time.Now(),
		events:    []interface{}{},
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

type transaction struct {
	id        uuid.UUID
	tenantID  uuid.UUID
	status    Status
	amount    Amount
	gateway   Gateway
	details   details.Details
	createdAt time.Time
	updatedAt time.Time
	events    []interface{}
}

func (t *transaction) ID() uuid.UUID {
	return t.id
}

func (t *transaction) TenantID() uuid.UUID {
	return t.tenantID
}

func (t *transaction) Status() Status {
	return t.status
}

func (t *transaction) Gateway() Gateway {
	return t.gateway
}

func (t *transaction) Amount() Amount {
	return t.amount
}

func (t *transaction) Details() details.Details {
	return t.details
}

func (t *transaction) CreatedAt() time.Time {
	return t.createdAt
}

func (t *transaction) UpdatedAt() time.Time {
	return t.updatedAt
}

func (t *transaction) Events() []interface{} {
	return t.events
}

func (t *transaction) SetTenantID(tenantID uuid.UUID) Transaction {
	result := *t
	result.tenantID = tenantID
	result.updatedAt = time.Now()

	return &result
}

func (t *transaction) SetStatus(status Status) Transaction {
	result := *t
	event := &StatusChangedEvent{
		TransactionID: result.id,
		Data:          result.status,
		Result:        status,
	}

	result.status = status
	result.updatedAt = time.Now()

	result.events = append(result.events, event)

	return &result
}

func (t *transaction) SetAmount(quantity float64, currency Currency) Transaction {
	amount := &amount{
		quantity: quantity,
		currency: currency,
	}

	result := *t
	event := &AmountChangedEvent{
		TransactionID: result.id,
		Data:          result.amount,
		Result:        amount,
	}

	result.amount = amount
	result.updatedAt = time.Now()

	result.events = append(result.events, event)

	return &result
}

func (t *transaction) SetDetails(details details.Details) Transaction {
	result := *t
	event := &DetailsChangedEvent{
		TransactionID: result.id,
		Data:          result.details,
		Result:        details,
	}

	result.details = details
	result.updatedAt = time.Now()

	result.events = append(result.events, event)

	return &result
}
