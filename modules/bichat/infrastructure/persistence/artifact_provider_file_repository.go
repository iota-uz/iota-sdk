package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
)

const (
	selectArtifactProviderFileQuery = `
		SELECT provider_file_id, source_url, source_size_bytes
		FROM bichat.artifact_provider_files
		WHERE tenant_id = $1 AND artifact_id = $2 AND provider = $3
	`
	upsertArtifactProviderFileQuery = `
		INSERT INTO bichat.artifact_provider_files (
			tenant_id, artifact_id, provider, provider_file_id, source_url, source_size_bytes, synced_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $7, $7)
		ON CONFLICT (tenant_id, artifact_id, provider)
		DO UPDATE SET
			provider_file_id = EXCLUDED.provider_file_id,
			source_url = EXCLUDED.source_url,
			source_size_bytes = EXCLUDED.source_size_bytes,
			synced_at = EXCLUDED.synced_at,
			updated_at = EXCLUDED.updated_at
	`
)

var ErrArtifactProviderFileNotFound = errors.New("artifact provider file mapping not found")

// GetArtifactProviderFile returns the provider file mapping for an artifact.
func (r *PostgresChatRepository) GetArtifactProviderFile(
	ctx context.Context,
	artifactID uuid.UUID,
	provider string,
) (providerFileID string, sourceURL string, sourceSizeBytes int64, err error) {
	const op serrors.Op = "PostgresChatRepository.GetArtifactProviderFile"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return "", "", 0, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return "", "", 0, serrors.E(op, err)
	}

	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	if normalizedProvider == "" {
		return "", "", 0, serrors.E(op, serrors.KindValidation, "provider is required")
	}

	err = tx.QueryRow(ctx, selectArtifactProviderFileQuery, tenantID, artifactID, normalizedProvider).
		Scan(&providerFileID, &sourceURL, &sourceSizeBytes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", 0, serrors.E(op, ErrArtifactProviderFileNotFound)
		}
		return "", "", 0, serrors.E(op, err)
	}

	return providerFileID, sourceURL, sourceSizeBytes, nil
}

// UpsertArtifactProviderFile creates or updates the provider file mapping for an artifact.
func (r *PostgresChatRepository) UpsertArtifactProviderFile(
	ctx context.Context,
	artifactID uuid.UUID,
	provider string,
	providerFileID string,
	sourceURL string,
	sourceSizeBytes int64,
) error {
	const op serrors.Op = "PostgresChatRepository.UpsertArtifactProviderFile"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	normalizedProviderFileID := strings.TrimSpace(providerFileID)
	normalizedSourceURL := strings.TrimSpace(sourceURL)
	if normalizedProvider == "" {
		return serrors.E(op, serrors.KindValidation, "provider is required")
	}
	if normalizedProviderFileID == "" {
		return serrors.E(op, serrors.KindValidation, "provider_file_id is required")
	}
	if normalizedSourceURL == "" {
		return serrors.E(op, serrors.KindValidation, "source_url is required")
	}

	now := time.Now()
	_, err = tx.Exec(
		ctx,
		upsertArtifactProviderFileQuery,
		tenantID,
		artifactID,
		normalizedProvider,
		normalizedProviderFileID,
		normalizedSourceURL,
		sourceSizeBytes,
		now,
	)
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}
