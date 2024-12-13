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
	CreatedAt DateRange
}

type Repository interface {
	GetPaginated(ctx context.Context, params *FindParams) ([]*Product, error)
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Product, error)
	GetByID(ctx context.Context, id uint) (*Product, error)
	GetByRfid(ctx context.Context, rfid string) (*Product, error)
	Create(ctx context.Context, data *Product) error
	BulkCreate(ctx context.Context, data []*Product) error
	CreateOrUpdate(ctx context.Context, data *Product) error
	Update(ctx context.Context, data *Product) error
	Delete(ctx context.Context, id uint) error
}
