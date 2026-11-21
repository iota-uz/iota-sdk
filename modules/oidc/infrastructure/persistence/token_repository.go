// Package persistence provides this package.
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
		FROM oidc.refresh_tokens
	`

	insertRefreshTokenQuery = `
		INSERT INTO oidc.refresh_tokens (
			id, token_hash, client_id, user_id, tenant_id,
			scopes, audience, auth_time, amr, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	lockRefreshGrantQuery    = `SELECT pg_advisory_xact_lock(hashtextextended(format('oidc-refresh-grant:%s:%s', $1::bigint, $2::text), 0))`
	grantOfRefreshTokenQuery = `SELECT user_id, client_id FROM oidc.refresh_tokens WHERE token_hash = $1`

	consumeRefreshTokenQuery = `
		UPDATE oidc.refresh_tokens SET revoked_at = NOW(), replaced_by = $2
		WHERE token_hash = $1 AND revoked_at IS NULL`
	// The rotation chain starting at a consumed token: every replacement it
	// led to, however many rotations deep.
	revokeReusedRefreshTokenQuery = `
		WITH RECURSIVE chain AS (
			SELECT id, replaced_by FROM oidc.refresh_tokens
			WHERE token_hash = $1 AND replaced_by IS NOT NULL
			UNION
			SELECT t.id, t.replaced_by FROM oidc.refresh_tokens t
			JOIN chain c ON t.id = c.replaced_by
		)
		UPDATE oidc.refresh_tokens SET revoked_at = COALESCE(revoked_at, NOW())
		WHERE id IN (SELECT id FROM chain)`

	deleteRefreshTokenQuery                = `DELETE FROM oidc.refresh_tokens WHERE id = $1`
	deleteRefreshTokenByHashQuery          = `DELETE FROM oidc.refresh_tokens WHERE token_hash = $1`
	deleteRefreshTokenByUserAndClientQuery = `DELETE FROM oidc.refresh_tokens WHERE user_id = $1 AND client_id = $2`
	deleteExpiredRefreshTokensQuery        = `DELETE FROM oidc.refresh_tokens WHERE expires_at < NOW()`
	deleteRefreshTokensByUserQuery         = `DELETE FROM oidc.refresh_tokens WHERE user_id = $1`
)

type TokenRepository struct{}

func NewTokenRepository() token.Repository {
	return &TokenRepository{}
}

func (r *TokenRepository) Create(ctx context.Context, t token.RefreshToken) error {
	const op serrors.Op = "TokenRepository.Create"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
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
		return serrors.FromDB(op, err)
	}

	return nil
}

func (r *TokenRepository) GetByTokenHash(ctx context.Context, tokenHash string) (token.RefreshToken, error) {
	const op serrors.Op = "TokenRepository.GetByTokenHash"

	query := selectRefreshTokenQuery + " WHERE token_hash = $1 AND revoked_at IS NULL"
	tokens, err := r.queryTokens(ctx, op, query, tokenHash)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}

	if len(tokens) == 0 {
		return nil, serrors.New(serrors.NotFound, "refresh token not found").WithOp(op)
	}

	return tokens[0], nil
}

func (r *TokenRepository) Consume(ctx context.Context, tokenHash string, replacedBy uuid.UUID) error {
	const op serrors.Op = "TokenRepository.Consume"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	result, err := tx.Exec(ctx, consumeRefreshTokenQuery, tokenHash, replacedBy)
	if err != nil {
		return serrors.FromDB(op, err)
	}
	if result.RowsAffected() == 0 {
		return serrors.New(serrors.NotFound, "refresh token not found").WithOp(op)
	}
	return nil
}

func (r *TokenRepository) LockGrant(ctx context.Context, userID int, clientID string) error {
	const op serrors.Op = "TokenRepository.LockGrant"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if _, err := tx.Exec(ctx, lockRefreshGrantQuery, userID, clientID); err != nil {
		return serrors.FromDB(op, err)
	}
	return nil
}

func (r *TokenRepository) RevokeReused(ctx context.Context, tokenHash string) (bool, error) {
	const op serrors.Op = "TokenRepository.RevokeReused"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, serrors.Wrap(op, err)
	}
	var userID int
	var clientID string
	if err := tx.QueryRow(ctx, grantOfRefreshTokenQuery, tokenHash).Scan(&userID, &clientID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, serrors.FromDB(op, err)
	}
	// After the lock, a rotation of this grant has either committed (its
	// replacement is in the chain snapshot below) or waits for this revoke.
	if err := r.LockGrant(ctx, userID, clientID); err != nil {
		return false, serrors.Wrap(op, err)
	}
	result, err := tx.Exec(ctx, revokeReusedRefreshTokenQuery, tokenHash)
	if err != nil {
		return false, serrors.FromDB(op, err)
	}
	return result.RowsAffected() > 0, nil
}

func (r *TokenRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "TokenRepository.Delete"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}

	result, err := tx.Exec(ctx, deleteRefreshTokenQuery, id)
	if err != nil {
		return serrors.FromDB(op, err)
	}

	if result.RowsAffected() == 0 {
		return serrors.New(serrors.NotFound, "refresh token not found").WithOp(op)
	}

	return nil
}

func (r *TokenRepository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	const op serrors.Op = "TokenRepository.DeleteByTokenHash"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}

	result, err := tx.Exec(ctx, deleteRefreshTokenByHashQuery, tokenHash)
	if err != nil {
		return serrors.FromDB(op, err)
	}

	if result.RowsAffected() == 0 {
		return serrors.New(serrors.NotFound, "refresh token not found").WithOp(op)
	}

	return nil
}

func (r *TokenRepository) DeleteByUserID(ctx context.Context, userID int) error {
	const op serrors.Op = "TokenRepository.DeleteByUserID"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if _, err := tx.Exec(ctx, deleteRefreshTokensByUserQuery, userID); err != nil {
		return serrors.FromDB(op, err)
	}
	return nil
}

func (r *TokenRepository) DeleteByUserAndClient(ctx context.Context, userID int, clientID string) error {
	const op serrors.Op = "TokenRepository.DeleteByUserAndClient"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}

	// Delete all refresh tokens for this user + client combination
	_, err = tx.Exec(ctx, deleteRefreshTokenByUserAndClientQuery, userID, clientID)
	if err != nil {
		return serrors.FromDB(op, err)
	}

	return nil
}

func (r *TokenRepository) DeleteExpired(ctx context.Context) error {
	const op serrors.Op = "TokenRepository.DeleteExpired"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}

	_, err = tx.Exec(ctx, deleteExpiredRefreshTokensQuery)
	if err != nil {
		return serrors.FromDB(op, err)
	}

	return nil
}

// Helper methods

func (r *TokenRepository) queryTokens(ctx context.Context, op serrors.Op, query string, args ...interface{}) ([]token.RefreshToken, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, serrors.FromDB(op, err)
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
			return nil, serrors.FromDB(op, err)
		}

		domainToken, err := ToDomainRefreshToken(&dbToken)
		if err != nil {
			return nil, serrors.Wrap(op, err)
		}
		tokens = append(tokens, domainToken)
	}

	if err := rows.Err(); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []token.RefreshToken{}, nil
		}
		return nil, serrors.FromDB(op, err)
	}

	return tokens, nil
}
