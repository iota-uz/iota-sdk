// Package persistence provides this package.
package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SQL query constants (tables live in bichat schema)
const (
	// Session queries
	insertSessionQuery = `
			INSERT INTO bichat.sessions (
				id, tenant_id, user_id, title, status, pinned,
				parent_session_id, llm_previous_response_id, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`
	selectSessionQuery = `
			SELECT id, tenant_id, user_id, title, status, pinned,
				   parent_session_id, llm_previous_response_id, created_at, updated_at
			FROM bichat.sessions
			WHERE tenant_id = $1 AND id = $2
		`
	updateSessionQuery = `
				UPDATE bichat.sessions
				SET title = $1, status = $2, pinned = $3,
					parent_session_id = $4, llm_previous_response_id = $5, updated_at = $6
				WHERE tenant_id = $7 AND id = $8
			`
	updateSessionTitleQuery = `
		UPDATE bichat.sessions
		SET title = $1, updated_at = $2
		WHERE tenant_id = $3 AND id = $4
	`
	updateSessionTitleIfEmptyQuery = `
		UPDATE bichat.sessions
		SET title = $1, updated_at = $2
		WHERE tenant_id = $3 AND id = $4 AND (
			btrim(title) = '' OR title = 'Untitled Session' OR title = 'Untitled Chat'
		)
	`
	listUserSessionsQuery = `
			SELECT id, tenant_id, user_id, title, status, pinned,
				   parent_session_id, llm_previous_response_id, created_at, updated_at
			FROM bichat.sessions
			WHERE tenant_id = $1 AND user_id = $2 AND ($5::boolean OR status != 'ARCHIVED')
			ORDER BY pinned DESC, created_at DESC
		LIMIT $3 OFFSET $4
	`
	countUserSessionsQuery = `
			SELECT COUNT(*)
			FROM bichat.sessions
			WHERE tenant_id = $1 AND user_id = $2 AND ($3::boolean OR status != 'ARCHIVED')
		`
	deleteSessionQuery = `
		DELETE FROM bichat.sessions
		WHERE tenant_id = $1 AND id = $2
	`
	listAccessibleSessionSummariesQuery = `
		SELECT
			s.id, s.tenant_id, s.user_id, s.title, s.status, s.pinned,
			s.parent_session_id, s.llm_previous_response_id, s.created_at, s.updated_at,
			COALESCE(owner_u.first_name, ''), COALESCE(owner_u.last_name, ''),
			CASE
				WHEN s.user_id = $2 THEN 'OWNER'
				WHEN sm_self.role = 'EDITOR' THEN 'EDITOR'
				WHEN sm_self.role = 'VIEWER' THEN 'VIEWER'
				ELSE 'NONE'
			END AS access_role,
			CASE
				WHEN s.user_id = $2 THEN 'owner'
				WHEN sm_self.user_id IS NOT NULL THEN 'member'
				ELSE 'none'
			END AS access_source,
			(1 + COALESCE(member_stats.member_count, 0))::int AS participant_count
		FROM bichat.sessions s
		LEFT JOIN bichat.session_members sm_self
			ON sm_self.tenant_id = s.tenant_id
			AND sm_self.session_id = s.id
			AND sm_self.user_id = $2
		LEFT JOIN public.users owner_u ON owner_u.id = s.user_id AND owner_u.tenant_id = s.tenant_id
		LEFT JOIN (
			SELECT session_id, COUNT(*) AS member_count
			FROM bichat.session_members
			WHERE tenant_id = $1
			GROUP BY session_id
		) member_stats ON member_stats.session_id = s.id
		WHERE s.tenant_id = $1
		  AND ($5::boolean OR s.status != 'ARCHIVED')
		  AND (s.user_id = $2 OR sm_self.user_id IS NOT NULL)
		ORDER BY s.pinned DESC, s.created_at DESC
		LIMIT $3 OFFSET $4
	`
	countAccessibleSessionsQuery = `
		SELECT COUNT(*)
		FROM bichat.sessions s
		LEFT JOIN bichat.session_members sm_self
			ON sm_self.tenant_id = s.tenant_id
			AND sm_self.session_id = s.id
			AND sm_self.user_id = $2
		WHERE s.tenant_id = $1
		  AND ($3::boolean OR s.status != 'ARCHIVED')
		  AND (s.user_id = $2 OR sm_self.user_id IS NOT NULL)
	`
	listAllSessionSummariesQuery = `
		SELECT
			s.id, s.tenant_id, s.user_id, s.title, s.status, s.pinned,
			s.parent_session_id, s.llm_previous_response_id, s.created_at, s.updated_at,
			COALESCE(owner_u.first_name, ''), COALESCE(owner_u.last_name, ''),
			CASE
				WHEN s.user_id = $2 THEN 'OWNER'
				WHEN sm_self.role = 'EDITOR' THEN 'EDITOR'
				WHEN sm_self.role = 'VIEWER' THEN 'VIEWER'
				ELSE 'READ_ALL'
			END AS access_role,
			CASE
				WHEN s.user_id = $2 THEN 'owner'
				WHEN sm_self.user_id IS NOT NULL THEN 'member'
				ELSE 'permission'
			END AS access_source,
			(1 + COALESCE(member_stats.member_count, 0))::int AS participant_count
		FROM bichat.sessions s
		LEFT JOIN bichat.session_members sm_self
			ON sm_self.tenant_id = s.tenant_id
			AND sm_self.session_id = s.id
			AND sm_self.user_id = $2
		LEFT JOIN public.users owner_u ON owner_u.id = s.user_id AND owner_u.tenant_id = s.tenant_id
		LEFT JOIN (
			SELECT session_id, COUNT(*) AS member_count
			FROM bichat.session_members
			WHERE tenant_id = $1
			GROUP BY session_id
		) member_stats ON member_stats.session_id = s.id
		WHERE s.tenant_id = $1
		  AND ($5::boolean OR s.status != 'ARCHIVED')
		  AND ($6::bigint IS NULL OR s.user_id = $6)
		ORDER BY s.pinned DESC, s.created_at DESC
		LIMIT $3 OFFSET $4
	`
	countAllSessionsQuery = `
		SELECT COUNT(*)
		FROM bichat.sessions s
		WHERE s.tenant_id = $1
		  AND ($2::boolean OR s.status != 'ARCHIVED')
		  AND ($3::bigint IS NULL OR s.user_id = $3)
	`
	resolveSessionAccessQuery = `
		SELECT
			s.user_id,
			COALESCE(sm_self.role, '')
		FROM bichat.sessions s
		LEFT JOIN bichat.session_members sm_self
			ON sm_self.tenant_id = s.tenant_id
			AND sm_self.session_id = s.id
			AND sm_self.user_id = $3
		WHERE s.tenant_id = $1
		  AND s.id = $2
	`
	listSessionMembersQuery = `
		SELECT
			sm.session_id,
			sm.user_id,
			sm.role,
			sm.created_at,
			sm.updated_at,
			COALESCE(u.first_name, ''),
			COALESCE(u.last_name, '')
		FROM bichat.session_members sm
		JOIN public.users u ON u.id = sm.user_id AND u.tenant_id = sm.tenant_id
		WHERE sm.tenant_id = $1
		  AND sm.session_id = $2
		ORDER BY sm.created_at ASC, sm.user_id ASC
	`
	upsertSessionMemberQuery = `
		INSERT INTO bichat.session_members (
			tenant_id, session_id, user_id, role, created_at, updated_at
		)
		SELECT
			$1, $2, $3, $4, NOW(), NOW()
		WHERE EXISTS (
			SELECT 1 FROM bichat.sessions s
			WHERE s.tenant_id = $1 AND s.id = $2 AND s.user_id <> $3
		)
		  AND EXISTS (
			SELECT 1 FROM public.users u
			WHERE u.id = $3 AND u.tenant_id = $1
		)
		ON CONFLICT (tenant_id, session_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, updated_at = NOW()
	`
	removeSessionMemberQuery = `
		DELETE FROM bichat.session_members
		WHERE tenant_id = $1
		  AND session_id = $2
		  AND user_id = $3
	`
	countSessionParticipantsQuery = `
		SELECT (1 + COALESCE((
			SELECT COUNT(*)
			FROM bichat.session_members sm
			WHERE sm.tenant_id = $1 AND sm.session_id = $2
		), 0))::int
		FROM bichat.sessions s
		WHERE s.tenant_id = $1
		  AND s.id = $2
	`
	listTenantUsersQuery = `
		SELECT id, COALESCE(first_name, ''), COALESCE(last_name, '')
		FROM public.users
		WHERE tenant_id = $1
		ORDER BY first_name ASC, last_name ASC, id ASC
	`
	listTenantUsersByGroupQuery = `
		SELECT u.id, COALESCE(u.first_name, ''), COALESCE(u.last_name, '')
		FROM public.users u
		INNER JOIN group_users gu ON gu.user_id = u.id
		INNER JOIN user_groups ug ON ug.id = gu.group_id AND ug.tenant_id = $1
		WHERE u.tenant_id = $1 AND gu.group_id = $2
		ORDER BY u.first_name ASC, u.last_name ASC, u.id ASC
	`
	getTenantUserQuery = `
		SELECT id, COALESCE(first_name, ''), COALESCE(last_name, '')
		FROM public.users
		WHERE tenant_id = $1 AND id = $2
	`
	sessionUserExistsQuery = `
		SELECT EXISTS (
			SELECT 1 FROM public.users
			WHERE tenant_id = $1 AND id = $2
		)
	`

	// Message queries
	truncateMessagesFromQuery = `
		DELETE FROM bichat.messages m
		USING bichat.sessions s
		WHERE m.session_id = s.id
		  AND s.tenant_id = $1
		  AND m.session_id = $2
		  AND m.created_at >= $3
	`
	selectSessionOwnerUserIDQuery = `
		SELECT user_id
		FROM bichat.sessions
		WHERE tenant_id = $1 AND id = $2
	`

	// Attachment compatibility queries (backed by bichat.artifacts type='attachment')
	selectMessageSessionQuery = `
		SELECT m.session_id
		FROM bichat.messages m
		JOIN bichat.sessions s ON m.session_id = s.id
		WHERE s.tenant_id = $1 AND m.id = $2
	`
	selectAttachmentQuery = `
		SELECT a.id, a.message_id, a.upload_id, a.name, a.mime_type, a.size_bytes, a.url, a.created_at
		FROM bichat.artifacts a
		WHERE a.tenant_id = $1
		  AND a.id = $2
		  AND a.type = 'attachment'
		  AND a.message_id IS NOT NULL
	`
	selectMessageAttachmentsQuery = `
		SELECT a.id, a.message_id, a.upload_id, a.name, a.mime_type, a.size_bytes, a.url, a.created_at
		FROM bichat.artifacts a
		WHERE a.tenant_id = $1
		  AND a.message_id = $2
		  AND a.type = 'attachment'
		ORDER BY a.created_at ASC
	`

	selectMessageCodeOutputArtifactsQuery = `
			SELECT a.id, a.message_id, a.name, COALESCE(a.mime_type, ''), COALESCE(a.url, ''), COALESCE(a.size_bytes, 0), a.created_at
			FROM bichat.artifacts a
			WHERE a.tenant_id = $1
			  AND a.message_id = $2
			  AND a.type = 'code_output'
			ORDER BY a.created_at ASC
		`

	// Generation run queries (refresh-safe streaming)
	insertGenerationRunQuery = `
		INSERT INTO bichat.generation_runs (
			id, session_id, tenant_id, user_id, status, partial_content, partial_metadata, started_at, last_updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	selectActiveGenerationRunBySessionQuery = `
		SELECT id, session_id, tenant_id, user_id, status, partial_content, partial_metadata, started_at, last_updated_at
		FROM bichat.generation_runs
		WHERE tenant_id = $1 AND session_id = $2 AND status = 'streaming'
		LIMIT 1
	`
	selectGenerationRunByIDQuery = `
		SELECT id, session_id, tenant_id, user_id, status, partial_content, partial_metadata, started_at, last_updated_at
		FROM bichat.generation_runs
		WHERE tenant_id = $1 AND id = $2
		LIMIT 1
	`
	updateGenerationRunSnapshotQuery = `
		UPDATE bichat.generation_runs
		SET partial_content = $1, partial_metadata = $2, last_updated_at = $3
		WHERE tenant_id = $4 AND id = $5 AND status = 'streaming'
	`
	completeGenerationRunQuery = `
		UPDATE bichat.generation_runs
		SET status = 'completed', last_updated_at = $1
		WHERE tenant_id = $2 AND id = $3
	`
	cancelGenerationRunQuery = `
		UPDATE bichat.generation_runs
		SET status = 'cancelled', last_updated_at = $1
		WHERE tenant_id = $2 AND id = $3
	`
)

var (
	ErrSessionNotFound    = errors.New("session not found")
	ErrMessageNotFound    = errors.New("message not found")
	ErrAttachmentNotFound = errors.New("attachment not found")
	ErrTenantUserNotFound = errors.New("tenant user not found")
)

// ChatRepoOption configures PostgresChatRepository.
type ChatRepoOption func(*PostgresChatRepository)

// WithUserGroupFilter restricts ListTenantUsers to members of the given group.
func WithUserGroupFilter(groupID uuid.UUID) ChatRepoOption {
	return func(r *PostgresChatRepository) {
		r.userGroupID = groupID
	}
}

// PostgresChatRepository implements domain.ChatRepository using PostgreSQL.
type PostgresChatRepository struct {
	userGroupID uuid.UUID
}

// NewPostgresChatRepository creates a new PostgreSQL chat repository.
func NewPostgresChatRepository(opts ...ChatRepoOption) domain.ChatRepository {
	r := &PostgresChatRepository{}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Session operations

// CreateSession creates a new session in the database.
func (r *PostgresChatRepository) CreateSession(ctx context.Context, session domain.Session) error {
	const op serrors.Op = "PostgresChatRepository.CreateSession"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	model, err := models.SessionModelFromDomain(session)
	if err != nil {
		return serrors.E(op, err)
	}
	model.TenantID = tenantID
	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now()
	}
	if model.UpdatedAt.IsZero() {
		model.UpdatedAt = model.CreatedAt
	}

	_, err = tx.Exec(ctx, insertSessionQuery,
		model.ID,
		model.TenantID,
		model.UserID,
		model.Title,
		model.Status,
		model.Pinned,
		model.ParentSessionID,
		model.LLMPreviousResponseID,
		model.CreatedAt,
		model.UpdatedAt,
	)
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// GetSession retrieves a session by ID.
func (r *PostgresChatRepository) GetSession(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	const op serrors.Op = "PostgresChatRepository.GetSession"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	var model models.SessionModel
	err = tx.QueryRow(ctx, selectSessionQuery, tenantID, id).Scan(
		&model.ID,
		&model.TenantID,
		&model.UserID,
		&model.Title,
		&model.Status,
		&model.Pinned,
		&model.ParentSessionID,
		&model.LLMPreviousResponseID,
		&model.CreatedAt,
		&model.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serrors.E(op, ErrSessionNotFound)
		}
		return nil, serrors.E(op, err)
	}

	sessionEntity, err := model.ToDomain()
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return sessionEntity, nil
}

// UpdateSession updates an existing session.
func (r *PostgresChatRepository) UpdateSession(ctx context.Context, session domain.Session) error {
	const op serrors.Op = "PostgresChatRepository.UpdateSession"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	model, err := models.SessionModelFromDomain(session)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, updateSessionQuery,
		model.Title,
		model.Status,
		model.Pinned,
		model.ParentSessionID,
		model.LLMPreviousResponseID,
		model.UpdatedAt,
		tenantID,
		model.ID,
	)
	if err != nil {
		return serrors.E(op, err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return serrors.E(op, ErrSessionNotFound)
	}

	return nil
}

// UpdateSessionTitle updates a session title.
func (r *PostgresChatRepository) UpdateSessionTitle(ctx context.Context, id uuid.UUID, title string) error {
	const op serrors.Op = "PostgresChatRepository.UpdateSessionTitle"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, updateSessionTitleQuery, title, time.Now(), tenantID, id)
	if err != nil {
		return serrors.E(op, err)
	}
	if result.RowsAffected() == 0 {
		return serrors.E(op, ErrSessionNotFound)
	}

	return nil
}

// UpdateSessionTitleIfEmpty updates a session title only when it is currently
// blank or still using an untitled placeholder title.
func (r *PostgresChatRepository) UpdateSessionTitleIfEmpty(ctx context.Context, id uuid.UUID, title string) (bool, error) {
	const op serrors.Op = "PostgresChatRepository.UpdateSessionTitleIfEmpty"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return false, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return false, serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, updateSessionTitleIfEmptyQuery, title, time.Now(), tenantID, id)
	if err != nil {
		return false, serrors.E(op, err)
	}

	return result.RowsAffected() > 0, nil
}

// ListUserSessions lists all sessions for a user with pagination.
func (r *PostgresChatRepository) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error) {
	const op serrors.Op = "PostgresChatRepository.ListUserSessions"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, listUserSessionsQuery, tenantID, userID, opts.Limit, opts.Offset, opts.IncludeArchived)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	var sessions []domain.Session
	for rows.Next() {
		var model models.SessionModel
		err := rows.Scan(
			&model.ID,
			&model.TenantID,
			&model.UserID,
			&model.Title,
			&model.Status,
			&model.Pinned,
			&model.ParentSessionID,
			&model.LLMPreviousResponseID,
			&model.CreatedAt,
			&model.UpdatedAt,
		)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		sessionEntity, err := model.ToDomain()
		if err != nil {
			return nil, serrors.E(op, err)
		}
		sessions = append(sessions, sessionEntity)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	return sessions, nil
}

// CountUserSessions returns the total number of sessions for a user matching the same filter as ListUserSessions.
func (r *PostgresChatRepository) CountUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	const op serrors.Op = "PostgresChatRepository.CountUserSessions"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	var count int
	err = tx.QueryRow(ctx, countUserSessionsQuery, tenantID, userID, opts.IncludeArchived).Scan(&count)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}

// ListAccessibleSessionSummaries returns owned + shared sessions for a user.
func (r *PostgresChatRepository) ListAccessibleSessionSummaries(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.SessionSummary, error) {
	const op serrors.Op = "PostgresChatRepository.ListAccessibleSessionSummaries"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, listAccessibleSessionSummariesQuery, tenantID, userID, opts.Limit, opts.Offset, opts.IncludeArchived)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	out := make([]domain.SessionSummary, 0)
	for rows.Next() {
		summary, scanErr := scanSessionSummaryRow(rows)
		if scanErr != nil {
			return nil, serrors.E(op, scanErr)
		}
		out = append(out, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	return out, nil
}

// CountAccessibleSessions returns total owned + shared sessions for a user.
func (r *PostgresChatRepository) CountAccessibleSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	const op serrors.Op = "PostgresChatRepository.CountAccessibleSessions"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	var count int
	if err := tx.QueryRow(ctx, countAccessibleSessionsQuery, tenantID, userID, opts.IncludeArchived).Scan(&count); err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}

// ListAllSessionSummaries returns tenant-wide sessions (for read-all views).
func (r *PostgresChatRepository) ListAllSessionSummaries(ctx context.Context, requestingUserID int64, opts domain.ListOptions, ownerUserID *int64) ([]domain.SessionSummary, error) {
	const op serrors.Op = "PostgresChatRepository.ListAllSessionSummaries"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, listAllSessionSummariesQuery, tenantID, requestingUserID, opts.Limit, opts.Offset, opts.IncludeArchived, ownerUserID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	out := make([]domain.SessionSummary, 0)
	for rows.Next() {
		summary, scanErr := scanSessionSummaryRow(rows)
		if scanErr != nil {
			return nil, serrors.E(op, scanErr)
		}
		out = append(out, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	return out, nil
}

// CountAllSessions returns total tenant sessions for read-all views.
func (r *PostgresChatRepository) CountAllSessions(ctx context.Context, opts domain.ListOptions, ownerUserID *int64) (int, error) {
	const op serrors.Op = "PostgresChatRepository.CountAllSessions"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	var count int
	if err := tx.QueryRow(ctx, countAllSessionsQuery, tenantID, opts.IncludeArchived, ownerUserID).Scan(&count); err != nil {
		return 0, serrors.E(op, err)
	}
	return count, nil
}

// ResolveSessionAccess resolves owner/member access for a user.
func (r *PostgresChatRepository) ResolveSessionAccess(ctx context.Context, sessionID uuid.UUID, userID int64) (domain.SessionAccess, error) {
	const op serrors.Op = "PostgresChatRepository.ResolveSessionAccess"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return domain.SessionAccess{}, serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return domain.SessionAccess{}, serrors.E(op, err)
	}

	var (
		ownerID    int64
		memberRole string
	)
	if err := tx.QueryRow(ctx, resolveSessionAccessQuery, tenantID, sessionID, userID).Scan(&ownerID, &memberRole); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SessionAccess{}, serrors.E(op, ErrSessionNotFound)
		}
		return domain.SessionAccess{}, serrors.E(op, err)
	}

	if ownerID == userID {
		access, err := (&models.SessionAccessModel{
			Role:   domain.SessionMemberRoleOwner.String(),
			Source: string(domain.SessionAccessSourceOwner),
		}).ToDomain()
		if err != nil {
			return domain.SessionAccess{}, serrors.E(op, err)
		}
		return access, nil
	}

	switch domain.ParseSessionMemberRole(memberRole) {
	case domain.SessionMemberRoleEditor:
		access, err := (&models.SessionAccessModel{
			Role:   domain.SessionMemberRoleEditor.String(),
			Source: string(domain.SessionAccessSourceMember),
		}).ToDomain()
		if err != nil {
			return domain.SessionAccess{}, serrors.E(op, err)
		}
		return access, nil
	case domain.SessionMemberRoleViewer:
		access, err := (&models.SessionAccessModel{
			Role:   domain.SessionMemberRoleViewer.String(),
			Source: string(domain.SessionAccessSourceMember),
		}).ToDomain()
		if err != nil {
			return domain.SessionAccess{}, serrors.E(op, err)
		}
		return access, nil
	case domain.SessionMemberRoleNone, domain.SessionMemberRoleOwner, domain.SessionMemberRoleReadAll:
		// OWNER is already handled above and READ_ALL is permission-derived, not membership-derived.
		// Unknown/none roles should resolve to no access.
		fallthrough
	default:
		access, err := (&models.SessionAccessModel{
			Role:   domain.SessionMemberRoleNone.String(),
			Source: string(domain.SessionAccessSourceNone),
		}).ToDomain()
		if err != nil {
			return domain.SessionAccess{}, serrors.E(op, err)
		}
		return access, nil
	}
}

// ListSessionMembers returns explicit non-owner members for a session.
func (r *PostgresChatRepository) ListSessionMembers(ctx context.Context, sessionID uuid.UUID) ([]domain.SessionMember, error) {
	const op serrors.Op = "PostgresChatRepository.ListSessionMembers"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, listSessionMembersQuery, tenantID, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	out := make([]domain.SessionMember, 0)
	for rows.Next() {
		var model models.SessionMemberModel
		if err := rows.Scan(
			&model.SessionID,
			&model.UserID,
			&model.Role,
			&model.CreatedAt,
			&model.UpdatedAt,
			&model.FirstName,
			&model.LastName,
		); err != nil {
			return nil, serrors.E(op, err)
		}
		member, memberErr := model.ToDomain()
		if memberErr != nil {
			return nil, serrors.E(op, memberErr)
		}
		out = append(out, member)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	return out, nil
}

// UpsertSessionMember creates or updates an explicit non-owner member role.
func (r *PostgresChatRepository) UpsertSessionMember(ctx context.Context, command domain.SessionMemberUpsert) error {
	const op serrors.Op = "PostgresChatRepository.UpsertSessionMember"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	model := models.SessionMemberUpsertModelFromDomain(command)
	result, err := tx.Exec(ctx, upsertSessionMemberQuery, tenantID, model.SessionID, model.UserID, model.Role)
	if err != nil {
		return serrors.E(op, err)
	}
	if result.RowsAffected() == 0 {
		session, err := r.GetSession(ctx, model.SessionID)
		if err != nil {
			return serrors.E(op, err)
		}
		if session.UserID() == model.UserID {
			return serrors.E(op, serrors.KindValidation, "cannot add session owner as a member")
		}

		var exists bool
		if err := tx.QueryRow(ctx, sessionUserExistsQuery, tenantID, model.UserID).Scan(&exists); err != nil {
			return serrors.E(op, err)
		}
		if !exists {
			return serrors.E(op, ErrTenantUserNotFound)
		}

		return serrors.E(op, serrors.KindValidation, "failed to add or update session member")
	}
	return nil
}

// RemoveSessionMember removes an explicit non-owner member.
func (r *PostgresChatRepository) RemoveSessionMember(ctx context.Context, command domain.SessionMemberRemoval) error {
	const op serrors.Op = "PostgresChatRepository.RemoveSessionMember"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	model := models.SessionMemberRemovalModelFromDomain(command)
	if _, err := tx.Exec(ctx, removeSessionMemberQuery, tenantID, model.SessionID, model.UserID); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

// CountSessionParticipants returns owner + explicit member count.
func (r *PostgresChatRepository) CountSessionParticipants(ctx context.Context, sessionID uuid.UUID) (int, error) {
	const op serrors.Op = "PostgresChatRepository.CountSessionParticipants"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	var count int
	if err := tx.QueryRow(ctx, countSessionParticipantsQuery, tenantID, sessionID).Scan(&count); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, serrors.E(op, ErrSessionNotFound)
		}
		return 0, serrors.E(op, err)
	}
	return count, nil
}

// ListTenantUsers lists users in current tenant ordered by name.
// When a user group filter is configured, only members of that group are returned.
func (r *PostgresChatRepository) ListTenantUsers(ctx context.Context) ([]domain.SessionUser, error) {
	const op serrors.Op = "PostgresChatRepository.ListTenantUsers"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	var rows pgx.Rows
	if r.userGroupID != uuid.Nil {
		rows, err = tx.Query(ctx, listTenantUsersByGroupQuery, tenantID, r.userGroupID)
	} else {
		rows, err = tx.Query(ctx, listTenantUsersQuery, tenantID)
	}
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	out := make([]domain.SessionUser, 0)
	for rows.Next() {
		var u domain.SessionUser
		if err := rows.Scan(&u.ID, &u.FirstName, &u.LastName); err != nil {
			return nil, serrors.E(op, err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}
	return out, nil
}

// GetTenantUser returns a single tenant user by id.
func (r *PostgresChatRepository) GetTenantUser(ctx context.Context, userID int64) (domain.SessionUser, error) {
	const op serrors.Op = "PostgresChatRepository.GetTenantUser"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return domain.SessionUser{}, serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return domain.SessionUser{}, serrors.E(op, err)
	}

	var user domain.SessionUser
	if err := tx.QueryRow(ctx, getTenantUserQuery, tenantID, userID).Scan(&user.ID, &user.FirstName, &user.LastName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.SessionUser{}, serrors.E(op, ErrTenantUserNotFound)
		}
		return domain.SessionUser{}, serrors.E(op, err)
	}

	return user, nil
}

// DeleteSession deletes a session and all related data (cascades to messages/attachments).
func (r *PostgresChatRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "PostgresChatRepository.DeleteSession"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, deleteSessionQuery, tenantID, id)
	if err != nil {
		return serrors.E(op, err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return serrors.E(op, ErrSessionNotFound)
	}

	return nil
}

// Message operations

// SaveMessage saves a message to the database.
func (r *PostgresChatRepository) SaveMessage(ctx context.Context, msg types.Message) error {
	const op serrors.Op = "PostgresChatRepository.SaveMessage"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	model, err := messageDomainToModel(msg)
	if err != nil {
		return serrors.E(op, err)
	}

	if model.CreatedAt.IsZero() {
		model.CreatedAt = time.Now()
	}

	if model.Role == types.RoleUser && model.AuthorUserID == nil {
		var ownerUserID int64
		if err := tx.QueryRow(ctx, selectSessionOwnerUserIDQuery, tenantID, model.SessionID).Scan(&ownerUserID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return serrors.E(op, ErrSessionNotFound)
			}
			return serrors.E(op, err)
		}
		model.AuthorUserID = &ownerUserID
	}

	_, err = tx.Exec(ctx, insertMessageQuery,
		model.ID,
		model.SessionID,
		model.Role,
		model.Content,
		model.AuthorUserID,
		model.ToolCallsJSON,
		model.ToolCallID,
		model.CitationsJSON,
		model.DebugTraceJSON,
		model.QuestionDataJSON,
		model.CreatedAt,
	)
	if err != nil {
		return serrors.E(op, err)
	}

	if err := r.persistDebugTraceProjection(ctx, tx, tenantID, msg); err != nil {
		return serrors.E(op, err)
	}

	if len(msg.CodeOutputs()) > 0 {
		msgID := model.ID
		for _, output := range msg.CodeOutputs() {
			outputCreatedAt := output.CreatedAt
			if outputCreatedAt.IsZero() {
				outputCreatedAt = model.CreatedAt
			}

			codeOutputArtifact := domain.NewArtifact(
				domain.WithArtifactID(output.ID),
				domain.WithArtifactTenantID(tenantID),
				domain.WithArtifactSessionID(model.SessionID),
				domain.WithArtifactMessageID(&msgID),
				domain.WithArtifactType(domain.ArtifactTypeCodeOutput),
				domain.WithArtifactName(output.Name),
				domain.WithArtifactMimeType(output.MimeType),
				domain.WithArtifactURL(output.URL),
				domain.WithArtifactSizeBytes(output.Size),
				domain.WithArtifactCreatedAt(outputCreatedAt),
			)

			if err := r.SaveArtifact(ctx, codeOutputArtifact); err != nil {
				return serrors.E(op, err)
			}
		}
	}

	return nil
}

// GetMessage retrieves a message by ID.
func (r *PostgresChatRepository) GetMessage(ctx context.Context, id uuid.UUID) (types.Message, error) {
	const op serrors.Op = "PostgresChatRepository.GetMessage"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	model, err := scanMessageModel(tx.QueryRow(ctx, buildSelectMessageQuery(), tenantID, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serrors.E(op, ErrMessageNotFound)
		}
		return nil, serrors.E(op, err)
	}

	return r.hydrateMessageModel(ctx, tenantID, &model)
}

func (r *PostgresChatRepository) hydrateMessageModel(
	ctx context.Context,
	tenantID uuid.UUID,
	model *models.MessageModel,
) (types.Message, error) {
	codeOutputs, err := r.loadCodeOutputsForMessage(ctx, tenantID, model.ID)
	if err != nil {
		return nil, err
	}

	domainAttachments, err := r.GetMessageAttachments(ctx, model.ID)
	if err != nil {
		return nil, err
	}

	return messageModelToDomain(
		model,
		codeOutputs,
		convertDomainAttachmentsToTypes(domainAttachments),
	)
}

// GetSessionMessages retrieves all messages for a session with pagination.
func (r *PostgresChatRepository) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error) {
	const op serrors.Op = "PostgresChatRepository.GetSessionMessages"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, buildSelectSessionMessagesQuery(opts), tenantID, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	// First pass: collect all message data without nested queries
	// This avoids "conn busy" error from nested queries on the same connection
	modelsData := make([]models.MessageModel, 0)
	for rows.Next() {
		model, scanErr := scanMessageModel(rows)
		if scanErr != nil {
			return nil, serrors.E(op, scanErr)
		}
		modelsData = append(modelsData, model)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	// Second pass: load code outputs for each message (rows are now closed)
	messages := make([]types.Message, 0, len(modelsData))
	for _, model := range modelsData {
		message, hydrateErr := r.hydrateMessageModel(ctx, tenantID, &model)
		if hydrateErr != nil {
			return nil, serrors.E(op, hydrateErr)
		}
		messages = append(messages, message)
	}

	return messages, nil
}

// TruncateMessagesFrom deletes messages in a session from a given timestamp forward.
func (r *PostgresChatRepository) TruncateMessagesFrom(ctx context.Context, sessionID uuid.UUID, from time.Time) (int64, error) {
	const op serrors.Op = "PostgresChatRepository.TruncateMessagesFrom"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, truncateMessagesFromQuery, tenantID, sessionID, from)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	return result.RowsAffected(), nil
}

// UpdateMessageQuestionData updates the question_data field of a message.
func (r *PostgresChatRepository) UpdateMessageQuestionData(ctx context.Context, msgID uuid.UUID, qd *types.QuestionData) error {
	const op serrors.Op = "PostgresChatRepository.UpdateMessageQuestionData"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	qdJSON, err := json.Marshal(qd)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, updateMessageQuestionDataQuery, qdJSON, tenantID, msgID)
	if err != nil {
		return serrors.E(op, err)
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, ErrMessageNotFound)
	}

	return nil
}

// GetPendingQuestionMessage retrieves a pending question message for a session.
func (r *PostgresChatRepository) GetPendingQuestionMessage(ctx context.Context, sessionID uuid.UUID) (types.Message, error) {
	const op serrors.Op = "PostgresChatRepository.GetPendingQuestionMessage"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	model, err := scanMessageModel(tx.QueryRow(ctx, buildSelectPendingQuestionMessageQuery(), tenantID, sessionID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serrors.E(op, domain.ErrNoPendingQuestion)
		}
		return nil, serrors.E(op, err)
	}

	return r.hydrateMessageModel(ctx, tenantID, &model)
}

// Attachment operations

// SaveAttachment saves an attachment to the database.
func (r *PostgresChatRepository) SaveAttachment(ctx context.Context, attachment domain.Attachment) error {
	const op serrors.Op = "PostgresChatRepository.SaveAttachment"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	var sessionID uuid.UUID
	if err := tx.QueryRow(ctx, selectMessageSessionQuery, tenantID, attachment.MessageID()).Scan(&sessionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return serrors.E(op, ErrMessageNotFound)
		}
		return serrors.E(op, err)
	}

	createdAt := attachment.CreatedAt()
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	msgID := attachment.MessageID()

	artifactOpts := []domain.ArtifactOption{
		domain.WithArtifactID(attachment.ID()),
		domain.WithArtifactTenantID(tenantID),
		domain.WithArtifactSessionID(sessionID),
		domain.WithArtifactMessageID(&msgID),
		domain.WithArtifactType(domain.ArtifactTypeAttachment),
		domain.WithArtifactName(attachment.FileName()),
		domain.WithArtifactMimeType(attachment.MimeType()),
		domain.WithArtifactURL(attachment.FilePath()),
		domain.WithArtifactSizeBytes(attachment.SizeBytes()),
		domain.WithArtifactCreatedAt(createdAt),
	}
	if attachment.UploadID() != nil {
		artifactOpts = append(artifactOpts, domain.WithArtifactUploadID(*attachment.UploadID()))
	}

	err = r.SaveArtifact(ctx, domain.NewArtifact(artifactOpts...))
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// GetAttachment retrieves an attachment by ID.
func (r *PostgresChatRepository) GetAttachment(ctx context.Context, id uuid.UUID) (domain.Attachment, error) {
	const op serrors.Op = "PostgresChatRepository.GetAttachment"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	var (
		aid       uuid.UUID
		messageID *uuid.UUID
		uploadID  *int64
		fileName  string
		mimeType  *string
		sizeBytes int64
		filePath  *string
		createdAt time.Time
	)
	err = tx.QueryRow(ctx, selectAttachmentQuery, tenantID, id).Scan(
		&aid,
		&messageID,
		&uploadID,
		&fileName,
		&mimeType,
		&sizeBytes,
		&filePath,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serrors.E(op, ErrAttachmentNotFound)
		}
		return nil, serrors.E(op, err)
	}
	if messageID == nil {
		return nil, serrors.E(op, ErrAttachmentNotFound)
	}

	opts := []domain.AttachmentOption{
		domain.WithAttachmentID(aid),
		domain.WithAttachmentMessageID(*messageID),
		domain.WithFileName(fileName),
		domain.WithMimeType(derefString(mimeType)),
		domain.WithSizeBytes(sizeBytes),
		domain.WithFilePath(derefString(filePath)),
		domain.WithAttachmentCreatedAt(createdAt),
	}
	if uploadID != nil {
		opts = append(opts, domain.WithUploadID(*uploadID))
	}

	return domain.NewAttachment(opts...), nil
}

// GetMessageAttachments retrieves all attachments for a message.
func (r *PostgresChatRepository) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]domain.Attachment, error) {
	const op serrors.Op = "PostgresChatRepository.GetMessageAttachments"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, selectMessageAttachmentsQuery, tenantID, messageID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	var attachments []domain.Attachment
	for rows.Next() {
		var (
			aid       uuid.UUID
			msgID     *uuid.UUID
			uploadID  *int64
			fileName  string
			mimeType  *string
			sizeBytes int64
			filePath  *string
			createdAt time.Time
		)
		err := rows.Scan(
			&aid,
			&msgID,
			&uploadID,
			&fileName,
			&mimeType,
			&sizeBytes,
			&filePath,
			&createdAt,
		)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		if msgID == nil {
			continue
		}
		opts := []domain.AttachmentOption{
			domain.WithAttachmentID(aid),
			domain.WithAttachmentMessageID(*msgID),
			domain.WithFileName(fileName),
			domain.WithMimeType(derefString(mimeType)),
			domain.WithSizeBytes(sizeBytes),
			domain.WithFilePath(derefString(filePath)),
			domain.WithAttachmentCreatedAt(createdAt),
		}
		if uploadID != nil {
			opts = append(opts, domain.WithUploadID(*uploadID))
		}
		attachments = append(attachments, domain.NewAttachment(opts...))
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	return attachments, nil
}

// DeleteAttachment deletes an attachment.
func (r *PostgresChatRepository) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	if err := r.DeleteArtifact(ctx, id); err != nil {
		if errors.Is(err, ErrArtifactNotFound) {
			return serrors.E("PostgresChatRepository.DeleteAttachment", ErrAttachmentNotFound)
		}
		return err
	}
	return nil
}

// Generation run operations (refresh-safe streaming)

// CreateRun inserts a new streaming run. Returns domain.ErrActiveRunExists if session already has an active run.
func (r *PostgresChatRepository) CreateRun(ctx context.Context, run domain.GenerationRun) error {
	const op serrors.Op = "PostgresChatRepository.CreateRun"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	model, err := models.GenerationRunModelFromDomain(run)
	if err != nil {
		return serrors.E(op, err)
	}
	model.TenantID = tenantID

	_, err = tx.Exec(ctx, insertGenerationRunQuery,
		model.ID,
		model.SessionID,
		model.TenantID,
		model.UserID,
		model.Status,
		model.PartialContent,
		model.PartialMeta,
		model.StartedAt,
		model.LastUpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrActiveRunExists
		}
		return serrors.E(op, err)
	}
	return nil
}

// GetActiveRunBySession returns the active run for the session, or nil if none.
func (r *PostgresChatRepository) GetActiveRunBySession(ctx context.Context, sessionID uuid.UUID) (domain.GenerationRun, error) {
	const op serrors.Op = "PostgresChatRepository.GetActiveRunBySession"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	row := tx.QueryRow(ctx, selectActiveGenerationRunBySessionQuery, tenantID, sessionID)
	var model models.GenerationRunModel
	err = row.Scan(
		&model.ID,
		&model.SessionID,
		&model.TenantID,
		&model.UserID,
		&model.Status,
		&model.PartialContent,
		&model.PartialMeta,
		&model.StartedAt,
		&model.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNoActiveRun
		}
		return nil, serrors.E(op, err)
	}

	runEntity, err := model.ToDomain()
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return runEntity, nil
}

// GetRunByID returns a run by id regardless of status.
func (r *PostgresChatRepository) GetRunByID(ctx context.Context, runID uuid.UUID) (domain.GenerationRun, error) {
	const op serrors.Op = "PostgresChatRepository.GetRunByID"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	row := tx.QueryRow(ctx, selectGenerationRunByIDQuery, tenantID, runID)
	var model models.GenerationRunModel
	err = row.Scan(
		&model.ID,
		&model.SessionID,
		&model.TenantID,
		&model.UserID,
		&model.Status,
		&model.PartialContent,
		&model.PartialMeta,
		&model.StartedAt,
		&model.LastUpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serrors.E(op, domain.ErrRunNotFound)
		}
		return nil, serrors.E(op, err)
	}

	runEntity, err := model.ToDomain()
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return runEntity, nil
}

// UpdateRunSnapshot updates partial content and metadata for the run.
func (r *PostgresChatRepository) UpdateRunSnapshot(ctx context.Context, runID uuid.UUID, partialContent string, partialMetadata map[string]any) error {
	const op serrors.Op = "PostgresChatRepository.UpdateRunSnapshot"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	metaJSON, err := json.Marshal(partialMetadata)
	if err != nil {
		return serrors.E(op, err)
	}

	_, err = tx.Exec(ctx, updateGenerationRunSnapshotQuery, partialContent, metaJSON, time.Now(), tenantID, runID)
	if err != nil {
		return serrors.E(op, err)
	}
	return nil
}

// CompleteRun marks the run as completed.
func (r *PostgresChatRepository) CompleteRun(ctx context.Context, runID uuid.UUID) error {
	const op serrors.Op = "PostgresChatRepository.CompleteRun"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	_, err = tx.Exec(ctx, completeGenerationRunQuery, time.Now(), tenantID, runID)
	if err != nil {
		return serrors.E(op, err)
	}
	return nil
}

// CancelRun marks the run as cancelled.
func (r *PostgresChatRepository) CancelRun(ctx context.Context, runID uuid.UUID) error {
	const op serrors.Op = "PostgresChatRepository.CancelRun"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	_, err = tx.Exec(ctx, cancelGenerationRunQuery, time.Now(), tenantID, runID)
	if err != nil {
		return serrors.E(op, err)
	}
	return nil
}

// Helper methods

// loadCodeOutputsForMessage loads code output artifacts for a specific message.
func (r *PostgresChatRepository) loadCodeOutputsForMessage(ctx context.Context, tenantID uuid.UUID, messageID uuid.UUID) ([]types.CodeInterpreterOutput, error) {
	const op serrors.Op = "PostgresChatRepository.loadCodeOutputsForMessage"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, selectMessageCodeOutputArtifactsQuery, tenantID, messageID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	var outputs []types.CodeInterpreterOutput
	for rows.Next() {
		var output types.CodeInterpreterOutput
		err := rows.Scan(
			&output.ID,
			&output.MessageID,
			&output.Name,
			&output.MimeType,
			&output.URL,
			&output.Size,
			&output.CreatedAt,
		)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		outputs = append(outputs, output)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	return outputs, nil
}

func convertDomainAttachmentsToTypes(in []domain.Attachment) []types.Attachment {
	if len(in) == 0 {
		return nil
	}

	out := make([]types.Attachment, 0, len(in))
	for _, a := range in {
		out = append(out, types.Attachment{
			ID:        a.ID(),
			MessageID: a.MessageID(),
			UploadID:  a.UploadID(),
			FileName:  a.FileName(),
			MimeType:  a.MimeType(),
			SizeBytes: a.SizeBytes(),
			FilePath:  a.FilePath(),
			CreatedAt: a.CreatedAt(),
		})
	}

	return out
}

func scanSessionSummaryRow(rows pgx.Rows) (domain.SessionSummary, error) {
	var (
		model           models.SessionModel
		ownerFirstName  string
		ownerLastName   string
		accessRoleRaw   string
		accessSourceRaw string
		memberCount     int
	)

	if err := rows.Scan(
		&model.ID,
		&model.TenantID,
		&model.UserID,
		&model.Title,
		&model.Status,
		&model.Pinned,
		&model.ParentSessionID,
		&model.LLMPreviousResponseID,
		&model.CreatedAt,
		&model.UpdatedAt,
		&ownerFirstName,
		&ownerLastName,
		&accessRoleRaw,
		&accessSourceRaw,
		&memberCount,
	); err != nil {
		return domain.SessionSummary{}, err
	}

	sessionEntity, err := model.ToDomain()
	if err != nil {
		return domain.SessionSummary{}, err
	}

	role := domain.ParseSessionMemberRole(accessRoleRaw)
	source := domain.SessionAccessSourceNone
	switch strings.ToLower(strings.TrimSpace(accessSourceRaw)) {
	case "owner":
		source = domain.SessionAccessSourceOwner
	case "member":
		source = domain.SessionAccessSourceMember
	case "permission":
		source = domain.SessionAccessSourcePermission
	}

	access, err := (&models.SessionAccessModel{
		Role:   role.String(),
		Source: string(source),
	}).ToDomain()
	if err != nil {
		return domain.SessionSummary{}, err
	}
	return domain.NewSessionSummary(domain.SessionSummarySpec{
		Session: sessionEntity,
		Owner: domain.SessionUser{
			ID:        model.UserID,
			FirstName: ownerFirstName,
			LastName:  ownerLastName,
		},
		Access:      access,
		MemberCount: memberCount,
	})
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
