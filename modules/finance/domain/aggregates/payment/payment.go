package payment

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type Payment interface {
	ID() uuid.UUID
	SetID(id uuid.UUID)

	TenantID() uuid.UUID
	UpdateTenantID(id uuid.UUID) Payment

	Amount() *money.Money
	UpdateAmount(amount *money.Money) Payment

	TransactionID() uuid.UUID

	CounterpartyID() uuid.UUID
	UpdateCounterpartyID(partyID uuid.UUID) Payment

	Category() paymentcategory.PaymentCategory
	UpdateCategory(category paymentcategory.PaymentCategory) Payment
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
