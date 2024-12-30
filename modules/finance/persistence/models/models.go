package models

import (
	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"time"
)

type ExpenseCategory struct {
	ID               uint
	Name             string
	Description      *string
	Amount           float64
	AmountCurrencyID string
	AmountCurrency   coremodels.Currency `gorm:"foreignKey:AmountCurrencyID;references:Code"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type MoneyAccount struct {
	ID                uint
	Name              string
	AccountNumber     string
	Description       string
	Balance           float64
	BalanceCurrencyID string
	Currency          *coremodels.Currency `gorm:"foreignKey:BalanceCurrencyID;references:Code"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Transaction struct {
	ID                   uint
	Amount               float64
	OriginAccountID      *uint
	DestinationAccountID *uint
	TransactionDate      time.Time
	AccountingPeriod     time.Time
	TransactionType      string
	Comment              string
	CreatedAt            time.Time
}

type Expense struct {
	ID            uint
	TransactionID uint
	CategoryID    uint
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Category      *ExpenseCategory `gorm:"foreignKey:CategoryID;references:ID"`
}

type Payment struct {
	ID             uint
	TransactionID  uint
	CounterpartyID uint
	Transaction    *Transaction `gorm:"foreignKey:TransactionID;references:ID"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
