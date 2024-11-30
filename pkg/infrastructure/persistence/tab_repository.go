package persistence

import (
	"context"

	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/tab"
	"github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	"github.com/iota-agency/iota-sdk/pkg/infrastructure/persistence/models"
	"github.com/iota-agency/iota-sdk/pkg/service"
)

type GormTabRepository struct{}

func NewTabRepository() tab.Repository {
	return &GormTabRepository{}
}

func (g *GormTabRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Tab{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormTabRepository) GetAll(ctx context.Context, params *tab.FindParams) ([]*tab.Tab, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q, err := helpers.ApplySort(tx, params.SortBy)
	if err != nil {
		return nil, err
	}
	if params.UserID != 0 {
		q.Where("user_id = ?", params.UserID)
	}
	var entities []*models.Tab
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}

	tabs := make([]*tab.Tab, len(entities))
	for i, entity := range entities {
		tabs[i] = ToDomainTab(entity)
	}
	return tabs, nil
}

func (g *GormTabRepository) GetByID(ctx context.Context, id uint) (*tab.Tab, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.Tab
	if err := tx.Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	return ToDomainTab(&entity), nil
}

func (g *GormTabRepository) Create(ctx context.Context, data *tab.Tab) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	tab := ToDBTab(data)
	if err := tx.Create(tab).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormTabRepository) Update(ctx context.Context, data *tab.Tab) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	tab := ToDBTab(data)
	if err := tx.Save(tab).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormTabRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Where("id = ?", id).Delete(&models.Tab{}).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
