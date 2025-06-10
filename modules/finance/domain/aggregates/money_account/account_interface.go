package moneyaccount

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
)

type Account interface {
	ID() uint
	SetID(id uint)

	TenantID() uuid.UUID
	UpdateTenantID(id uuid.UUID) Account

	Name() string
	UpdateName(name string) Account

	AccountNumber() string
	UpdateAccountNumber(accountNumber string) Account

	Description() string
	UpdateDescription(description string) Account

	Balance() float64
	UpdateBalance(balance float64) Account

	Currency() currency.Currency
	UpdateCurrency(currency currency.Currency) Account

	CreatedAt() time.Time
	UpdatedAt() time.Time

	InitialTransaction() transaction.Transaction
}
