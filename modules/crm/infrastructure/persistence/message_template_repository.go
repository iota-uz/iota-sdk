package persistence

import (
	"context"

	"github.com/go-faster/errors"

	messagetemplate "github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message-template"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrMessageTemplateNotFound = errors.New("message template not found")
)

const (
	selectMessageTemplateQuery = `
		SELECT 
			id,
			template,
			created_at
		FROM message_templates
	`

	countMessageTemplateQuery = `SELECT COUNT(*) as count FROM message_templates`

	insertMessageTemplateQuery = `
		INSERT INTO message_templates (
			template,
			created_at
		) VALUES ($1, $2) RETURNING id`

	updateMessageTemplateQuery = `
		UPDATE message_templates 
		SET template = $1 
		WHERE id = $2`

	deleteMessageTemplateQuery = `DELETE FROM message_templates WHERE id = $1`
)

type MessageTemplateRepository struct {
}

func NewMessageTemplateRepository() messagetemplate.Repository {
	return &MessageTemplateRepository{}
}

func (r *MessageTemplateRepository) queryMessageTemplates(
	ctx context.Context,
	query string, args ...interface{},
) ([]messagetemplate.MessageTemplate, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dbTemplates := make([]*models.MessageTemplate, 0)
	for rows.Next() {
		var t models.MessageTemplate
		if err := rows.Scan(
			&t.ID,
			&t.Template,
			&t.CreatedAt,
		); err != nil {
			return nil, err
		}
		dbTemplates = append(dbTemplates, &t)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return mapping.MapDBModels(dbTemplates, ToDomainMessageTemplate)
}

func (r *MessageTemplateRepository) GetPaginated(
	ctx context.Context,
	params *messagetemplate.FindParams,
) ([]messagetemplate.MessageTemplate, error) {
	return r.queryMessageTemplates(
		ctx,
		repo.Join(
			selectClientQuery,
			repo.FormatLimitOffset(params.Limit, params.Offset),
		),
	)
}

func (r *MessageTemplateRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := pool.QueryRow(ctx, countMessageTemplateQuery).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *MessageTemplateRepository) GetAll(ctx context.Context) ([]messagetemplate.MessageTemplate, error) {
	return r.queryMessageTemplates(ctx, selectMessageTemplateQuery)
}

func (r *MessageTemplateRepository) GetByID(
	ctx context.Context, id uint,
) (messagetemplate.MessageTemplate, error) {
	templates, err := r.queryMessageTemplates(ctx, selectMessageTemplateQuery+` WHERE id = $1`, id)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, ErrMessageTemplateNotFound
	}
	return templates[0], nil
}

func (r *MessageTemplateRepository) Create(ctx context.Context, data messagetemplate.MessageTemplate) (messagetemplate.MessageTemplate, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	dbTemplate := ToDBMessageTemplate(data)
	if err := tx.QueryRow(
		ctx,
		insertMessageTemplateQuery,
		dbTemplate.Template,
		dbTemplate.CreatedAt,
	).Scan(&dbTemplate.ID); err != nil {
		return nil, err
	}

	return r.GetByID(ctx, dbTemplate.ID)
}

func (r *MessageTemplateRepository) Update(
	ctx context.Context,
	data messagetemplate.MessageTemplate,
) (messagetemplate.MessageTemplate, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	dbTemplate := ToDBMessageTemplate(data)
	result, err := tx.Exec(
		ctx,
		updateMessageTemplateQuery,
		dbTemplate.Template,
		dbTemplate.ID,
	)
	if err != nil {
		return nil, err
	}

	if result.RowsAffected() == 0 {
		return nil, ErrMessageTemplateNotFound
	}

	return r.GetByID(ctx, dbTemplate.ID)
}

func (r *MessageTemplateRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	result, err := tx.Exec(ctx, deleteMessageTemplateQuery, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return ErrMessageTemplateNotFound
	}

	return nil
}
