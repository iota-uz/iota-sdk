package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/applets"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresDBStore struct {
	pool *pgxpool.Pool
}

func NewPostgresDBStore(pool *pgxpool.Pool) (*PostgresDBStore, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres pool is required")
	}
	return &PostgresDBStore{pool: pool}, nil
}

func (s *PostgresDBStore) Get(ctx context.Context, id string) (any, error) {
	tenantID, appletID := tenantAndAppletFromContext(ctx)
	row := s.pool.QueryRow(ctx, `
		SELECT table_name, value
		FROM applet_engine_documents
		WHERE tenant_id = $1 AND applet_id = $2 AND document_id = $3
	`, tenantID, appletID, id)

	var tableName string
	var rawValue []byte
	if err := row.Scan(&tableName, &rawValue); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("postgres db.get: %w", err)
	}
	value, err := decodeJSONValue(rawValue)
	if err != nil {
		return nil, err
	}
	return map[string]any{"_id": id, "table": tableName, "value": value}, nil
}

func (s *PostgresDBStore) Query(ctx context.Context, table string) ([]any, error) {
	tenantID, appletID := tenantAndAppletFromContext(ctx)
	rows, err := s.pool.Query(ctx, `
		SELECT document_id, table_name, value
		FROM applet_engine_documents
		WHERE tenant_id = $1 AND applet_id = $2 AND table_name = $3
		ORDER BY updated_at DESC
	`, tenantID, appletID, table)
	if err != nil {
		return nil, fmt.Errorf("postgres db.query: %w", err)
	}
	defer rows.Close()

	result := make([]any, 0)
	for rows.Next() {
		var id, tableName string
		var rawValue []byte
		if err := rows.Scan(&id, &tableName, &rawValue); err != nil {
			return nil, fmt.Errorf("postgres db.query scan: %w", err)
		}
		value, err := decodeJSONValue(rawValue)
		if err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"_id": id, "table": tableName, "value": value})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres db.query rows: %w", err)
	}
	return result, nil
}

func (s *PostgresDBStore) Insert(ctx context.Context, table string, value any) (any, error) {
	tenantID, appletID := tenantAndAppletFromContext(ctx)
	documentID := uuid.NewString()
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal db.insert value: %w", err)
	}

	if _, err := s.pool.Exec(ctx, `
		INSERT INTO applet_engine_documents(tenant_id, applet_id, table_name, document_id, value)
		VALUES ($1, $2, $3, $4, $5::jsonb)
	`, tenantID, appletID, table, documentID, string(encoded)); err != nil {
		return nil, fmt.Errorf("postgres db.insert: %w", err)
	}
	return map[string]any{"_id": documentID, "table": table, "value": value}, nil
}

func (s *PostgresDBStore) Patch(ctx context.Context, id string, value any) (any, error) {
	return s.update(ctx, id, value, false)
}

func (s *PostgresDBStore) Replace(ctx context.Context, id string, value any) (any, error) {
	return s.update(ctx, id, value, true)
}

func (s *PostgresDBStore) update(ctx context.Context, id string, value any, strict bool) (any, error) {
	tenantID, appletID := tenantAndAppletFromContext(ctx)
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal db value: %w", err)
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE applet_engine_documents
		SET value = $4::jsonb, updated_at = NOW()
		WHERE tenant_id = $1 AND applet_id = $2 AND document_id = $3
		RETURNING table_name
	`, tenantID, appletID, id, string(encoded))

	var tableName string
	if err := row.Scan(&tableName); err != nil {
		if err == pgx.ErrNoRows {
			if strict {
				return nil, fmt.Errorf("document not found: %w", applets.ErrNotFound)
			}
			return nil, nil
		}
		return nil, fmt.Errorf("postgres db.update: %w", err)
	}
	return map[string]any{"_id": id, "table": tableName, "value": value}, nil
}

func (s *PostgresDBStore) Delete(ctx context.Context, id string) (bool, error) {
	tenantID, appletID := tenantAndAppletFromContext(ctx)
	commandTag, err := s.pool.Exec(ctx, `
		DELETE FROM applet_engine_documents
		WHERE tenant_id = $1 AND applet_id = $2 AND document_id = $3
	`, tenantID, appletID, id)
	if err != nil {
		return false, fmt.Errorf("postgres db.delete: %w", err)
	}
	return commandTag.RowsAffected() > 0, nil
}

func decodeJSONValue(raw []byte) (any, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("unmarshal db value: %w", err)
	}
	return value, nil
}
