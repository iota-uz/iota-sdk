package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/role"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence/models"

	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormRoleRepository struct{}

func NewRoleRepository() role.Repository {
	return &GormRoleRepository{}
}

func (g *GormRoleRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*role.Role, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy, &role.Role{}) //nolint:exhaustruct
	if err != nil {
		return nil, err
	}
	var entities []*role.Role
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormRoleRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Role{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormRoleRepository) GetAll(ctx context.Context) ([]*role.Role, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var rows []*models.Role
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	var entities []*role.Role
	for _, row := range rows {
		entities = append(entities, toDomainRole(row))
	}
	return entities, nil
}

func (g *GormRoleRepository) GetByID(ctx context.Context, id int64) (*role.Role, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity role.Role
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormRoleRepository) CreateOrUpdate(ctx context.Context, data *role.Role) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Save(data).Error
}

func (g *GormRoleRepository) Create(ctx context.Context, data *role.Role) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Create(data).Error
}

func (g *GormRoleRepository) Update(ctx context.Context, data *role.Role) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	return tx.Save(data).Error
}

func (g *GormRoleRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&role.Role{}, id).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
