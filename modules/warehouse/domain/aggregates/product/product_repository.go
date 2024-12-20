package product

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
	Status    string
	CreatedAt DateRange
}

type FindByPositionParams struct {
	Limit      int
	SortBy     []string
	PositionID uint
	Status     Status
}

type CountParams struct {
	PositionID uint
	Status     Status
}

type Repository interface {
	GetPaginated(context.Context, *FindParams) ([]*Product, error)
	Count(context.Context) (int64, error)
	CountWithFilters(context.Context, *CountParams) (int64, error)
	GetAll(context.Context) ([]*Product, error)
	GetByID(context.Context, uint) (*Product, error)
	GetByRfid(context.Context, string) (*Product, error)
	GetByRfidMany(context.Context, []string) ([]*Product, error)
	FindByPositionID(context.Context, *FindByPositionParams) ([]*Product, error)
	UpdateStatus(context.Context, []uint, Status) error
	Create(context.Context, *Product) error
	BulkCreate(context.Context, []*Product) error
	CreateOrUpdate(context.Context, *Product) error
	Update(context.Context, *Product) error
	BulkDelete(context.Context, []uint) error
	Delete(context.Context, uint) error
}
