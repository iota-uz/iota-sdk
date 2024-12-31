package moneyaccount

import (
	"context"
)

type DateRange struct {
	From string
	To   string
}

type FindParams struct {
	Limit     int
	Offset    int
	SortBy    []string
	Query     string
	Field     string
	CreatedAt DateRange
}

type Repository interface {
	Count(context.Context) (int64, error)
	GetAll(context.Context) ([]*Account, error)
	GetPaginated(context.Context, *FindParams) ([]*Account, error)
	GetByID(context.Context, uint) (*Account, error)
	RecalculateBalance(context.Context, uint) error
	Create(context.Context, *Account) (*Account, error)
	Update(context.Context, *Account) error
	Delete(context.Context, uint) error
}
