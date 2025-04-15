package category

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
)

type Option func(e *expenseCategory)

// Option setters
func WithID(id uint) Option {
	return func(e *expenseCategory) {
		e.id = id
	}
}

func WithName(name string) Option {
	return func(e *expenseCategory) {
		e.name = name
	}
}

func WithDescription(description string) Option {
	return func(e *expenseCategory) {
		e.description = description
	}
}

func WithAmount(amount float64) Option {
	return func(e *expenseCategory) {
		e.amount = amount
	}
}

func WithCurrency(currency *currency.Currency) Option {
	return func(e *expenseCategory) {
		e.currency = currency
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(e *expenseCategory) {
		e.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(e *expenseCategory) {
		e.updatedAt = updatedAt
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(e *expenseCategory) {
		e.tenantID = tenantID
	}
}

// Interface
type ExpenseCategory interface {
	ID() uint
	TenantID() uuid.UUID
	Name() string
	Description() string
	Amount() float64
	Currency() *currency.Currency
	CreatedAt() time.Time
	UpdatedAt() time.Time

	UpdateAmount(a float64) ExpenseCategory
}

// Implementation
func New(
	name string,
	amount float64,
	currency *currency.Currency,
	opts ...Option,
) ExpenseCategory {
	e := &expenseCategory{
		id:          0,
		tenantID:    uuid.Nil,
		name:        name,
		description: "", // description is optional
		amount:      amount,
		currency:    currency,
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

type expenseCategory struct {
	id          uint
	tenantID    uuid.UUID
	name        string
	description string
	amount      float64
	currency    *currency.Currency
	createdAt   time.Time
	updatedAt   time.Time
}

func (e *expenseCategory) ID() uint {
	return e.id
}

func (e *expenseCategory) TenantID() uuid.UUID {
	return e.tenantID
}

func (e *expenseCategory) Name() string {
	return e.name
}

func (e *expenseCategory) Description() string {
	return e.description
}

func (e *expenseCategory) Amount() float64 {
	return e.amount
}

func (e *expenseCategory) UpdateAmount(a float64) ExpenseCategory {
	return New(
		e.name,
		a,
		e.currency,
		WithID(e.id),
		WithTenantID(e.tenantID),
		WithDescription(e.description),
		WithCreatedAt(e.createdAt),
		WithUpdatedAt(time.Now()),
	)
}

func (e *expenseCategory) Currency() *currency.Currency {
	return e.currency
}

func (e *expenseCategory) CreatedAt() time.Time {
	return e.createdAt
}

func (e *expenseCategory) UpdatedAt() time.Time {
	return e.updatedAt
}
