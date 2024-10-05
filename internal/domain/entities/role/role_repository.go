package role

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Role, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Role, error)
	GetByID(ctx context.Context, id int64) (*Role, error)
	CreateOrUpdate(ctx context.Context, role *Role) error
	Create(ctx context.Context, upload *Role) error
	Update(ctx context.Context, upload *Role) error
	Delete(ctx context.Context, id int64) error
}
