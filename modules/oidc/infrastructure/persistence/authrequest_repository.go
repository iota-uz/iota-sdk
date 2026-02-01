package persistence

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/authrequest"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	selectAuthRequestQuery = `
		SELECT
			id, client_id, redirect_uri, scopes, state, nonce,
			response_type, code_challenge, code_challenge_method,
			user_id, tenant_id, auth_time, created_at, expires_at
		FROM oidc_auth_requests
	`

	insertAuthRequestQuery = `
		INSERT INTO oidc_auth_requests (
			id, client_id, redirect_uri, scopes, state, nonce,
			response_type, code_challenge, code_challenge_method,
			user_id, tenant_id, auth_time, created_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	updateAuthRequestQuery = `
		UPDATE oidc_auth_requests
		SET client_id = $1, redirect_uri = $2, scopes = $3, state = $4, nonce = $5,
			response_type = $6, code_challenge = $7, code_challenge_method = $8,
			user_id = $9, tenant_id = $10, auth_time = $11, expires_at = $12
		WHERE id = $13
	`

	deleteAuthRequestQuery = `DELETE FROM oidc_auth_requests WHERE id = $1`

	deleteExpiredAuthRequestsQuery = `DELETE FROM oidc_auth_requests WHERE expires_at < NOW()`
)

type AuthRequestRepository struct{}

func NewAuthRequestRepository() authrequest.Repository {
	return &AuthRequestRepository{}
}

func (r *AuthRequestRepository) Create(ctx context.Context, req authrequest.AuthRequest) error {
	const op serrors.Op = "AuthRequestRepository.Create"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	dbReq := ToDBAuthRequest(req)

	_, err = tx.Exec(
		ctx,
		insertAuthRequestQuery,
		dbReq.ID,
		dbReq.ClientID,
		dbReq.RedirectURI,
		dbReq.Scopes,
		dbReq.State,
		dbReq.Nonce,
		dbReq.ResponseType,
		dbReq.CodeChallenge,
		dbReq.CodeChallengeMethod,
		dbReq.UserID,
		dbReq.TenantID,
		dbReq.AuthTime,
		dbReq.CreatedAt,
		dbReq.ExpiresAt,
	)

	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

func (r *AuthRequestRepository) GetByID(ctx context.Context, id uuid.UUID) (authrequest.AuthRequest, error) {
	const op serrors.Op = "AuthRequestRepository.GetByID"

	query := selectAuthRequestQuery + " WHERE id = $1"
	requests, err := r.queryAuthRequests(ctx, op, query, id)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if len(requests) == 0 {
		return nil, serrors.E(op, serrors.NotFound, "auth request not found")
	}

	return requests[0], nil
}

func (r *AuthRequestRepository) Update(ctx context.Context, req authrequest.AuthRequest) error {
	const op serrors.Op = "AuthRequestRepository.Update"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	dbReq := ToDBAuthRequest(req)

	result, err := tx.Exec(
		ctx,
		updateAuthRequestQuery,
		dbReq.ClientID,
		dbReq.RedirectURI,
		dbReq.Scopes,
		dbReq.State,
		dbReq.Nonce,
		dbReq.ResponseType,
		dbReq.CodeChallenge,
		dbReq.CodeChallengeMethod,
		dbReq.UserID,
		dbReq.TenantID,
		dbReq.AuthTime,
		dbReq.ExpiresAt,
		dbReq.ID,
	)

	if err != nil {
		return serrors.E(op, err)
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, serrors.NotFound, "auth request not found")
	}

	return nil
}

func (r *AuthRequestRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "AuthRequestRepository.Delete"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, deleteAuthRequestQuery, id)
	if err != nil {
		return serrors.E(op, err)
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, serrors.NotFound, "auth request not found")
	}

	return nil
}

func (r *AuthRequestRepository) DeleteExpired(ctx context.Context) error {
	const op serrors.Op = "AuthRequestRepository.DeleteExpired"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	_, err = tx.Exec(ctx, deleteExpiredAuthRequestsQuery)
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// Helper methods

func (r *AuthRequestRepository) queryAuthRequests(ctx context.Context, op serrors.Op, query string, args ...interface{}) ([]authrequest.AuthRequest, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	var requests []authrequest.AuthRequest
	for rows.Next() {
		var dbReq models.AuthRequest
		if err := rows.Scan(
			&dbReq.ID,
			&dbReq.ClientID,
			&dbReq.RedirectURI,
			&dbReq.Scopes,
			&dbReq.State,
			&dbReq.Nonce,
			&dbReq.ResponseType,
			&dbReq.CodeChallenge,
			&dbReq.CodeChallengeMethod,
			&dbReq.UserID,
			&dbReq.TenantID,
			&dbReq.AuthTime,
			&dbReq.CreatedAt,
			&dbReq.ExpiresAt,
		); err != nil {
			return nil, serrors.E(op, err)
		}

		domainReq, err := ToDomainAuthRequest(&dbReq)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		requests = append(requests, domainReq)
	}

	if err := rows.Err(); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return []authrequest.AuthRequest{}, nil
		}
		return nil, serrors.E(op, err)
	}

	return requests, nil
}
