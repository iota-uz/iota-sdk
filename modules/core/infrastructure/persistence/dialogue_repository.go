package persistence

import (
	"context"
<<<<<<< Updated upstream
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/dialogue"
	"github.com/iota-uz/iota-sdk/pkg/composables"
=======
	"fmt"
	"github.com/go-faster/errors"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/dialogue"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrDialogueNotFound = errors.New("dialogue not found")
)

const (
	dialogueFindQuery = `
		SELECT id,
		       user_id,
		       label,
		       messages,
		       created_at,
		       updated_at
		  FROM dialogues`

	dialogueCountQuery = `SELECT COUNT(*) as count FROM dialogues`

	dialogueInsertQuery = `
		INSERT INTO dialogues (
			user_id,
			label,
			messages,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5) RETURNING id`

	dialogueUpdateQuery = `
		UPDATE dialogues SET 
			   label = $1,
		       messages = $2,
		       updated_at = $3
		 WHERE id = $4`

	dialogueDeleteQuery = `DELETE FROM dialogues WHERE id = $1`
>>>>>>> Stashed changes
)

type GormDialogueRepository struct{}

<<<<<<< Updated upstream
=======
func (g *GormDialogueRepository) GetByUserID(ctx context.Context, userID uint) ([]dialogue.Dialogue, error) {
	return g.queryDialogues(ctx, repo.Join(dialogueFindQuery, "WHERE user_id = $1"), userID)
}

>>>>>>> Stashed changes
func NewDialogueRepository() dialogue.Repository {
	return &GormDialogueRepository{}
}

<<<<<<< Updated upstream
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
=======
func (g *GormDialogueRepository) GetPaginated(ctx context.Context, params *dialogue.FindParams) ([]dialogue.Dialogue, error) {
	var args []interface{}
	where := []string{"1 = 1"}

	if params.Query != "" && params.Field != "" {
		where = append(where, fmt.Sprintf("%s::VARCHAR ILIKE $%d", params.Field, len(where)))
		args = append(args, "%"+params.Query+"%")
	}

	q := repo.Join(
		dialogueFindQuery,
		repo.JoinWhere(where...),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)
	return g.queryDialogues(ctx, q, args...)
}

func (g *GormDialogueRepository) Count(ctx context.Context) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := tx.QueryRow(ctx, dialogueCountQuery).Scan(&count); err != nil {
>>>>>>> Stashed changes
		return 0, err
	}
	return count, nil
}

<<<<<<< Updated upstream
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
=======
func (g *GormDialogueRepository) GetAll(ctx context.Context) ([]dialogue.Dialogue, error) {
	return g.queryDialogues(ctx, dialogueFindQuery)
}

func (g *GormDialogueRepository) GetByID(ctx context.Context, id uint) (dialogue.Dialogue, error) {
	dialogues, err := g.queryDialogues(ctx, repo.Join(dialogueFindQuery, "WHERE id = $1"), id)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get dialogue by id")
	}
	if len(dialogues) == 0 {
		return nil, ErrDialogueNotFound
	}
	return dialogues[0], nil
}

func (g *GormDialogueRepository) Create(ctx context.Context, d dialogue.Dialogue) (dialogue.Dialogue, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	dbDialogue, err := toDBDialogue(d)
	if err != nil {
		return nil, err
	}
	row := tx.QueryRow(
		ctx,
		dialogueInsertQuery,
		dbDialogue.UserID,
		dbDialogue.Label,
		dbDialogue.Messages,
		dbDialogue.CreatedAt,
		dbDialogue.UpdatedAt,
	)

	var id uint
	if err := row.Scan(&id); err != nil {
		return nil, errors.Wrap(err, "failed to create dialogue")
	}

	return g.GetByID(ctx, id)
}

func (g *GormDialogueRepository) Update(ctx context.Context, d dialogue.Dialogue) error {
	dbDialogue, err := toDBDialogue(d)
	if err != nil {
		return err
	}
	return g.execQuery(
		ctx,
		dialogueUpdateQuery,
		dbDialogue.Label,
		dbDialogue.Messages,
		dbDialogue.UpdatedAt,
		dbDialogue.ID,
	)
}

func (g *GormDialogueRepository) Delete(ctx context.Context, id uint) error {
	return g.execQuery(ctx, dialogueDeleteQuery, id)
}

func (g *GormDialogueRepository) queryDialogues(ctx context.Context, query string, args ...interface{}) ([]dialogue.Dialogue, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dialogues []dialogue.Dialogue
	for rows.Next() {
		var d models.Dialogue
		if err := rows.Scan(
			&d.ID,
			&d.UserID,
			&d.Label,
			&d.Messages,
			&d.CreatedAt,
			&d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		entity, err := toDomainDialogue(&d)
		if err != nil {
			return nil, err
		}
		dialogues = append(dialogues, entity)
	}
	return dialogues, nil
}

func (g *GormDialogueRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
>>>>>>> Stashed changes
}
