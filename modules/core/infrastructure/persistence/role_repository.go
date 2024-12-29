package persistence

import (
	"context"
	role2 "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/graphql/helpers"
)

type GormRoleRepository struct{}

func NewRoleRepository() role2.Repository {
	return &GormRoleRepository{}
}

func (g *GormRoleRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*role2.Role, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy)
	if err != nil {
		return nil, err
	}
	var entities []*role2.Role
	if err := q.Preload("Permissions").Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormRoleRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Role{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormRoleRepository) GetAll(ctx context.Context) ([]*role2.Role, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var rows []*models.Role
	if err := tx.Preload("Permissions").Find(&rows).Error; err != nil {
		return nil, err
	}
	var entities []*role2.Role
	for _, row := range rows {
		entities = append(entities, toDomainRole(row))
	}
	return entities, nil
}

func (g *GormRoleRepository) GetByID(ctx context.Context, id int64) (*role2.Role, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entity models.Role
	if err := tx.Preload("Permissions").First(&entity, id).Error; err != nil {
		return nil, err
	}
	return toDomainRole(&entity), nil
}

func (g *GormRoleRepository) CreateOrUpdate(ctx context.Context, data *role2.Role) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	entity, permissions := toDBRole(data)
	if err := tx.Save(entity).Error; err != nil {
		return err
	}
	if err := tx.Model(entity).Association("Permissions").Replace(permissions); err != nil {
		return err
	}
	return nil
}

func (g *GormRoleRepository) Create(ctx context.Context, data *role2.Role) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	entity, permissions := toDBRole(data)
	if err := tx.Create(entity).Error; err != nil {
		return err
	}
	if err := tx.Model(entity).Association("Permissions").Append(permissions); err != nil {
		return err
	}
	return nil
}

func (g *GormRoleRepository) Update(ctx context.Context, data *role2.Role) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	entity, permissions := toDBRole(data)
	if err := tx.Updates(entity).Error; err != nil {
		return err
	}
	if err := tx.Model(entity).Association("Permissions").Replace(permissions); err != nil {
		return err
	}
	return nil
}

func (g *GormRoleRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	return tx.Delete(&models.Role{}, id).Error //nolint:exhaustruct
}
