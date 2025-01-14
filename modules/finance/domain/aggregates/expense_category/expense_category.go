package category

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"time"
)

type ExpenseCategory interface {
	ID() uint
	Name() string
	Description() string

	Amount() float64
	UpdateAmount(a float64) ExpenseCategory

	Currency() *currency.Currency
	CreatedAt() time.Time
	UpdatedAt() time.Time
}
