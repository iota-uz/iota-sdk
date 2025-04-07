package category

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
)

func New(
	tenantID uuid.UUID,
	name string,
	description string,
	amount float64,
	currency *currency.Currency,
) ExpenseCategory {
	return &expenseCategory{
		id:          0,
		tenantID:    tenantID,
		name:        name,
		description: description,
		amount:      amount,
		currency:    currency,
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}
}

func NewWithID(
	id uint,
	tenantID uuid.UUID,
	name string,
	description string,
	amount float64,
	currency *currency.Currency,
	createdAt time.Time,
	updatedAt time.Time,
) ExpenseCategory {
	return &expenseCategory{
		id:          id,
		tenantID:    tenantID,
		name:        name,
		description: description,
		amount:      amount,
		currency:    currency,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}
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
	return &expenseCategory{
		id:          e.id,
		name:        e.name,
		description: e.description,
		amount:      a,
		currency:    e.currency,
		createdAt:   e.createdAt,
		updatedAt:   time.Now(),
	}
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
