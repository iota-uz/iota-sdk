package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
)

type ExpenseCategory struct {
	ID               uuid.UUID
	TenantID         string
	Name             string
	Description      sql.NullString
	Amount           float64
	AmountCurrencyID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type MoneyAccount struct {
	ID                uuid.UUID
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
	ID                   uuid.UUID
	TenantID             string
	Amount               float64
	OriginAccountID      uuid.UUID
	DestinationAccountID uuid.UUID
	TransactionDate      time.Time
	AccountingPeriod     time.Time
	TransactionType      string
	Comment              string
	CreatedAt            time.Time
}

type Expense struct {
	ID            uuid.UUID
	TransactionID uuid.UUID
	CategoryID    uuid.UUID
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Payment struct {
	ID             uuid.UUID
	TransactionID  uuid.UUID
	CounterpartyID uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Counterparty struct {
	ID           uuid.UUID
	TenantID     string
	Tin          string
	Name         string
	Type         string
	LegalType    string
	LegalAddress string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
