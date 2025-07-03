package position

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
	Fields    []string
	UnitID    string
	CreatedAt DateRange
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Position, error)
	GetAllPositionIds(ctx context.Context) ([]uint, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Position, error)
	GetByID(ctx context.Context, id uint) (Position, error)
	GetByIDs(ctx context.Context, ids []uint) ([]Position, error)
	GetByBarcode(ctx context.Context, barcode string) (Position, error)
	Create(ctx context.Context, data Position) error
	CreateOrUpdate(ctx context.Context, data Position) error
	Update(ctx context.Context, data Position) error
	Delete(ctx context.Context, id uint) error
}
