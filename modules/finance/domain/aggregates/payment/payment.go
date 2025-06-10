package payment

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
)

type Payment interface {
	ID() uint
	SetID(id uint)

	TenantID() uuid.UUID
	UpdateTenantID(id uuid.UUID) Payment

	Amount() float64
	UpdateAmount(amount float64) Payment

	TransactionID() uint

	CounterpartyID() uint
	UpdateCounterpartyID(partyID uint) Payment

	Category() paymentcategory.PaymentCategory
	TransactionDate() time.Time
	UpdateTransactionDate(t time.Time) Payment

	AccountingPeriod() time.Time
	UpdateAccountingPeriod(t time.Time) Payment

	Comment() string
	UpdateComment(comment string) Payment

	Account() moneyaccount.Account
	User() user.User
	CreatedAt() time.Time
	UpdatedAt() time.Time
}
