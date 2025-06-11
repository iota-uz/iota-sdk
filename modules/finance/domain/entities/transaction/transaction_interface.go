package transaction

import (
	"time"

	"github.com/google/uuid"
)

type Transaction interface {
	ID() uuid.UUID
	SetID(id uuid.UUID)

	TenantID() uuid.UUID
	UpdateTenantID(id uuid.UUID) Transaction

	Amount() float64
	UpdateAmount(amount float64) Transaction

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

	DestinationAmount() *float64
	UpdateDestinationAmount(amount *float64) Transaction
}
