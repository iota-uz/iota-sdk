package permission

import "context"

type Field int

const (
	FieldName Field = iota
	FieldResource
	FieldAction
	FieldModifier
)

type SortBy struct {
	Fields    []Field
	Ascending bool
}

type FindParams struct {
	Limit  int
	Offset int
	RoleID uint
	SortBy SortBy
}

type Repository interface {
	GetPaginated(ctx context.Context, params *FindParams) ([]*Permission, error)
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]*Permission, error)
	GetByID(ctx context.Context, id string) (*Permission, error)
	Save(ctx context.Context, p *Permission) error
	Delete(ctx context.Context, id string) error
}
