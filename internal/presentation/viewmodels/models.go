package viewmodels

import "time"

type ExpenseCategory struct {
	ID                 string
	Name               string
	Amount             string
	AmountWithCurrency string
	Description        string
	CreatedAt          string
	UpdatedAt          string
}

type Account struct {
	ID            string
	Name          string
	AccountNumber string
	Description   string
	Balance       string
	CurrencyCode  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
