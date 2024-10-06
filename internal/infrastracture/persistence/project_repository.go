package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/project"
	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence/models"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormProjectRepository struct {
}

func NewProjectRepository() project.Repository {
	return &GormProjectRepository{}
}

func (g *GormProjectRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*project.Project, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.Project
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	var entities []*project.Project
	for _, r := range rows {
		p, err := toDomainProject(r)
		if err != nil {
			return nil, err
		}
		entities = append(entities, p)
	}
	return entities, nil
}

func (g *GormProjectRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&payment.Payment{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return uint(count), nil
}

func (g *GormProjectRepository) GetAll(ctx context.Context) ([]*project.Project, error) {
	// TODO: use joins
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.Project
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	var entities []*project.Project
	for _, r := range rows {
		p, err := toDomainProject(r)
		if err != nil {
			return nil, err
		}
		entities = append(entities, p)
	}
	return entities, nil
}

func (g *GormProjectRepository) GetByID(ctx context.Context, id uint) (*project.Project, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity project.Project
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormProjectRepository) Create(ctx context.Context, data *project.Project) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	entity, err := toDbProject(data)
	if err != nil {
		return err
	}
	return tx.Create(entity).Error
}

func (g *GormProjectRepository) Update(ctx context.Context, data *project.Project) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Save(data).Error
}

func (g *GormProjectRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Delete(&payment.Payment{}, id).Error
}
