package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/domain/entities/position"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/persistence/models"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormPositionRepository struct{}

func NewPositionRepository() position.Repository {
	return &GormPositionRepository{}
}

func (g *GormPositionRepository) GetPaginated(
	ctx context.Context, limit, offset int,
	sortBy []string,
) ([]*position.Position, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy, &position.Position{}) //nolint:exhaustruct
	if err != nil {
		return nil, err
	}
	var entities []*models.WarehousePosition
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	positions := make([]*position.Position, len(entities))
	for i, entity := range entities {
		p, err := toDomainPosition(entity)
		if err != nil {
			return nil, err
		}
		positions[i] = p
	}
	return positions, nil
}

func (g *GormPositionRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.WarehousePosition{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormPositionRepository) GetAll(ctx context.Context) ([]*position.Position, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*models.WarehousePosition
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	positions := make([]*position.Position, len(entities))
	for i, entity := range entities {
		p, err := toDomainPosition(entity)
		if err != nil {
			return nil, err
		}
		positions[i] = p
	}
	return positions, nil
}

func (g *GormPositionRepository) GetByID(ctx context.Context, id uint) (*position.Position, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.WarehousePosition
	if err := tx.Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return toDomainPosition(&entity)
}

func (g *GormPositionRepository) Create(ctx context.Context, data *position.Position) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(toDBPosition(data)).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormPositionRepository) Update(ctx context.Context, data *position.Position) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Save(toDBPosition(data)).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormPositionRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Where("id = ?", id).Delete(&models.WarehousePosition{}).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
