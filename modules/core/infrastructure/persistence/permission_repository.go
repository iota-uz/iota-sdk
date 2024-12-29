package persistence

import (
	"context"
	permission2 "github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/graphql/helpers"
)

type GormPermissionRepository struct{}

func NewPermissionRepository() permission2.Repository {
	return &GormPermissionRepository{}
}

func (g *GormPermissionRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*permission2.Permission, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy)
	if err != nil {
		return nil, err
	}
	var rows []*models.Permission
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*permission2.Permission, len(rows))
	for i, row := range rows {
		entities[i] = toDomainPermission(row)
	}
	return entities, nil
}

func (g *GormPermissionRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Permission{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormPermissionRepository) GetAll(ctx context.Context) ([]*permission2.Permission, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var rows []*models.Permission
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	entities := make([]*permission2.Permission, len(rows))
	for i, row := range rows {
		entities[i] = toDomainPermission(row)
	}
	return entities, nil
}

func (g *GormPermissionRepository) GetByID(ctx context.Context, id uint) (*permission2.Permission, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var row models.Permission
	if err := tx.First(&row, id).Error; err != nil {
		return nil, err
	}
	return toDomainPermission(&row), nil
}

func (g *GormPermissionRepository) CreateOrUpdate(ctx context.Context, data *permission2.Permission) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Save(toDBPermission(data)).Error
}

func (g *GormPermissionRepository) Create(ctx context.Context, data *permission2.Permission) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Create(toDBPermission(data)).Error
}

func (g *GormPermissionRepository) Update(ctx context.Context, data *permission2.Permission) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Updates(toDBPermission(data)).Error
}

func (g *GormPermissionRepository) Delete(ctx context.Context, id uint) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Delete(&models.Permission{}, id).Error //nolint:exhaustruct
}
