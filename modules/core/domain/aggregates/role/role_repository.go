package role

import (
	"context"
)

type FindParams struct {
	UserID            uint
	Name              string
	AttachPermissions bool
	Limit             int
	Offset            int
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Role, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Role, error)
	GetByID(ctx context.Context, id uint) (Role, error)
	Create(ctx context.Context, upload Role) (Role, error)
	Update(ctx context.Context, upload Role) (Role, error)
	Delete(ctx context.Context, id uint) error
}
