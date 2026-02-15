package persistence

import (
	"context"
	"fmt"

	"github.com/google/uuid"
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
	selectSessionQuery = `SELECT token, user_id, expires_at, ip, user_agent, created_at, tenant_id, audience, status FROM sessions`
	countSessionQuery  = `SELECT COUNT(*) as count FROM sessions`
	insertSessionQuery = `
        INSERT INTO sessions (
            token,
            user_id,
            expires_at,
            ip,
            user_agent,
            created_at,
            tenant_id,
            audience,
            status
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	updateSessionQuery = `
        UPDATE sessions
        SET expires_at = $1,
            ip = $2,
            user_agent = $3
        WHERE token = $4 AND tenant_id = $5`
	updateSessionStatusQuery  = `UPDATE sessions SET status = $1 WHERE user_id = $2 AND tenant_id = $3`
	deleteUserSessionQuery    = `DELETE FROM sessions WHERE user_id = $1 AND tenant_id = $2`
	deleteSessionQuery        = `DELETE FROM sessions WHERE token = $1 AND tenant_id = $2`
	deleteAllExceptTokenQuery = `DELETE FROM sessions WHERE user_id = $1 AND token != $2 AND tenant_id = $3`
)

type SessionRepository struct {
	fieldMap        map[session.Field]string
	aliasedFieldMap map[session.Field]string // for queries that use "s" as sessions alias
}

func NewSessionRepository() session.Repository {
	return &SessionRepository{
		fieldMap: map[session.Field]string{
			session.ExpiresAt: "sessions.expires_at",
			session.CreatedAt: "sessions.created_at",
		},
		aliasedFieldMap: map[session.Field]string{
			session.ExpiresAt: "s.expires_at",
			session.CreatedAt: "s.created_at",
		},
	}
}

func (g *SessionRepository) GetPaginated(ctx context.Context, params *session.FindParams) ([]session.Session, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	args := []interface{}{tenantID}
	where := []string{fmt.Sprintf("s.tenant_id = $%d", len(args))}

	if params.Token != "" {
		args = append(args, params.Token)
		where = append(where, fmt.Sprintf("s.token = $%d", len(args)))
	}

	// Add search filter with JOIN if search query provided
	var joinClause string
	if params.Search != "" {
		joinClause = "JOIN users u ON s.user_id = u.id"
		searchPattern := "%" + params.Search + "%"
		args = append(args, searchPattern)
		pos := len(args)
		where = append(where, fmt.Sprintf("(u.first_name ILIKE $%d OR u.last_name ILIKE $%d OR u.email ILIKE $%d)", pos, pos, pos))
	}

	// Use table alias for select query (must match querySessions scan: 9 columns including status)
	selectQuery := "SELECT s.token, s.user_id, s.expires_at, s.ip, s.user_agent, s.created_at, s.tenant_id, s.audience, s.status FROM sessions s"

	query := repo.Join(
		selectQuery,
		joinClause,
		repo.JoinWhere(where...),
		params.SortBy.ToSQL(g.aliasedFieldMap),
		repo.FormatLimitOffset(params.Limit, params.Offset),
	)

	return g.querySessions(ctx, query, args...)
}

func (g *SessionRepository) Count(ctx context.Context) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, err
	}

	var count int64
	if err := tx.QueryRow(ctx, countSessionQuery+" WHERE tenant_id = $1", tenantID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (g *SessionRepository) CountFiltered(ctx context.Context, search string) (int64, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, err
	}

	var count int64
	if search != "" {
		query := `
			SELECT COUNT(*) as count 
			FROM sessions s
			JOIN users u ON s.user_id = u.id
			WHERE s.tenant_id = $1 
			AND (u.first_name ILIKE $2 OR u.last_name ILIKE $2 OR u.email ILIKE $2)
		`
		if err := tx.QueryRow(ctx, query, tenantID, "%"+search+"%").Scan(&count); err != nil {
			return 0, err
		}
	} else {
		// Fall back to regular count if no search
		return g.Count(ctx)
	}
	return count, nil
}

func (g *SessionRepository) GetAll(ctx context.Context) ([]session.Session, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	return g.querySessions(ctx, selectSessionQuery+" WHERE tenant_id = $1", tenantID)
}

func (g *SessionRepository) GetByToken(ctx context.Context, token string) (session.Session, error) {
	// First try with tenant from context
	tenantID, err := composables.UseTenantID(ctx)

	// If tenant is not in context (like during login), get the session regardless of tenant
	if err != nil {
		// Ensure we have a transaction context
		_, err := composables.UseTx(ctx)
		if err != nil {
			return nil, err
		}

		// Query without tenant filter during login
		sessions, err := g.querySessions(ctx, repo.Join(selectSessionQuery, "WHERE token = $1"), token)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get session by token")
		}
		if len(sessions) == 0 {
			return nil, ErrSessionNotFound
		}
		return sessions[0], nil
	}

	// Normal flow with tenant from context
	sessions, err := g.querySessions(ctx, repo.Join(selectSessionQuery, "WHERE token = $1 AND tenant_id = $2"), token, tenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get session by token")
	}
	if len(sessions) == 0 {
		return nil, ErrSessionNotFound
	}
	return sessions[0], nil
}

func (g *SessionRepository) GetByTokenAndAudience(ctx context.Context, token string, audience session.SessionAudience) (session.Session, error) {
	// First try with tenant from context
	tenantID, err := composables.UseTenantID(ctx)

	// If tenant is not in context (like during login), get the session regardless of tenant
	if err != nil {
		// Ensure we have a transaction context
		_, err := composables.UseTx(ctx)
		if err != nil {
			return nil, err
		}

		// Query without tenant filter during login
		sessions, err := g.querySessions(ctx, repo.Join(selectSessionQuery, "WHERE token = $1 AND audience = $2"), token, audience)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get session by token and audience")
		}
		if len(sessions) == 0 {
			return nil, ErrSessionNotFound
		}
		return sessions[0], nil
	}

	// Normal flow with tenant from context
	sessions, err := g.querySessions(ctx, repo.Join(selectSessionQuery, "WHERE token = $1 AND tenant_id = $2 AND audience = $3"), token, tenantID, audience)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get session by token and audience")
	}
	if len(sessions) == 0 {
		return nil, ErrSessionNotFound
	}
	return sessions[0], nil
}

func (g *SessionRepository) Create(ctx context.Context, data session.Session) error {
	dbSession := ToDBSession(data)

	// First try to get tenant from context
	tenantID, err := composables.UseTenantID(ctx)
	if err == nil {
		dbSession.TenantID = tenantID.String()
	}
	// If tenant is not in context but session has TenantID set (from session.CreateDTO), use that
	return g.execQuery(
		ctx,
		insertSessionQuery,
		dbSession.Token,
		dbSession.UserID,
		dbSession.ExpiresAt,
		dbSession.IP,
		dbSession.UserAgent,
		dbSession.CreatedAt,
		dbSession.TenantID,
		dbSession.Audience,
		dbSession.Status,
	)
}

func (g *SessionRepository) Update(ctx context.Context, data session.Session) error {
	dbSession := ToDBSession(data)

	// First try to get tenant from context
	tenantID, err := composables.UseTenantID(ctx)
	if err == nil {
		dbSession.TenantID = tenantID.String()
	} else if dbSession.TenantID == uuid.Nil.String() {
		// If tenant is not in context and session has no TenantID, get the current session's tenant ID
		existingSession, err := g.GetByToken(ctx, dbSession.Token)
		if err != nil {
			return err
		}
		dbSession.TenantID = existingSession.TenantID().String()
	}
	return g.execQuery(
		ctx,
		updateSessionQuery,
		dbSession.ExpiresAt,
		dbSession.IP,
		dbSession.UserAgent,
		dbSession.Token,
		dbSession.TenantID,
	)
}

func (g *SessionRepository) Delete(ctx context.Context, token string) error {
	sess, err := g.GetByToken(ctx, token)
	if err != nil {
		return err
	}
	return g.execQuery(ctx, deleteSessionQuery, token, sess.TenantID())
}

func (g *SessionRepository) UpdateStatus(ctx context.Context, id uint, status session.SessionStatus) error {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get tenant from context")
	}

	return g.execQuery(ctx, updateSessionStatusQuery, string(status), id, tenantID.String())
}

func (g *SessionRepository) DeleteByUserId(ctx context.Context, userId uint) ([]session.Session, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}
	sql := repo.Join(
		selectSessionQuery,
		repo.JoinWhere("sessions.user_id = $1", "sessions.tenant_id = $2"),
	)
	sessions, err := g.querySessions(
		ctx,
		sql,
		userId,
		tenantID,
	)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	err = g.execQuery(ctx, deleteUserSessionQuery, userId, tenantID)
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (g *SessionRepository) GetByUserID(ctx context.Context, userID uint) ([]session.Session, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	query := repo.Join(
		selectSessionQuery,
		repo.JoinWhere("user_id = $1", "tenant_id = $2"),
	)

	return g.querySessions(ctx, query, userID, tenantID)
}

func (g *SessionRepository) DeleteAllExceptToken(ctx context.Context, userID uint, exceptToken string) (int, error) {
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}

	result, err := tx.Exec(ctx, deleteAllExceptTokenQuery, userID, exceptToken, tenantID)
	if err != nil {
		return 0, err
	}

	return int(result.RowsAffected()), nil
}

func (g *SessionRepository) querySessions(ctx context.Context, query string, args ...interface{}) ([]session.Session, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []session.Session
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
			&sessionRow.Audience,
			&sessionRow.Status,
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

func (g *SessionRepository) execQuery(ctx context.Context, query string, args ...interface{}) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, query, args...)
	return err
}
