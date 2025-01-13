package payment

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"time"
)

type Payment interface {
	ID() uint
	SetID(id uint)

	Amount() float64
	SetAmount(amount float64)

	TransactionID() uint

	CounterpartyID() uint
	SetCounterpartyID(partyID uint)

	TransactionDate() time.Time
	SetTransactionDate(t time.Time)

	AccountingPeriod() time.Time
	SetAccountingPeriod(t time.Time)

	Comment() string
	SetComment(comment string)

	Account() *moneyaccount.Account
	User() user.User
	CreatedAt() time.Time
	UpdatedAt() time.Time
}
