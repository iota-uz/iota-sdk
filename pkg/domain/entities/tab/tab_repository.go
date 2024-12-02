package tab

import "context"

type FindParams struct {
	SortBy []string
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context, params *FindParams) ([]*Tab, error)
	GetUserTabs(ctx context.Context, userID uint) ([]*Tab, error)
	GetByID(ctx context.Context, id uint) (*Tab, error)
	Create(ctx context.Context, data *Tab) error
	Update(ctx context.Context, data *Tab) error
	Delete(ctx context.Context, id uint) error
	DeleteUserTabs(ctx context.Context, userID uint) error
}
