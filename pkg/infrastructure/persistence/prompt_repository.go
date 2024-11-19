package persistence

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	"github.com/iota-agency/iota-sdk/pkg/service"

	"github.com/iota-agency/iota-sdk/pkg/domain/entities/prompt"
)

type GormPromptRepository struct{}

func NewPromptRepository() prompt.Repository {
	return &GormPromptRepository{}
}

func (g *GormPromptRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*prompt.Prompt, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy)
	if err != nil {
		return nil, err
	}
	var entities []*prompt.Prompt
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormPromptRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, service.ErrNoTx
	}
	var count int64
	if err := tx.Model(&prompt.Prompt{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormPromptRepository) GetAll(ctx context.Context) ([]*prompt.Prompt, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entities []*prompt.Prompt
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormPromptRepository) GetByID(ctx context.Context, id string) (*prompt.Prompt, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, service.ErrNoTx
	}
	var entity prompt.Prompt
	if err := tx.First(&entity, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormPromptRepository) Create(ctx context.Context, data *prompt.Prompt) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Create(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormPromptRepository) Update(ctx context.Context, data *prompt.Prompt) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Save(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormPromptRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return service.ErrNoTx
	}
	if err := tx.Delete(&prompt.Prompt{}, id).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
