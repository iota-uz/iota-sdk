package transaction

import (
	"time"
)

type Transaction struct {
	ID                   uint
	Amount               float64
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
	originAccount,
	destinationAccount uint,
	date time.Time,
	accountingPeriod time.Time,
	comment string,
) *Transaction {
	var origAccID *uint
	if originAccount != 0 {
		origAccID = &originAccount
	}
	var destAccID *uint
	if destinationAccount != 0 {
		destAccID = &destinationAccount
	}
	return &Transaction{
		ID:                   0,
		Amount:               amount,
		OriginAccountID:      origAccID,
		DestinationAccountID: destAccID,
		TransactionType:      Deposit,
		TransactionDate:      date,
		AccountingPeriod:     accountingPeriod,
		Comment:              comment,
		CreatedAt:            time.Now(),
	}
}

func NewWithdrawal(
	amount float64,
	originAccount,
	destinationAccount uint,
	date time.Time,
	accountingPeriod time.Time,
	comment string,
) *Transaction {
	var origAccID *uint
	if originAccount != 0 {
		origAccID = &originAccount
	}
	var destAccID *uint
	if destinationAccount != 0 {
		destAccID = &destinationAccount
	}
	return &Transaction{
		ID:                   0,
		Amount:               amount,
		OriginAccountID:      origAccID,
		DestinationAccountID: destAccID,
		TransactionType:      Withdrawal,
		TransactionDate:      date,
		AccountingPeriod:     accountingPeriod,
		Comment:              comment,
		CreatedAt:            time.Now(),
	}
}
