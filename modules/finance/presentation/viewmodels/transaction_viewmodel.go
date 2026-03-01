// Package viewmodels provides this package.
package viewmodels

import "time"

// Account represents a money account in viewmodels
type Account struct {
	ID       string
	Name     string
	Number   string
	Currency string
}

// Category represents either an expense or payment category
type Category struct {
	ID   string
	Name string
	Type string // "expense" or "payment"
}

// CounterpartyInfo represents counterparty data in view models.
type CounterpartyInfo struct {
	ID   string
	Name string
	TIN  string
}

type Transaction struct {
	ID                 string
	Amount             string
	AmountWithCurrency string
	TransactionDate    time.Time
	AccountingPeriod   time.Time
	TransactionType    string
	TypeBadgeClass     string
	Comment            string
	CreatedAt          time.Time

	// Accounts
	OriginAccount      *Account
	DestinationAccount *Account

	// Exchange fields
	ExchangeRate                  string
	DestinationAmount             string
	DestinationAmountWithCurrency string

	// Related entities
	Category     *Category
	Counterparty *CounterpartyInfo
}

type TransactionListItem struct {
	ID                 string
	Amount             string
	AmountWithCurrency string
	Account            *Account // Origin or destination based on type
	TransactionDate    time.Time
	TransactionType    string
	TypeBadgeClass     string // CSS class for transaction type badge
	Comment            string
	Category           *Category
	Counterparty       *CounterpartyInfo
}
