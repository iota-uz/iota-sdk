package moneyaccount

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type Account interface {
	ID() uuid.UUID
	SetID(id uuid.UUID)

	TenantID() uuid.UUID
	UpdateTenantID(id uuid.UUID) Account

	Name() string
	UpdateName(name string) Account

	AccountNumber() string
	UpdateAccountNumber(accountNumber string) Account

	Description() string
	UpdateDescription(description string) Account

	Balance() *money.Money
	UpdateBalance(balance *money.Money) Account

	CreatedAt() time.Time
	UpdatedAt() time.Time

	InitialTransaction() transaction.Transaction
}
