// Package persistence keeps the automations module's own rows: pending OAuth2 states and connection
// check times (connections themselves are pkg/automation/connectors').
package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/automations/services"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type oauthStates struct{ db *pgxpool.Pool }

// NewOAuthStates keeps pending OAuth2 states in automation.oauth_states.
func NewOAuthStates(db *pgxpool.Pool) services.OAuthStates { return &oauthStates{db: db} }

func (r *oauthStates) Put(ctx context.Context, s services.OAuthState) error {
	const op = serrors.Op("automations.OAuthStates.Put")
	// Expired states of every tenant are swept on the way (an abandoned sign-in never comes back); they
	// carry no data of a tenant beyond the hash of a dead state.
	if _, err := r.db.Exec(ctx, `DELETE FROM automation.oauth_states WHERE expires_at < now()`); err != nil {
		return serrors.Wrap(op, err)
	}
	var conn *uuid.UUID
	if s.ConnectionID != uuid.Nil {
		conn = &s.ConnectionID
	}
	_, err := r.db.Exec(ctx, `INSERT INTO automation.oauth_states
		(state_hash, tenant_id, user_id, kind, name, display_name, connection_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		s.Hash, s.TenantID, s.UserID, string(s.Kind), s.Name, s.DisplayName, conn, s.ExpiresAt)
	return serrors.Wrap(op, err)
}

func (r *oauthStates) Take(ctx context.Context, tenantID uuid.UUID, userID int64, hash []byte, now time.Time) (services.OAuthState, error) {
	const op = serrors.Op("automations.OAuthStates.Take")
	var (
		s    services.OAuthState
		kind string
		conn *uuid.UUID
	)
	err := r.db.QueryRow(ctx, `DELETE FROM automation.oauth_states WHERE state_hash = $1 AND tenant_id = $2 AND user_id = $3
		RETURNING state_hash, tenant_id, user_id, kind, name, display_name, connection_id, expires_at`,
		hash, tenantID, userID).Scan(&s.Hash, &s.TenantID, &s.UserID, &kind, &s.Name, &s.DisplayName, &conn, &s.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return services.OAuthState{}, services.ErrStateInvalid
	}
	if err != nil {
		return services.OAuthState{}, serrors.Wrap(op, err)
	}
	if !now.Before(s.ExpiresAt) {
		return services.OAuthState{}, services.ErrStateInvalid
	}
	s.Kind = catalog.ConnectorKind(kind)
	if conn != nil {
		s.ConnectionID = *conn
	}
	return s, nil
}

type connectionChecks struct{ db *pgxpool.Pool }

// NewConnectionChecks keeps check times in automation.connection_checks.
func NewConnectionChecks(db *pgxpool.Pool) services.ConnectionChecks {
	return &connectionChecks{db: db}
}

func (r *connectionChecks) Touch(ctx context.Context, tenantID, connectionID uuid.UUID, at time.Time) error {
	// The row exists only while the connection of the tenant does (the foreign key and the check of
	// the tenant below).
	_, err := r.db.Exec(ctx, `INSERT INTO automation.connection_checks (connection_id, tenant_id, checked_at)
		SELECT id, tenant_id, $3 FROM automation.connections WHERE tenant_id = $1 AND id = $2
		ON CONFLICT (connection_id) DO UPDATE SET checked_at = EXCLUDED.checked_at`, tenantID, connectionID, at)
	return serrors.Wrap("automations.ConnectionChecks.Touch", err)
}

func (r *connectionChecks) LastChecked(ctx context.Context, tenantID uuid.UUID) (map[uuid.UUID]time.Time, error) {
	const op = serrors.Op("automations.ConnectionChecks.LastChecked")
	rows, err := r.db.Query(ctx, `SELECT connection_id, checked_at FROM automation.connection_checks WHERE tenant_id = $1`, tenantID)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	defer rows.Close()
	out := map[uuid.UUID]time.Time{}
	for rows.Next() {
		var (
			id uuid.UUID
			at time.Time
		)
		if err := rows.Scan(&id, &at); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		out[id] = at
	}
	return out, serrors.Wrap(op, rows.Err())
}

type requests struct{ db *pgxpool.Pool }

// NewRequests keeps idempotency keys in automation.connection_requests.
func NewRequests(db *pgxpool.Pool) services.Requests { return &requests{db: db} }

// staleReservation is how long a reservation without a connection holds its key: past it, the
// request that made it is taken to have failed (it rolled back or its process died).
const staleReservation = "2 minutes"

func (r *requests) Reserve(ctx context.Context, tenantID uuid.UUID, key, fingerprint string) (services.Reservation, error) {
	const op = serrors.Op("automations.Requests.Reserve")
	if _, err := r.db.Exec(ctx, `DELETE FROM automation.connection_requests WHERE created_at < now() - interval '1 day'`); err != nil {
		return services.Reservation{}, serrors.Wrap(op, err)
	}
	owner := uuid.New()
	tag, err := r.db.Exec(ctx, `INSERT INTO automation.connection_requests (tenant_id, key, fingerprint, owner)
		VALUES ($1, $2, $3, $4) ON CONFLICT (tenant_id, key) DO NOTHING`, tenantID, key, fingerprint, owner)
	if err != nil {
		return services.Reservation{}, serrors.Wrap(op, err)
	}
	if tag.RowsAffected() == 1 {
		return services.Reservation{State: services.RequestNew, Owner: owner}, nil
	}
	var (
		fp   string
		conn *uuid.UUID
	)
	err = r.db.QueryRow(ctx, `SELECT fingerprint, connection_id FROM automation.connection_requests WHERE tenant_id = $1 AND key = $2`,
		tenantID, key).Scan(&fp, &conn)
	if errors.Is(err, pgx.ErrNoRows) {
		// Released or swept between the two statements: reserve again.
		return r.Reserve(ctx, tenantID, key, fingerprint)
	}
	if err != nil {
		return services.Reservation{}, serrors.Wrap(op, err)
	}
	switch {
	case fp != fingerprint:
		return services.Reservation{State: services.RequestMismatch}, nil
	case conn != nil:
		return services.Reservation{State: services.RequestDone, ConnectionID: *conn}, nil
	}
	// A reservation of a request that died is taken over; a live one is in progress.
	// The takeover gives the reservation a new owner: the request it replaces is fenced out.
	tag, err = r.db.Exec(ctx, `UPDATE automation.connection_requests SET created_at = now(), owner = $3
		WHERE tenant_id = $1 AND key = $2 AND connection_id IS NULL AND created_at < now() - interval '`+staleReservation+`'`, tenantID, key, owner)
	if err != nil {
		return services.Reservation{}, serrors.Wrap(op, err)
	}
	if tag.RowsAffected() == 1 {
		return services.Reservation{State: services.RequestNew, Owner: owner}, nil
	}
	return services.Reservation{State: services.RequestInProgress}, nil
}

func (r *requests) Complete(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, key string, owner, connectionID uuid.UUID) error {
	tag, err := tx.Exec(ctx, `UPDATE automation.connection_requests SET connection_id = $4
		WHERE tenant_id = $1 AND key = $2 AND owner = $3 AND connection_id IS NULL`, tenantID, key, owner, connectionID)
	if err != nil {
		return serrors.Wrap("automations.Requests.Complete", err)
	}
	if tag.RowsAffected() != 1 {
		// The reservation expired and another request took the key over (or it was released): this
		// write must not commit.
		return serrors.Wrap("automations.Requests.Complete", services.ErrRequestInProgress)
	}
	return nil
}

func (r *requests) Release(ctx context.Context, tenantID uuid.UUID, key string, owner uuid.UUID) error {
	_, err := r.db.Exec(ctx, `DELETE FROM automation.connection_requests
		WHERE tenant_id = $1 AND key = $2 AND owner = $3 AND connection_id IS NULL`, tenantID, key, owner)
	return serrors.Wrap("automations.Requests.Release", err)
}
