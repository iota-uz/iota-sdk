package persistence

import (
	"context"

	"github.com/iota-agency/iota-erp/internal/domain/entities/upload"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/service"
)

type GormUploadRepository struct{}

func NewUploadRepository() upload.Repository {
	return &GormUploadRepository{}
}

func (g *GormUploadRepository) GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]*upload.Upload, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var uploads []*upload.Upload
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	if err := q.Find(&uploads).Error; err != nil {
		return nil, err
	}
	return uploads, nil
}

func (g *GormUploadRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&upload.Upload{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormUploadRepository) GetAll(ctx context.Context) ([]*upload.Upload, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*upload.Upload
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormUploadRepository) GetByID(ctx context.Context, id int64) (*upload.Upload, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity upload.Upload
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormUploadRepository) Create(ctx context.Context, data *upload.Upload) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormUploadRepository) Update(ctx context.Context, data *upload.Upload) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Save(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormUploadRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&upload.Upload{}, id).Error; err != nil {
		return err
	}
	return nil
}
