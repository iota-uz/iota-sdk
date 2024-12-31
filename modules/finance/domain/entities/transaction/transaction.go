package transaction

import (
	"time"
)

type Transaction struct {
	ID                   uint
	Amount               float64
	AmountCurrencyID     string
	OriginAccountID      *uint
	DestinationAccountID *uint
	TransactionDate      time.Time
	AccountingPeriod     time.Time
	TransactionType      Type
	Comment              string
	CreatedAt            time.Time
}

func NewDeposit(
	amount float64,
	currencyID string,
	originAccount,
	destinationAccount uint,
	date time.Time,
	accountingPeriod time.Time,
	comment string,
) *Transaction {
	var origAccId *uint
	if originAccount != 0 {
		origAccId = &originAccount
	}
	var destAccId *uint
	if destinationAccount != 0 {
		destAccId = &destinationAccount
	}
	return &Transaction{
		Amount:               amount,
		AmountCurrencyID:     currencyID,
		OriginAccountID:      origAccId,
		DestinationAccountID: destAccId,
		TransactionType:      Deposit,
		TransactionDate:      date,
		AccountingPeriod:     accountingPeriod,
		Comment:              comment,
		CreatedAt:            time.Now(),
	}
}

func NewWithdrawal(
	amount float64,
	currencyID string,
	originAccount,
	destinationAccount uint,
	date time.Time,
	accountingPeriod time.Time,
	comment string,
) *Transaction {
	var origAccId *uint
	if originAccount != 0 {
		origAccId = &originAccount
	}
	var destAccId *uint
	if destinationAccount != 0 {
		destAccId = &destinationAccount
	}
	return &Transaction{
		Amount:               amount,
		AmountCurrencyID:     currencyID,
		OriginAccountID:      origAccId,
		DestinationAccountID: destAccId,
		TransactionType:      Withdrawal,
		TransactionDate:      date,
		AccountingPeriod:     accountingPeriod,
		Comment:              comment,
		CreatedAt:            time.Now(),
	}
}
