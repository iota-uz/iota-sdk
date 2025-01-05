package role

import (
	"context"
)

type FindParams struct {
	ID                uint
	UserID            uint
	AttachPermissions bool
	Limit             int
	Offset            int
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Role, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Role, error)
	GetByID(ctx context.Context, id uint) (Role, error)
	CreateOrUpdate(ctx context.Context, role Role) error
	Create(ctx context.Context, upload Role) error
	Update(ctx context.Context, upload Role) error
	Delete(ctx context.Context, id uint) error
}
