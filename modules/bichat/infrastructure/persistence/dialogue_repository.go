package persistence

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/bichat/domain/entities/dialogue"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence/models"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"

	"github.com/go-faster/errors"
)

var (
	ErrDialogueNotFound = errors.New("dialogue not found")
)

const (
	dialogueFindQuery = `
		SELECT id,
		       tenant_id,
		       user_id,
		       label,
		       messages,
		       created_at,
		       updated_at
		  FROM dialogues`

	dialogueCountQuery = `SELECT COUNT(*) as count FROM dialogues`

	dialogueInsertQuery = `
		INSERT INTO dialogues (
			tenant_id,
			user_id,
			label,
			messages,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	dialogueUpdateQuery = `
		UPDATE dialogues SET
			   label = $1,
		       messages = $2,
		       updated_at = $3
		 WHERE id = $4 AND tenant_id = $5`

	dialogueDeleteQuery = `DELETE FROM dialogues WHERE id = $1 AND tenant_id = $2`
)

type GormDialogueRepository struct{}

func (g *GormDialogueRepository) GetByUserID(ctx context.Context, userID uint) ([]dialogue.Dialogue, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	return g.queryDialogues(ctx, dialogueFindQuery+" WHERE user_id = $1 AND tenant_id = $2", userID, tenantID)
}

func NewDialogueRepository() dialogue.Repository {
	return &GormDialogueRepository{}
}

func (g *GormDialogueRepository) GetPaginated(ctx context.Context, params *dialogue.FindParams) ([]dialogue.Dialogue, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	where := []string{"tenant_id = $1"}
	args := []interface{}{tenantID}

	if params.Query != "" && params.Field != "" {
		where = append(where, fmt.Sprintf("%s::VARCHAR ILIKE $%d", params.Field, len(args)+1))
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

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get tenant from context")
	}

	var count int64
	if err := tx.QueryRow(ctx, dialogueCountQuery+" WHERE tenant_id = $1", tenantID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormDialogueRepository) GetAll(ctx context.Context) ([]dialogue.Dialogue, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	return g.queryDialogues(ctx, dialogueFindQuery+" WHERE tenant_id = $1", tenantID)
}

func (g *GormDialogueRepository) GetByID(ctx context.Context, id uint) (dialogue.Dialogue, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	dialogues, err := g.queryDialogues(ctx, repo.Join(dialogueFindQuery, "WHERE id = $1 AND tenant_id = $2"), id, tenantID)
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

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant from context")
	}

	dbDialogue, err := toDBDialogue(d)
	if err != nil {
		return nil, err
	}
	dbDialogue.TenantID = tenantID.String()

	row := tx.QueryRow(
		ctx,
		dialogueInsertQuery,
		dbDialogue.TenantID,
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
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	dbDialogue, err := toDBDialogue(d)
	if err != nil {
		return err
	}
	dbDialogue.TenantID = tenantID.String()

	return g.execQuery(
		ctx,
		dialogueUpdateQuery,
		dbDialogue.Label,
		dbDialogue.Messages,
		dbDialogue.UpdatedAt,
		dbDialogue.ID,
		dbDialogue.TenantID,
	)
}

func (g *GormDialogueRepository) Delete(ctx context.Context, id uint) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	return g.execQuery(ctx, dialogueDeleteQuery, id, tenantID)
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
			&d.TenantID,
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
}
