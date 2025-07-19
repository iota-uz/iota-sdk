package viewmodels

import "time"

type Transaction struct {
	ID                     string
	Amount                 string
	AmountWithCurrency     string
	OriginAccountID        string
	OriginAccountName      string
	DestinationAccountID   string
	DestinationAccountName string
	TransactionDate        time.Time
	AccountingPeriod       time.Time
	TransactionType        string
	Comment                string
	CreatedAt              time.Time
	
	// Exchange fields
	ExchangeRate           string
	DestinationAmount      string
	DestinationAmountWithCurrency string
	
	// Related entities
	PaymentID              string
	PaymentCategory        string
	ExpenseID              string
	ExpenseCategory        string
	Counterparty           string
}

type TransactionListItem struct {
	ID                     string
	Amount                 string
	AmountWithCurrency     string
	AccountName            string // Origin or destination based on type
	TransactionDate        time.Time
	TransactionType        string
	TypeBadgeClass         string // CSS class for transaction type badge
	Comment                string
	Category               string // Payment or expense category
	Counterparty           string
}