package models

import (
	"database/sql"
	"time"

	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
)

type ExpenseCategory struct {
	ID               uint
	TenantID         string
	Name             string
	Description      sql.NullString
	Amount           float64
	AmountCurrencyID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type MoneyAccount struct {
	ID                uint
	TenantID          string
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
	TenantID             string
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
	TenantID     uint
	Tin          string
	Name         string
	Type         string
	LegalType    string
	LegalAddress string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
