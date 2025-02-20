package role

import "context"

type Field int

const (
	Name Field = iota
	Description
	CreatedAt
)

type SortBy struct {
	Fields    []Field
	Ascending bool
}

type FindParams struct {
	Name              string
	AttachPermissions bool
	Limit             int
	Offset            int
	SortBy            SortBy
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
