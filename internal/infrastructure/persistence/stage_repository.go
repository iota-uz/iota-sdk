package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence/models"
	"github.com/iota-agency/iota-erp/pkg/composables"

	stage "github.com/iota-agency/iota-erp/internal/domain/entities/project_stages"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
)

func NewProjectStageRepository() stage.Repository {
	return &GormStageRepository{}
}

type GormStageRepository struct{}

func (g *GormStageRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*stage.ProjectStage, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy, &stage.ProjectStage{}) //nolint:exhaustruct
	if err != nil {
		return nil, err
	}
	var rows []*models.ProjectStage
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	stages := make([]*stage.ProjectStage, len(rows))
	for i, row := range rows {
		stages[i] = toDomainProjectStage(row)
	}
	return stages, nil
}

func (g *GormStageRepository) Count(ctx context.Context) (uint, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.ProjectStage{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return uint(count), nil
}

func (g *GormStageRepository) GetAll(ctx context.Context) ([]*stage.ProjectStage, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.ProjectStage
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	stages := make([]*stage.ProjectStage, len(rows))
	for i, row := range rows {
		stages[i] = toDomainProjectStage(row)
	}
	return stages, nil
}

func (g *GormStageRepository) GetByID(ctx context.Context, id uint) (*stage.ProjectStage, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	entity := &models.ProjectStage{} //nolint:exhaustruct
	if err := tx.First(entity, id).Error; err != nil {
		return nil, err
	}
	return toDomainProjectStage(entity), nil
}

func (g *GormStageRepository) Create(ctx context.Context, entity *stage.ProjectStage) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(toDBProjectStage(entity)).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormStageRepository) Update(ctx context.Context, entity *stage.ProjectStage) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Save(toDBProjectStage(entity)).Error
}

func (g *GormStageRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&models.ProjectStage{}, id).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
