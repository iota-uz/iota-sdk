package persistence

import (
	"context"
	"github.com/iota-agency/iota-erp/pkg/composables"

	"github.com/iota-agency/iota-erp/internal/domain/entities/authlog"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormAuthLogRepository struct{}

func NewAuthLogRepository() authlog.Repository {
	return &GormAuthLogRepository{}
}

func (g *GormAuthLogRepository) GetPaginated(
	ctx context.Context,
	limit,
	offset int,
	sortBy []string,
) ([]*authlog.AuthenticationLog, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy, &authlog.AuthenticationLog{}) //nolint:exhaustruct
	if err != nil {
		return nil, err
	}
	var entities []*authlog.AuthenticationLog
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormAuthLogRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&authlog.AuthenticationLog{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormAuthLogRepository) GetAll(ctx context.Context) ([]*authlog.AuthenticationLog, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*authlog.AuthenticationLog
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormAuthLogRepository) GetByID(ctx context.Context, id int64) (*authlog.AuthenticationLog, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity authlog.AuthenticationLog
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormAuthLogRepository) Create(ctx context.Context, data *authlog.AuthenticationLog) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormAuthLogRepository) Update(ctx context.Context, data *authlog.AuthenticationLog) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Save(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormAuthLogRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&authlog.AuthenticationLog{}, id).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
