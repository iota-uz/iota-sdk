package project_stages

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (uint, error)
	GetAll(ctx context.Context) ([]*ProjectStage, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*ProjectStage, error)
	GetByID(ctx context.Context, id uint) (*ProjectStage, error)
	Create(ctx context.Context, data *ProjectStage) error
	Update(ctx context.Context, data *ProjectStage) error
	Delete(ctx context.Context, id uint) error
}
