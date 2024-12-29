package persistence

import (
	"context"
	authlog2 "github.com/iota-uz/iota-sdk/modules/core/domain/entities/authlog"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/graphql/helpers"
)

type GormAuthLogRepository struct{}

func NewAuthLogRepository() authlog2.Repository {
	return &GormAuthLogRepository{}
}

func (g *GormAuthLogRepository) GetPaginated(
	ctx context.Context,
	limit,
	offset int,
	sortBy []string,
) ([]*authlog2.AuthenticationLog, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy)
	if err != nil {
		return nil, err
	}
	var entities []*authlog2.AuthenticationLog
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormAuthLogRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&authlog2.AuthenticationLog{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormAuthLogRepository) GetAll(ctx context.Context) ([]*authlog2.AuthenticationLog, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entities []*authlog2.AuthenticationLog
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormAuthLogRepository) GetByID(ctx context.Context, id int64) (*authlog2.AuthenticationLog, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entity authlog2.AuthenticationLog
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormAuthLogRepository) Create(ctx context.Context, data *authlog2.AuthenticationLog) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	if err := tx.Create(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormAuthLogRepository) Update(ctx context.Context, data *authlog2.AuthenticationLog) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	if err := tx.Save(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormAuthLogRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	if err := tx.Delete(&authlog2.AuthenticationLog{}, id).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
