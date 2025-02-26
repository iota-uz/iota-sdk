package employee

import "context"

type Field int

const (
	Id Field = iota
	FirstName
	LastName
	MiddleName
	Salary
	HourlyRate
	Coefficient
	CreatedAt
)

type SortBy struct {
	Fields    []Field
	Ascending bool
}

type FindParams struct {
	Limit  int
	Offset int
	Query  string
	Field  string
	SortBy SortBy
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Employee, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Employee, error)
	GetByID(ctx context.Context, id uint) (Employee, error)
	Create(ctx context.Context, data Employee) (Employee, error)
	Update(ctx context.Context, data Employee) error
	Delete(ctx context.Context, id uint) error
}
