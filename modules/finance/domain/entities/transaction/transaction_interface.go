package transaction

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type Transaction interface {
	ID() uuid.UUID

	TenantID() uuid.UUID
	UpdateTenantID(id uuid.UUID) Transaction

	Amount() *money.Money
	UpdateAmount(amount *money.Money) Transaction

	OriginAccountID() uuid.UUID
	UpdateOriginAccountID(accountID uuid.UUID) Transaction

	DestinationAccountID() uuid.UUID
	UpdateDestinationAccountID(accountID uuid.UUID) Transaction

	TransactionDate() time.Time
	UpdateTransactionDate(date time.Time) Transaction

	AccountingPeriod() time.Time
	UpdateAccountingPeriod(period time.Time) Transaction

	TransactionType() Type
	UpdateTransactionType(transactionType Type) Transaction

	Comment() string
	UpdateComment(comment string) Transaction

	CreatedAt() time.Time

	// Exchange operation methods
	ExchangeRate() *float64
	UpdateExchangeRate(rate *float64) Transaction

	DestinationAmount() *money.Money
	UpdateDestinationAmount(amount *money.Money) Transaction
}
