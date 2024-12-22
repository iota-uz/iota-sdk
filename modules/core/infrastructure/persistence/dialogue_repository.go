package persistence

import (
	"context"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/dialogue"
)

type GormDialogueRepository struct{}

func NewDialogueRepository() dialogue.Repository {
	return &GormDialogueRepository{}
}

func (g *GormDialogueRepository) GetPaginated(
	ctx context.Context,
	limit, offset int,
	sortBy []string,
) ([]*dialogue.Dialogue, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var uploads []*dialogue.Dialogue
	q := tx.Limit(limit).Offset(offset)
	for _, s := range sortBy {
		q = q.Order(s)
	}
	if err := q.Find(&uploads).Error; err != nil {
		return nil, err
	}
	return uploads, nil
}

func (g *GormDialogueRepository) Count(ctx context.Context) (int64, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return 0, composables.ErrNoTx
	}
	var count int64
	if err := tx.Model(&dialogue.Dialogue{}).Count(&count).Error; err != nil { //nolint:exhaustruct
		return 0, err
	}
	return count, nil
}

func (g *GormDialogueRepository) GetAll(ctx context.Context) ([]*dialogue.Dialogue, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entities []*dialogue.Dialogue
	if err := tx.Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormDialogueRepository) GetByID(ctx context.Context, id int64) (*dialogue.Dialogue, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entity dialogue.Dialogue
	if err := tx.First(&entity, id).Error; err != nil {
		return nil, err
	}
	return &entity, nil
}

func (g *GormDialogueRepository) GetByUserID(ctx context.Context, userID int64) ([]*dialogue.Dialogue, error) {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return nil, composables.ErrNoTx
	}
	var entities []*dialogue.Dialogue
	if err := tx.Where("user_id = ?", userID).Find(&entities).Error; err != nil {
		return nil, err
	}
	return entities, nil
}

func (g *GormDialogueRepository) Create(ctx context.Context, data *dialogue.Dialogue) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	if err := tx.Create(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormDialogueRepository) Update(ctx context.Context, data *dialogue.Dialogue) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	if err := tx.Save(data).Error; err != nil {
		return err
	}
	return nil
}

func (g *GormDialogueRepository) Delete(ctx context.Context, id int64) error {
	tx, ok := composables.UseTx(ctx)
	if !ok {
		return composables.ErrNoTx
	}
	if err := tx.Delete(&dialogue.Dialogue{}, id).Error; err != nil { //nolint:exhaustruct
		return err
	}
	return nil
}
