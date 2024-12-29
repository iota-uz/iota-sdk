package persistence

import (
	"context"
	prompt2 "github.com/iota-uz/iota-sdk/modules/core/domain/entities/prompt"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/graphql/helpers"
)

type GormPromptRepository struct{}

func NewPromptRepository() prompt2.Repository {
	return &GormPromptRepository{}
}

func (g *GormPromptRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*prompt2.Prompt, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	q := tx.Limit(limit).Offset(offset)
	q, err := helpers.ApplySort(q, sortBy)
	if err != nil {
		return nil, err
	}
	var entities []*prompt2.Prompt
	if err := q.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormPromptRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&prompt2.Prompt{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormPromptRepository) GetAll(ctx context.Context) ([]*prompt2.Prompt, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entities []*prompt2.Prompt
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormPromptRepository) GetByID(ctx context.Context, id string) (*prompt2.Prompt, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entity prompt2.Prompt
	if err := tx.First(&entity, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormPromptRepository) Create(ctx context.Context, data *prompt2.Prompt) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	if err := tx.Create(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormPromptRepository) Update(ctx context.Context, data *prompt2.Prompt) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	if err := tx.Save(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormPromptRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	if err := tx.Delete(&prompt2.Prompt{}, id).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
