package permission

import "context"

type Repository interface {
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Permission, error)
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Permission, error)
	GetByID(ctx context.Context, id uint) (*Permission, error)
	Create(ctx context.Context, p *Permission) error
	Update(ctx context.Context, p *Permission) error
	Delete(ctx context.Context, id uint) error
}
