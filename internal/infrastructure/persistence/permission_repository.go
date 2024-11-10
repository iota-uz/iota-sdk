package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence/models"
	"github.com/iota-agency/iota-erp/pkg/composables"

	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormPermissionRepository struct{}

func NewPermissionRepository() permission.Repository {
	return &GormPermissionRepository{}
}

func (g *GormPermissionRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*permission.Permission, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy, &permission.Permission{}) //nolint:exhaustruct
	if err != nil {
		return nil, err
	}
	var rows []*models.Permission
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*permission.Permission, len(rows))
	for i, row := range rows {
		entities[i] = toDomainPermission(row)
	}
	return entities, nil
}

func (g *GormPermissionRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Permission{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormPermissionRepository) GetAll(ctx context.Context) ([]*permission.Permission, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.Permission
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*permission.Permission, len(rows))
	for i, row := range rows {
		entities[i] = toDomainPermission(row)
	}
	return entities, nil
}

func (g *GormPermissionRepository) GetByID(ctx context.Context, id uint) (*permission.Permission, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var row models.Permission
	if err := tx.First(&row, id).Error; err != nil {
		return nil, err
	}
	return toDomainPermission(&row), nil
}

func (g *GormPermissionRepository) CreateOrUpdate(ctx context.Context, data *permission.Permission) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Save(toDBPermission(data)).Error
}

func (g *GormPermissionRepository) Create(ctx context.Context, data *permission.Permission) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Create(toDBPermission(data)).Error
}

func (g *GormPermissionRepository) Update(ctx context.Context, data *permission.Permission) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Updates(toDBPermission(data)).Error
}

func (g *GormPermissionRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Delete(&models.Permission{}, id).Error //nolint:exhaustruct
}
