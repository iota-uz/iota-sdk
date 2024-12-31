package payment

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"time"
)

type Payment interface {
	ID() uint
	SetID(uint)

	Amount() float64
	SetAmount(float64)

	TransactionID() uint

	TransactionDate() time.Time
	SetTransactionDate(time.Time)

	AccountingPeriod() time.Time
	SetAccountingPeriod(time.Time)

	Comment() string
	SetComment(string)

	Account() *moneyaccount.Account
	User() *user.User
	CreatedAt() time.Time
	UpdatedAt() time.Time
}
