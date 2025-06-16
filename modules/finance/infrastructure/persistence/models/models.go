package models

import (
	"database/sql"
	"time"
)

type ExpenseCategory struct {
	ID          string
	TenantID    string
	Name        string
	Description sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type MoneyAccount struct {
	ID                string
	TenantID          string
	Name              string
	AccountNumber     string
	Description       string
	Balance           int64
	BalanceCurrencyID string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Transaction struct {
	ID                   string
	TenantID             string
	Amount               int64
	OriginAccountID      sql.NullString
	DestinationAccountID sql.NullString
	TransactionDate      time.Time
	AccountingPeriod     time.Time
	TransactionType      string
	Comment              string
	ExchangeRate         sql.NullFloat64
	DestinationAmount    sql.NullInt64
	CreatedAt            time.Time
}

type Expense struct {
	ID            string
	TransactionID string
	CategoryID    string
	TenantID      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Payment struct {
	ID                string
	TransactionID     string
	CounterpartyID    string
	PaymentCategoryID sql.NullString
	TenantID          string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Inventory struct {
	ID          string
	TenantID    string
	Name        string
	Description sql.NullString
	CurrencyID  sql.NullString
	Price       int64
	Quantity    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PaymentCategory struct {
	ID          string
	TenantID    string
	Name        string
	Description sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Counterparty struct {
	ID           string
	TenantID     string
	Tin          sql.NullString
	Name         string
	Type         string
	LegalType    string
	LegalAddress string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
