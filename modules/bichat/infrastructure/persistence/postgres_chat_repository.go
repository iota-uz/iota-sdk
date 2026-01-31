package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// SQL query constants
const (
	// Session queries
	insertSessionQuery = `
		INSERT INTO bichat_sessions (
			id, tenant_id, user_id, title, status, pinned,
			parent_session_id, pending_question_agent, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	selectSessionQuery = `
		SELECT id, tenant_id, user_id, title, status, pinned,
			   parent_session_id, pending_question_agent, created_at, updated_at
		FROM bichat_sessions
		WHERE tenant_id = $1 AND id = $2
	`
	updateSessionQuery = `
		UPDATE bichat_sessions
		SET title = $1, status = $2, pinned = $3,
			parent_session_id = $4, pending_question_agent = $5,
			updated_at = NOW()
		WHERE tenant_id = $6 AND id = $7
	`
	listUserSessionsQuery = `
		SELECT id, tenant_id, user_id, title, status, pinned,
			   parent_session_id, pending_question_agent, created_at, updated_at
		FROM bichat_sessions
		WHERE tenant_id = $1 AND user_id = $2
		ORDER BY pinned DESC, created_at DESC
		LIMIT $3 OFFSET $4
	`
	deleteSessionQuery = `
		DELETE FROM bichat_sessions
		WHERE tenant_id = $1 AND id = $2
	`

	// Message queries
	insertMessageQuery = `
		INSERT INTO bichat_messages (
			id, session_id, role, content, tool_calls, tool_call_id, citations, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	selectMessageQuery = `
		SELECT id, session_id, role, content, tool_calls, tool_call_id, citations, created_at
		FROM bichat_messages
		WHERE id = $1
	`
	selectSessionMessagesQuery = `
		SELECT id, session_id, role, content, tool_calls, tool_call_id, citations, created_at
		FROM bichat_messages
		WHERE session_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`
	truncateMessagesFromQuery = `
		DELETE FROM bichat_messages
		WHERE session_id = $1 AND created_at >= $2
	`

	// Attachment queries
	insertAttachmentQuery = `
		INSERT INTO bichat_attachments (
			id, message_id, file_name, mime_type, size_bytes, storage_path, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	selectAttachmentQuery = `
		SELECT id, message_id, file_name, mime_type, size_bytes, storage_path, created_at
		FROM bichat_attachments
		WHERE id = $1
	`
	selectMessageAttachmentsQuery = `
		SELECT id, message_id, file_name, mime_type, size_bytes, storage_path, created_at
		FROM bichat_attachments
		WHERE message_id = $1
		ORDER BY created_at ASC
	`
	deleteAttachmentQuery = `
		DELETE FROM bichat_attachments
		WHERE id = $1
	`
)

var (
	ErrSessionNotFound    = errors.New("session not found")
	ErrMessageNotFound    = errors.New("message not found")
	ErrAttachmentNotFound = errors.New("attachment not found")
)

// PostgresChatRepository implements ChatRepository using PostgreSQL.
type PostgresChatRepository struct{}

// NewPostgresChatRepository creates a new PostgreSQL chat repository.
func NewPostgresChatRepository() domain.ChatRepository {
	return &PostgresChatRepository{}
}

// Session operations

// CreateSession creates a new session in the database.
func (r *PostgresChatRepository) CreateSession(ctx context.Context, session *domain.Session) error {
	const op = "PostgresChatRepository.CreateSession"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = now
	}

	_, err = tx.Exec(ctx, insertSessionQuery,
		session.ID,
		tenantID,
		session.UserID,
		session.Title,
		session.Status,
		session.Pinned,
		session.ParentSessionID,
		session.PendingQuestionAgent,
		session.CreatedAt,
		session.UpdatedAt,
	)
	if err != nil {
		return err
	}

	return nil
}

// GetSession retrieves a session by ID.
func (r *PostgresChatRepository) GetSession(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	const op = "PostgresChatRepository.GetSession"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var session domain.Session
	err = tx.QueryRow(ctx, selectSessionQuery, tenantID, id).Scan(
		&session.ID,
		&session.TenantID,
		&session.UserID,
		&session.Title,
		&session.Status,
		&session.Pinned,
		&session.ParentSessionID,
		&session.PendingQuestionAgent,
		&session.CreatedAt,
		&session.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	return &session, nil
}

// UpdateSession updates an existing session.
func (r *PostgresChatRepository) UpdateSession(ctx context.Context, session *domain.Session) error {
	const op = "PostgresChatRepository.UpdateSession"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	result, err := tx.Exec(ctx, updateSessionQuery,
		session.Title,
		session.Status,
		session.Pinned,
		session.ParentSessionID,
		session.PendingQuestionAgent,
		tenantID,
		session.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrSessionNotFound
	}

	return nil
}

// ListUserSessions lists all sessions for a user with pagination.
func (r *PostgresChatRepository) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]*domain.Session, error) {
	const op = "PostgresChatRepository.ListUserSessions"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, listUserSessionsQuery, tenantID, userID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*domain.Session
	for rows.Next() {
		var session domain.Session
		err := rows.Scan(
			&session.ID,
			&session.TenantID,
			&session.UserID,
			&session.Title,
			&session.Status,
			&session.Pinned,
			&session.ParentSessionID,
			&session.PendingQuestionAgent,
			&session.CreatedAt,
			&session.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, &session)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return sessions, nil
}

// DeleteSession deletes a session and all related data (cascades to messages/attachments).
func (r *PostgresChatRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	const op = "PostgresChatRepository.DeleteSession"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return err
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	result, err := tx.Exec(ctx, deleteSessionQuery, tenantID, id)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrSessionNotFound
	}

	return nil
}

// Message operations

// SaveMessage saves a message to the database.
func (r *PostgresChatRepository) SaveMessage(ctx context.Context, msg *domain.Message) error {
	const op = "PostgresChatRepository.SaveMessage"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	// Marshal JSONB fields
	toolCallsJSON, err := json.Marshal(msg.ToolCalls)
	if err != nil {
		return err
	}

	citationsJSON, err := json.Marshal(msg.Citations)
	if err != nil {
		return err
	}

	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	_, err = tx.Exec(ctx, insertMessageQuery,
		msg.ID,
		msg.SessionID,
		msg.Role,
		msg.Content,
		toolCallsJSON,
		msg.ToolCallID,
		citationsJSON,
		msg.CreatedAt,
	)
	if err != nil {
		return err
	}

	return nil
}

// GetMessage retrieves a message by ID.
func (r *PostgresChatRepository) GetMessage(ctx context.Context, id uuid.UUID) (*domain.Message, error) {
	const op = "PostgresChatRepository.GetMessage"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var msg domain.Message
	var toolCallsJSON, citationsJSON []byte

	err = tx.QueryRow(ctx, selectMessageQuery, id).Scan(
		&msg.ID,
		&msg.SessionID,
		&msg.Role,
		&msg.Content,
		&toolCallsJSON,
		&msg.ToolCallID,
		&citationsJSON,
		&msg.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}

	// Unmarshal JSONB fields
	if err := json.Unmarshal(toolCallsJSON, &msg.ToolCalls); err != nil {
		return nil, err
	}

	if err := json.Unmarshal(citationsJSON, &msg.Citations); err != nil {
		return nil, err
	}

	return &msg, nil
}

// GetSessionMessages retrieves all messages for a session with pagination.
func (r *PostgresChatRepository) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]*domain.Message, error) {
	const op = "PostgresChatRepository.GetSessionMessages"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, selectSessionMessagesQuery, sessionID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		var msg domain.Message
		var toolCallsJSON, citationsJSON []byte

		err := rows.Scan(
			&msg.ID,
			&msg.SessionID,
			&msg.Role,
			&msg.Content,
			&toolCallsJSON,
			&msg.ToolCallID,
			&citationsJSON,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal JSONB fields
		if err := json.Unmarshal(toolCallsJSON, &msg.ToolCalls); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(citationsJSON, &msg.Citations); err != nil {
			return nil, err
		}

		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

// TruncateMessagesFrom deletes messages in a session from a given timestamp forward.
func (r *PostgresChatRepository) TruncateMessagesFrom(ctx context.Context, sessionID uuid.UUID, from time.Time) (int64, error) {
	const op = "PostgresChatRepository.TruncateMessagesFrom"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, err
	}

	result, err := tx.Exec(ctx, truncateMessagesFromQuery, sessionID, from)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}

// Attachment operations

// SaveAttachment saves an attachment to the database.
func (r *PostgresChatRepository) SaveAttachment(ctx context.Context, attachment *domain.Attachment) error {
	const op = "PostgresChatRepository.SaveAttachment"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	if attachment.CreatedAt.IsZero() {
		attachment.CreatedAt = time.Now()
	}

	_, err = tx.Exec(ctx, insertAttachmentQuery,
		attachment.ID,
		attachment.MessageID,
		attachment.FileName,
		attachment.MimeType,
		attachment.SizeBytes,
		attachment.FilePath,
		attachment.CreatedAt,
	)
	if err != nil {
		return err
	}

	return nil
}

// GetAttachment retrieves an attachment by ID.
func (r *PostgresChatRepository) GetAttachment(ctx context.Context, id uuid.UUID) (*domain.Attachment, error) {
	const op = "PostgresChatRepository.GetAttachment"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	var attachment domain.Attachment
	err = tx.QueryRow(ctx, selectAttachmentQuery, id).Scan(
		&attachment.ID,
		&attachment.MessageID,
		&attachment.FileName,
		&attachment.MimeType,
		&attachment.SizeBytes,
		&attachment.FilePath,
		&attachment.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAttachmentNotFound
		}
		return nil, err
	}

	return &attachment, nil
}

// GetMessageAttachments retrieves all attachments for a message.
func (r *PostgresChatRepository) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]*domain.Attachment, error) {
	const op = "PostgresChatRepository.GetMessageAttachments"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := tx.Query(ctx, selectMessageAttachmentsQuery, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []*domain.Attachment
	for rows.Next() {
		var attachment domain.Attachment
		err := rows.Scan(
			&attachment.ID,
			&attachment.MessageID,
			&attachment.FileName,
			&attachment.MimeType,
			&attachment.SizeBytes,
			&attachment.FilePath,
			&attachment.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, &attachment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return attachments, nil
}

// DeleteAttachment deletes an attachment.
func (r *PostgresChatRepository) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	const op = "PostgresChatRepository.DeleteAttachment"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	result, err := tx.Exec(ctx, deleteAttachmentQuery, id)
	if err != nil {
		return err
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return ErrAttachmentNotFound
	}

	return nil
}
