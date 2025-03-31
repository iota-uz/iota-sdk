package persistence

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"

	"github.com/go-faster/errors"
)

var (
	ErrSessionNotFound = errors.New("session not found")
)

const (
	sessionFindQuery   = `SELECT token, user_id, expires_at, ip, user_agent, created_at, tenant_id FROM sessions`
	sessionCountQuery  = `SELECT COUNT(*) as count FROM sessions`
	sessionInsertQuery = `
        INSERT INTO sessions (
            token,
            user_id,
            expires_at,
            ip,
            user_agent,
            created_at,
            tenant_id
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7)`
	sessionUpdateQuery = `
        UPDATE sessions
        SET expires_at = $1,
            ip = $2,
            user_agent = $3
        WHERE token = $4 AND tenant_id = $5`
	sessionDeleteQuery = `DELETE FROM sessions WHERE token = $1 AND tenant_id = $2`
)

type GormSessionRepository struct{}

func NewSessionRepository() session.Repository {
	return &GormSessionRepository{}
}

func (g *GormSessionRepository) GetPaginated(ctx context.Context, params *session.FindParams) ([]*session.Session, error) {
	sortFields := []string{}
	for _, f := range params.SortBy.Fields {
		switch f {
		case session.ExpiresAt:
			sortFields = append(sortFields, "sessions.expires_at")
		case session.CreatedAt:
			sortFields = append(sortFields, "sessions.created_at")
		default:
			return nil, fmt.Errorf("unknown sort field: %v", f)
		}
	}

	var args []interface{}
	where := []string{"1 = 1"}

	if params.Token != "" {
		where = append(where, fmt.Sprintf("token = $%d", len(args)+1))
		args = append(args, params.Token)
	}

	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return nil, err
	}
	where = append(where, fmt.Sprintf("tenant_id = $%d", len(args)+1))
	args = append(args, tenant.ID)

	return g.querySessions(
		ctx,
		repo.Join(
			sessionFindQuery,
			repo.JoinWhere(where...),
			repo.OrderBy(sortFields, params.SortBy.Ascending),
			repo.FormatLimitOffset(params.Limit, params.Offset),
		),
		args...,
	)
}

func (g *GormSessionRepository) Count(ctx context.Context) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}

	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return 0, err
	}

	var count int64
	if err := tx.QueryRow(ctx, sessionCountQuery+" WHERE tenant_id = $1", tenant.ID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *GormSessionRepository) GetAll(ctx context.Context) ([]*session.Session, error) {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return nil, err
	}

	return g.querySessions(ctx, sessionFindQuery+" WHERE tenant_id = $1", tenant.ID)
}

func (g *GormSessionRepository) GetByToken(ctx context.Context, token string) (*session.Session, error) {
	// First try with tenant from context
	tenant, err := composables.UseTenant(ctx)

	// If tenant is not in context (like during login), get the session regardless of tenant
	if err != nil {
		// Ensure we have a transaction context
		_, err := composables.UseTx(ctx)
		if err != nil {
			return nil, err
		}

		// Query without tenant filter during login
		sessions, err := g.querySessions(ctx, repo.Join(sessionFindQuery, "WHERE token = $1"), token)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get session by token")
		}
		if len(sessions) == 0 {
			return nil, ErrSessionNotFound
		}
		return sessions[0], nil
	}

	// Normal flow with tenant from context
	sessions, err := g.querySessions(ctx, repo.Join(sessionFindQuery, "WHERE token = $1 AND tenant_id = $2"), token, tenant.ID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get session by token")
	}
	if len(sessions) == 0 {
		return nil, ErrSessionNotFound
	}
	return sessions[0], nil
}

func (g *GormSessionRepository) Create(ctx context.Context, data *session.Session) error {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return err
	}

	dbSession := ToDBSession(data)
	dbSession.TenantID = tenant.ID

	return g.execQuery(
		ctx,
		sessionInsertQuery,
		dbSession.Token,
		dbSession.UserID,
		dbSession.ExpiresAt,
		dbSession.IP,
		dbSession.UserAgent,
		dbSession.CreatedAt,
		dbSession.TenantID,
	)
}

func (g *GormSessionRepository) Update(ctx context.Context, data *session.Session) error {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return err
	}

	dbSession := ToDBSession(data)
	dbSession.TenantID = tenant.ID

	return g.execQuery(
		ctx,
		sessionUpdateQuery,
		dbSession.ExpiresAt,
		dbSession.IP,
		dbSession.UserAgent,
		dbSession.Token,
		dbSession.TenantID,
	)
}

func (g *GormSessionRepository) Delete(ctx context.Context, token string) error {
	tenant, err := composables.UseTenant(ctx)
	if err != nil {
		return err
	}
	return g.execQuery(ctx, sessionDeleteQuery, token, tenant.ID)
}

func (g *GormSessionRepository) querySessions(ctx context.Context, query string, args ...interface{}) ([]*session.Session, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*session.Session
	for rows.Next() {
		var sessionRow models.Session
		if err := rows.Scan(
			&sessionRow.Token,
			&sessionRow.UserID,
			&sessionRow.ExpiresAt,
			&sessionRow.IP,
			&sessionRow.UserAgent,
			&sessionRow.CreatedAt,
			&sessionRow.TenantID,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, ToDomainSession(&sessionRow))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return sessions, nil
}

func (g *GormSessionRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
