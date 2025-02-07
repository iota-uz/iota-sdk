package client

import "context"

type DateRange struct {
	From string
	To   string
}

type Field int

const (
	FirstName Field = iota
	LastName
	MiddleName
	PhoneNumber
	CreatedAt
	UpdatedAt
)

type SortBy struct {
	Fields    []Field
	Ascending bool
}

type FindParams struct {
	Limit     int
	Offset    int
	Query     string
	Field     string
	SortBy    SortBy
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
