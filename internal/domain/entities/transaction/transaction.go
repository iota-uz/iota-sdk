package transaction

import (
	"time"
)

type Transaction struct {
	ID               uint
	Amount           float64
	AmountCurrencyID string
	MoneyAccountID   uint
	TransactionDate  time.Time
	AccountingPeriod time.Time
	TransactionType  Type
	Comment          string
	CreatedAt        time.Time
}
