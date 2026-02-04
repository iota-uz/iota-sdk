package persistence

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence/models"
)

const (
	insertArtifactQuery = `
		INSERT INTO bichat.artifacts (
			id, tenant_id, session_id, message_id, type, name, description,
			mime_type, url, size_bytes, metadata, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	selectArtifactQuery = `
		SELECT id, tenant_id, session_id, message_id, type, name, description,
			mime_type, url, size_bytes, metadata, created_at
		FROM bichat.artifacts
		WHERE tenant_id = $1 AND id = $2
	`
	selectSessionArtifactsQuery = `
		SELECT id, tenant_id, session_id, message_id, type, name, description,
			mime_type, url, size_bytes, metadata, created_at
		FROM bichat.artifacts
		WHERE tenant_id = $1 AND session_id = $2
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`
	deleteArtifactQuery = `
		DELETE FROM bichat.artifacts
		WHERE tenant_id = $1 AND id = $2
	`
	updateArtifactQuery = `
		UPDATE bichat.artifacts
		SET name = $1, description = $2
		WHERE tenant_id = $3 AND id = $4
	`
)

var ErrArtifactNotFound = errors.New("artifact not found")

// SaveArtifact persists an artifact with tenant isolation.
func (r *PostgresChatRepository) SaveArtifact(ctx context.Context, artifact domain.Artifact) error {
	const op serrors.Op = "PostgresChatRepository.SaveArtifact"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	model, err := models.ArtifactModelFromDomain(artifact)
	if err != nil {
		return serrors.E(op, err, "failed to convert artifact to model")
	}
	var messageID, description, mimeType, url any
	messageID = nil
	description = nil
	mimeType = nil
	url = nil
	if model.MessageID != nil {
		messageID = *model.MessageID
	}
	if model.Description != nil {
		description = *model.Description
	}
	if model.MimeType != nil {
		mimeType = *model.MimeType
	}
	if model.URL != nil {
		url = *model.URL
	}
	metadata := model.Metadata
	if metadata == nil {
		metadata = []byte("{}")
	}

	_, err = tx.Exec(ctx, insertArtifactQuery,
		model.ID,
		tenantID,
		model.SessionID,
		messageID,
		model.Type,
		model.Name,
		description,
		mimeType,
		url,
		model.SizeBytes,
		metadata,
		model.CreatedAt,
	)
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// GetArtifact retrieves an artifact by ID.
func (r *PostgresChatRepository) GetArtifact(ctx context.Context, id uuid.UUID) (domain.Artifact, error) {
	const op serrors.Op = "PostgresChatRepository.GetArtifact"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	var m models.ArtifactModel
	err = tx.QueryRow(ctx, selectArtifactQuery, tenantID, id).Scan(
		&m.ID,
		&m.TenantID,
		&m.SessionID,
		&m.MessageID,
		&m.Type,
		&m.Name,
		&m.Description,
		&m.MimeType,
		&m.URL,
		&m.SizeBytes,
		&m.Metadata,
		&m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serrors.E(op, ErrArtifactNotFound)
		}
		return nil, serrors.E(op, err)
	}

	return m.ToDomain()
}

// GetSessionArtifacts returns artifacts for a session with pagination and optional type filter.
func (r *PostgresChatRepository) GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]domain.Artifact, error) {
	const op serrors.Op = "PostgresChatRepository.GetSessionArtifacts"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	query := selectSessionArtifactsQuery
	args := []any{tenantID, sessionID, limit, offset}

	if len(opts.Types) > 0 {
		// Convert []ArtifactType to []string for SQL query
		typeStrings := make([]string, len(opts.Types))
		for i, t := range opts.Types {
			typeStrings[i] = string(t)
		}
		query = `
		SELECT id, tenant_id, session_id, message_id, type, name, description,
			mime_type, url, size_bytes, metadata, created_at
		FROM bichat.artifacts
		WHERE tenant_id = $1 AND session_id = $2 AND type = ANY($3)
		ORDER BY created_at DESC
		LIMIT $4 OFFSET $5
		`
		args = []any{tenantID, sessionID, typeStrings, limit, offset}
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	var artifacts []domain.Artifact
	for rows.Next() {
		var m models.ArtifactModel
		err := rows.Scan(
			&m.ID,
			&m.TenantID,
			&m.SessionID,
			&m.MessageID,
			&m.Type,
			&m.Name,
			&m.Description,
			&m.MimeType,
			&m.URL,
			&m.SizeBytes,
			&m.Metadata,
			&m.CreatedAt,
		)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		a, err := m.ToDomain()
		if err != nil {
			return nil, serrors.E(op, err)
		}
		artifacts = append(artifacts, a)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	return artifacts, nil
}

// DeleteArtifact removes an artifact by ID.
func (r *PostgresChatRepository) DeleteArtifact(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "PostgresChatRepository.DeleteArtifact"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, deleteArtifactQuery, tenantID, id)
	if err != nil {
		return serrors.E(op, err)
	}
	if result.RowsAffected() == 0 {
		return serrors.E(op, ErrArtifactNotFound)
	}

	return nil
}

// UpdateArtifact updates an artifact's name and description.
func (r *PostgresChatRepository) UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) error {
	const op serrors.Op = "PostgresChatRepository.UpdateArtifact"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, updateArtifactQuery, name, description, tenantID, id)
	if err != nil {
		return serrors.E(op, err)
	}
	if result.RowsAffected() == 0 {
		return serrors.E(op, ErrArtifactNotFound)
	}

	return nil
}
