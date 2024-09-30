package dialogue

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Dialogue, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Dialogue, error)
	GetByID(ctx context.Context, id int64) (*Dialogue, error)
	GetByUserID(ctx context.Context, userID int64) ([]*Dialogue, error)
	Create(ctx context.Context, upload *Dialogue) error
	Update(ctx context.Context, upload *Dialogue) error
	Delete(ctx context.Context, id int64) error
}
