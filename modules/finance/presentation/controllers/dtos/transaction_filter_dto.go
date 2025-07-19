package dtos

import (
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type TransactionFilterDTO struct {
	Limit            int
	Offset           int
	SortBy           []string
	CreatedAt        transaction.DateRange
	TransactionType  string
	AccountID        string
	CounterpartyID   string
	DateFrom         shared.DateOnly
	DateTo           shared.DateOnly
	AccountingPeriod shared.DateOnly
}

func (f *TransactionFilterDTO) ToFindParams() *transaction.FindParams {
	return &transaction.FindParams{
		Limit:     f.Limit,
		Offset:    f.Offset,
		SortBy:    f.SortBy,
		CreatedAt: f.CreatedAt,
	}
}