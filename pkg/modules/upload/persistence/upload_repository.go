package persistence

import (
	"context"

	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	"github.com/iota-agency/iota-sdk/pkg/service"

	"github.com/iota-agency/iota-sdk/pkg/modules/upload/domain/entities/upload"
	"github.com/iota-agency/iota-sdk/pkg/modules/upload/persistence/models"
)

type GormUploadRepository struct{}

func NewUploadRepository() upload.Repository {
	return &GormUploadRepository{}
}

func (g *GormUploadRepository) GetPaginated(
	ctx context.Context, params *upload.FindParams,
) ([]*upload.Upload, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(params.Limit).Offset(params.Offset)
	q, err := helpers.ApplySort(q, params.SortBy)
	if err != nil {
		return nil, err
	}
	var entities []*models.Upload
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	uploads := make([]*upload.Upload, len(entities))
	for i, entity := range entities {
		// TODO: proper implementation
		u, err := g.GetByID(ctx, entity.ID)
		if err != nil {
			return nil, err
		}
		uploads[i] = u
	}
	return uploads, nil
}

func (g *GormUploadRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&models.Upload{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormUploadRepository) GetAll(ctx context.Context) ([]*upload.Upload, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*models.Upload
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}

	orders := make([]*upload.Upload, len(entities))
	for i, entity := range entities {
		// TODO: proper implementation
		o, err := g.GetByID(ctx, entity.ID)
		if err != nil {
			return nil, err
		}
		orders[i] = o
	}
	return orders, nil
}

func (g *GormUploadRepository) GetByID(ctx context.Context, id string) (*upload.Upload, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity models.Upload
	if err := tx.Where("id = ?", id).First(&entity).Error; err != nil {
		return nil, err
	}
	var uploads []*models.Upload
	if err := tx.Where("order_id = ?", entity.ID).Find(&uploads).Error; err != nil {
		return nil, err
	}
	u, err := toDomainUpload(&entity)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (g *GormUploadRepository) Create(ctx context.Context, data *upload.Upload) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	upload := toDBUpload(data)
	if err := tx.Create(upload).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormUploadRepository) Update(ctx context.Context, data *upload.Upload) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	upload := toDBUpload(data)
	if err := tx.Save(upload).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormUploadRepository) Delete(ctx context.Context, id string) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Where("id = ?", id).Delete(&models.Upload{}).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
