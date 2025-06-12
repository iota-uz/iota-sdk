package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/go-faster/errors"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

const (
	aiConfigFindQuery = `
		SELECT id,
			tenant_id,
			model_name,
			model_type,
			system_prompt,
			temperature,
			max_tokens,
			base_url,
			access_token,
			is_default,
			created_at,
			updated_at
		FROM ai_chat_configs`

	aiConfigExistsQuery = `SELECT EXISTS(SELECT 1 FROM ai_chat_configs WHERE id = $1)`

	aiConfigInsertQuery = `
		INSERT INTO ai_chat_configs 
		(id, tenant_id, model_name, model_type, system_prompt, temperature, max_tokens, base_url, access_token, is_default, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) 
		RETURNING id`

	aiConfigUpdateQuery = `
		UPDATE ai_chat_configs 
		SET model_name = $1, model_type = $2, system_prompt = $3, temperature = $4, max_tokens = $5, base_url = $6, access_token = $7, is_default = $8, updated_at = $9
		WHERE id = $10
		RETURNING id`

	aiConfigDeleteQuery = `DELETE FROM ai_chat_configs WHERE id = $1`

	aiConfigClearDefaultQuery = `UPDATE ai_chat_configs SET is_default = false, updated_at = $1 WHERE is_default = true AND tenant_id = $2`

	aiConfigSetDefaultQuery = `UPDATE ai_chat_configs SET is_default = true, updated_at = $1 WHERE id = $2`

	aiConfigGetIsDefaultQuery = `SELECT is_default FROM ai_chat_configs WHERE id = $1`
)

type AIChatConfigRepository struct{}

func NewAIChatConfigRepository() aichatconfig.Repository {
	return &AIChatConfigRepository{}
}

func (r *AIChatConfigRepository) GetByID(ctx context.Context, id uuid.UUID) (aichatconfig.AIConfig, error) {
	configs, err := r.queryConfigs(ctx, aiConfigFindQuery+" WHERE id = $1", id.String())
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to query AI chat config with id: %s", id.String()))
	}
	if len(configs) == 0 {
		return nil, aichatconfig.ErrConfigNotFound
	}
	return configs[0], nil
}

func (r *AIChatConfigRepository) GetDefault(ctx context.Context) (aichatconfig.AIConfig, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant ID from context")
	}

	configs, err := r.queryConfigs(ctx, repo.Join(aiConfigFindQuery, "WHERE is_default = true AND tenant_id = $1 LIMIT 1"), tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query default AI chat config")
	}
	if len(configs) == 0 {
		return nil, aichatconfig.ErrConfigNotFound
	}
	return configs[0], nil
}

func (r *AIChatConfigRepository) List(ctx context.Context) ([]aichatconfig.AIConfig, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get tenant ID from context")
	}

	q := repo.Join(aiConfigFindQuery, "WHERE tenant_id = $1 ORDER BY is_default DESC, id ASC")
	configs, err := r.queryConfigs(ctx, q, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list AI chat configs")
	}
	return configs, nil
}

func (r *AIChatConfigRepository) Save(ctx context.Context, config aichatconfig.AIConfig) (aichatconfig.AIConfig, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbConfig := ToDBConfig(config)

	var exists bool
	if config.ID() != uuid.Nil {
		if err := tx.QueryRow(ctx, aiConfigExistsQuery, config.ID().String()).Scan(&exists); err != nil {
			return nil, errors.Wrap(err, "failed to check if config exists")
		}
	}

	if exists {
		rows := tx.QueryRow(
			ctx,
			aiConfigUpdateQuery,
			dbConfig.ModelName,
			dbConfig.ModelType,
			dbConfig.SystemPrompt,
			dbConfig.Temperature,
			dbConfig.MaxTokens,
			dbConfig.BaseURL,
			dbConfig.AccessToken,
			dbConfig.IsDefault,
			dbConfig.UpdatedAt,
			dbConfig.ID,
		).Scan(&dbConfig.ID)
		if rows != nil {
			return nil, errors.Wrap(rows, fmt.Sprintf("failed to update AI chat config with ID: %s", dbConfig.ID))
		}
	} else {
		rows := tx.QueryRow(
			ctx,
			aiConfigInsertQuery,
			dbConfig.ID,
			dbConfig.TenantID,
			dbConfig.ModelName,
			dbConfig.ModelType,
			dbConfig.SystemPrompt,
			dbConfig.Temperature,
			dbConfig.MaxTokens,
			dbConfig.BaseURL,
			dbConfig.AccessToken,
			dbConfig.IsDefault,
			dbConfig.CreatedAt,
			dbConfig.UpdatedAt,
		).Scan(
			&dbConfig.ID,
		)
		if rows != nil {
			return nil, errors.Wrap(rows, "failed to insert AI chat config")
		}
	}

	return ToDomainConfig(dbConfig)
}

func (r *AIChatConfigRepository) SetDefault(ctx context.Context, id uuid.UUID) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	var exists bool
	err = tx.QueryRow(ctx, aiConfigExistsQuery, id.String()).Scan(&exists)
	if err != nil {
		return errors.Wrap(err, "failed to check if config exists")
	}

	if !exists {
		return aichatconfig.ErrConfigNotFound
	}

	now := time.Now()

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant ID from context")
	}

	_, err = tx.Exec(ctx, aiConfigClearDefaultQuery, now, tenantID)
	if err != nil {
		return errors.Wrap(err, "failed to clear default config")
	}

	result, err := tx.Exec(ctx, aiConfigSetDefaultQuery, now, id.String())
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to set config ID %s as default", id.String()))
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return aichatconfig.ErrConfigNotFound
	}

	return nil
}

func (r *AIChatConfigRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	var exists bool
	err = tx.QueryRow(ctx, aiConfigExistsQuery, id.String()).Scan(&exists)
	if err != nil {
		return errors.Wrap(err, "failed to check if config exists")
	}

	if !exists {
		return aichatconfig.ErrConfigNotFound
	}

	var isDefault bool
	err = tx.QueryRow(ctx, aiConfigGetIsDefaultQuery, id.String()).Scan(&isDefault)
	if err != nil {
		return errors.Wrap(err, "failed to check if config is default")
	}

	if isDefault {
		return errors.New("cannot delete default config")
	}

	result, err := tx.Exec(ctx, aiConfigDeleteQuery, id.String())
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to delete AI chat config with ID: %s", id.String()))
	}

	if result.RowsAffected() == 0 {
		return aichatconfig.ErrConfigNotFound
	}

	return nil
}

func (r *AIChatConfigRepository) queryConfigs(ctx context.Context, query string, args ...interface{}) ([]aichatconfig.AIConfig, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute query")
	}
	defer rows.Close()

	var configs []models.AIChatConfig
	for rows.Next() {
		var cfg models.AIChatConfig

		if err := rows.Scan(
			&cfg.ID,
			&cfg.TenantID,
			&cfg.ModelName,
			&cfg.ModelType,
			&cfg.SystemPrompt,
			&cfg.Temperature,
			&cfg.MaxTokens,
			&cfg.BaseURL,
			&cfg.AccessToken,
			&cfg.IsDefault,
			&cfg.CreatedAt,
			&cfg.UpdatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan config row")
		}
		configs = append(configs, cfg)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "row iteration error")
	}

	entities := make([]aichatconfig.AIConfig, 0, len(configs))
	for _, cfg := range configs {
		entity, err := ToDomainConfig(cfg)
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to convert config ID: %s to domain entity", cfg.ID))
		}
		entities = append(entities, entity)
	}

	return entities, nil
}
