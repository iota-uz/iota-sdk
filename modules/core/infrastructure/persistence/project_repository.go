package persistence

import (
	"context"
	project2 "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type GormProjectRepository struct{}

func NewProjectRepository() project2.Repository {
	return &GormProjectRepository{}
}

func (g *GormProjectRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*project2.Project, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var rows []*models.Project
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*project2.Project, len(rows))
	for i, r := range rows {
		entities[i] = toDomainProject(r)
	}
	return entities, nil
}

func (g *GormProjectRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Project{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return uint(count), nil
}

func (g *GormProjectRepository) GetAll(ctx context.Context) ([]*project2.Project, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var rows []*models.Project
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*project2.Project, len(rows))
	for i, r := range rows {
		entities[i] = toDomainProject(r)
	}
	return entities, nil
}

func (g *GormProjectRepository) GetByID(ctx context.Context, id uint) (*project2.Project, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var row models.Project
	if err := tx.First(&row, id).Error; err != nil {
		return nil, err
	}
	return toDomainProject(&row), nil
}

func (g *GormProjectRepository) Create(ctx context.Context, data *project2.Project) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	entity := toDBProject(data)
	return tx.Create(entity).Error
}

func (g *GormProjectRepository) Update(ctx context.Context, data *project2.Project) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	entity := toDBProject(data)
	return tx.Save(entity).Error
}

func (g *GormProjectRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Delete(&models.Project{}, id).Error //nolint:exhaustruct
}
