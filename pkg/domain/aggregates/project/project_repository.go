package project

import (
	"context"
)

type Repository interface {
	Count(ctx context.Context) (uint, error)
	GetAll(ctx context.Context) ([]*Project, error)
	GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*Project, error)
	GetByID(ctx context.Context, id uint) (*Project, error)
	Create(ctx context.Context, project *Project) error
	Update(ctx context.Context, project *Project) error
	Delete(ctx context.Context, id uint) error
}
