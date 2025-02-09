package dialogue

import "context"

type FindParams struct {
	Query  string
	Field  string
	Limit  int
	Offset int
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Dialogue, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Dialogue, error)
	GetByID(ctx context.Context, id uint) (Dialogue, error)
	GetByUserID(ctx context.Context, userID uint) ([]Dialogue, error)
	Create(ctx context.Context, data Dialogue) (Dialogue, error)
	Update(ctx context.Context, data Dialogue) error
	Delete(ctx context.Context, id uint) error
}
