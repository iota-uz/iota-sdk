package persistence

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/token"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	selectRefreshTokenQuery = `
		SELECT
			id, token_hash, client_id, user_id, tenant_id,
			scopes, audience, auth_time, amr, expires_at, created_at
		FROM oidc_refresh_tokens
	`

	insertRefreshTokenQuery = `
		INSERT INTO oidc_refresh_tokens (
			id, token_hash, client_id, user_id, tenant_id,
			scopes, audience, auth_time, amr, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	deleteRefreshTokenQuery                = `DELETE FROM oidc_refresh_tokens WHERE id = $1`
	deleteRefreshTokenByHashQuery          = `DELETE FROM oidc_refresh_tokens WHERE token_hash = $1`
	deleteRefreshTokenByUserAndClientQuery = `DELETE FROM oidc_refresh_tokens WHERE user_id = $1 AND client_id = $2`
	deleteExpiredRefreshTokensQuery        = `DELETE FROM oidc_refresh_tokens WHERE expires_at < NOW()`
)

type TokenRepository struct{}

func NewTokenRepository() token.Repository {
	return &TokenRepository{}
}

func (r *TokenRepository) Create(ctx context.Context, t token.RefreshToken) error {
	const op serrors.Op = "TokenRepository.Create"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	dbToken := ToDBRefreshToken(t)

	_, err = tx.Exec(
		ctx,
		insertRefreshTokenQuery,
		dbToken.ID,
		dbToken.TokenHash,
		dbToken.ClientID,
		dbToken.UserID,
		dbToken.TenantID,
		dbToken.Scopes,
		dbToken.Audience,
		dbToken.AuthTime,
		dbToken.AMR,
		dbToken.ExpiresAt,
		dbToken.CreatedAt,
	)

	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

func (r *TokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (token.RefreshToken, error) {
	const op serrors.Op = "TokenRepository.GetByTokenHash"

	query := selectRefreshTokenQuery + " WHERE token_hash = $1"
	tokens, err := r.queryTokens(ctx, op, query, tokenHash)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if len(tokens) == 0 {
		return nil, serrors.E(op, serrors.NotFound, "refresh token not found")
	}

	return tokens[0], nil
}

func (r *TokenRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "TokenRepository.Delete"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, deleteRefreshTokenQuery, id)
	if err != nil {
		return serrors.E(op, err)
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, serrors.NotFound, "refresh token not found")
	}

	return nil
}

func (r *TokenRepository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	const op serrors.Op = "TokenRepository.DeleteByTokenHash"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, deleteRefreshTokenByHashQuery, tokenHash)
	if err != nil {
		return serrors.E(op, err)
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, serrors.NotFound, "refresh token not found")
	}

	return nil
}

func (r *TokenRepository) DeleteByUserAndClient(ctx context.Context, userID int, clientID string) error {
	const op serrors.Op = "TokenRepository.DeleteByUserAndClient"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	// Delete all refresh tokens for this user + client combination
	_, err = tx.Exec(ctx, deleteRefreshTokenByUserAndClientQuery, userID, clientID)
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

func (r *TokenRepository) DeleteExpired(ctx context.Context) error {
	const op serrors.Op = "TokenRepository.DeleteExpired"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	_, err = tx.Exec(ctx, deleteExpiredRefreshTokensQuery)
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// Helper methods

func (r *TokenRepository) queryTokens(ctx context.Context, op serrors.Op, query string, args ...interface{}) ([]token.RefreshToken, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	var tokens []token.RefreshToken
	for rows.Next() {
		var dbToken models.RefreshToken
		if err := rows.Scan(
			&dbToken.ID,
			&dbToken.TokenHash,
			&dbToken.ClientID,
			&dbToken.UserID,
			&dbToken.TenantID,
			&dbToken.Scopes,
			&dbToken.Audience,
			&dbToken.AuthTime,
			&dbToken.AMR,
			&dbToken.ExpiresAt,
			&dbToken.CreatedAt,
		); err != nil {
			return nil, serrors.E(op, err)
		}

		domainToken, err := ToDomainRefreshToken(&dbToken)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		tokens = append(tokens, domainToken)
	}

	if err := rows.Err(); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []token.RefreshToken{}, nil
		}
		return nil, serrors.E(op, err)
	}

	return tokens, nil
}
