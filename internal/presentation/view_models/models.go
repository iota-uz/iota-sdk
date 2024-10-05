package view_models

import "time"

type ExpenseCategory struct {
	Id                 string
	Name               string
	Amount             string
	AmountWithCurrency string
	Description        string
	CreatedAt          string
	UpdatedAt          string
}

type Account struct {
	Id            string
	Name          string
	AccountNumber string
	Description   string
	Balance       string
	CurrencyCode  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
