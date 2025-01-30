package client

import "context"

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
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Client, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Client, error)
	GetByID(ctx context.Context, id uint) (Client, error)
	GetByPhone(ctx context.Context, phoneNumber string) (Client, error)
	Create(ctx context.Context, data Client) (Client, error)
	Update(ctx context.Context, data Client) (Client, error)
	Delete(ctx context.Context, id uint) error
}
