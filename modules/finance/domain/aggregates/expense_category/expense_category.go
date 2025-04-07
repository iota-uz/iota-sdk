package category

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
)

type ExpenseCategory interface {
	ID() uint
	TenantID() uuid.UUID
	Name() string
	Description() string

	Amount() float64
	UpdateAmount(a float64) ExpenseCategory

	Currency() *currency.Currency
	CreatedAt() time.Time
	UpdatedAt() time.Time
}
