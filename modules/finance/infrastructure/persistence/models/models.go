package models

import (
	"database/sql"
	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"time"
)

type ExpenseCategory struct {
	ID               uint
	Name             string
	Description      sql.NullString
	Amount           float64
	AmountCurrencyID string
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
}

type Payment struct {
	ID             uint
	TransactionID  uint
	CounterpartyID uint
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Counterparty struct {
	ID           uint
	Tin          string
	Name         string
	Type         string
	LegalType    string
	LegalAddress string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
