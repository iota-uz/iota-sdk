package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5"
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
			updated_at = $6
		WHERE tenant_id = $7 AND id = $8
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
		SELECT m.id, m.session_id, m.role, m.content, m.tool_calls, m.tool_call_id, m.citations, m.created_at
		FROM bichat_messages m
		JOIN bichat_sessions s ON m.session_id = s.id
		WHERE s.tenant_id = $1 AND m.id = $2
	`
	selectSessionMessagesQuery = `
		SELECT m.id, m.session_id, m.role, m.content, m.tool_calls, m.tool_call_id, m.citations, m.created_at
		FROM bichat_messages m
		JOIN bichat_sessions s ON m.session_id = s.id
		WHERE s.tenant_id = $1 AND m.session_id = $2
		ORDER BY m.created_at ASC
		LIMIT $3 OFFSET $4
	`
	truncateMessagesFromQuery = `
		DELETE FROM bichat_messages m
		USING bichat_sessions s
		WHERE m.session_id = s.id
		  AND s.tenant_id = $1
		  AND m.session_id = $2
		  AND m.created_at >= $3
	`

	// Attachment queries
	insertAttachmentQuery = `
		INSERT INTO bichat_attachments (
			id, message_id, file_name, mime_type, size_bytes, storage_path, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	selectAttachmentQuery = `
		SELECT a.id, a.message_id, a.file_name, a.mime_type, a.size_bytes, a.storage_path, a.created_at
		FROM bichat_attachments a
		JOIN bichat_messages m ON a.message_id = m.id
		JOIN bichat_sessions s ON m.session_id = s.id
		WHERE s.tenant_id = $1 AND a.id = $2
	`
	selectMessageAttachmentsQuery = `
		SELECT a.id, a.message_id, a.file_name, a.mime_type, a.size_bytes, a.storage_path, a.created_at
		FROM bichat_attachments a
		JOIN bichat_messages m ON a.message_id = m.id
		JOIN bichat_sessions s ON m.session_id = s.id
		WHERE s.tenant_id = $1 AND a.message_id = $2
		ORDER BY a.created_at ASC
	`
	deleteAttachmentQuery = `
		DELETE FROM bichat_attachments a
		USING bichat_messages m, bichat_sessions s
		WHERE a.message_id = m.id
		  AND m.session_id = s.id
		  AND s.tenant_id = $1
		  AND a.id = $2
	`

	// Code interpreter output queries
	insertCodeOutputQuery = `
		INSERT INTO bichat_code_interpreter_outputs (
			id, message_id, name, mime_type, url, size_bytes, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	selectMessageCodeOutputsQuery = `
		SELECT o.id, o.message_id, o.name, o.mime_type, o.url, o.size_bytes, o.created_at
		FROM bichat_code_interpreter_outputs o
		JOIN bichat_messages m ON o.message_id = m.id
		JOIN bichat_sessions s ON m.session_id = s.id
		WHERE s.tenant_id = $1 AND o.message_id = $2
		ORDER BY o.created_at ASC
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
	const op serrors.Op = "PostgresChatRepository.CreateSession"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
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
		return serrors.E(op, err)
	}

	return nil
}

// GetSession retrieves a session by ID.
func (r *PostgresChatRepository) GetSession(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
	const op serrors.Op = "PostgresChatRepository.GetSession"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serrors.E(op, ErrSessionNotFound)
		}
		return nil, serrors.E(op, err)
	}

	return &session, nil
}

// UpdateSession updates an existing session.
func (r *PostgresChatRepository) UpdateSession(ctx context.Context, session *domain.Session) error {
	const op serrors.Op = "PostgresChatRepository.UpdateSession"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	// Set updated_at in application layer
	session.UpdatedAt = time.Now()

	result, err := tx.Exec(ctx, updateSessionQuery,
		session.Title,
		session.Status,
		session.Pinned,
		session.ParentSessionID,
		session.PendingQuestionAgent,
		session.UpdatedAt,
		tenantID,
		session.ID,
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

// ListUserSessions lists all sessions for a user with pagination.
func (r *PostgresChatRepository) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]*domain.Session, error) {
	const op serrors.Op = "PostgresChatRepository.ListUserSessions"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, listUserSessionsQuery, tenantID, userID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, serrors.E(op, err)
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
			return nil, serrors.E(op, err)
		}
		sessions = append(sessions, &session)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	return sessions, nil
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
func (r *PostgresChatRepository) SaveMessage(ctx context.Context, msg *types.Message) error {
	const op serrors.Op = "PostgresChatRepository.SaveMessage"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	// Marshal JSONB fields
	toolCallsJSON, err := json.Marshal(msg.ToolCalls)
	if err != nil {
		return serrors.E(op, err)
	}

	citationsJSON, err := json.Marshal(msg.Citations)
	if err != nil {
		return serrors.E(op, err)
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
		return serrors.E(op, err)
	}

	// Save code interpreter outputs if present
	for _, output := range msg.CodeOutputs {
		if output.CreatedAt.IsZero() {
			output.CreatedAt = time.Now()
		}

		_, err = tx.Exec(ctx, insertCodeOutputQuery,
			output.ID,
			output.MessageID,
			output.Name,
			output.MimeType,
			output.URL,
			output.Size,
			output.CreatedAt,
		)
		if err != nil {
			return serrors.E(op, err)
		}
	}

	return nil
}

// GetMessage retrieves a message by ID.
func (r *PostgresChatRepository) GetMessage(ctx context.Context, id uuid.UUID) (*types.Message, error) {
	const op serrors.Op = "PostgresChatRepository.GetMessage"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	var msg types.Message
	var toolCallsJSON, citationsJSON []byte

	err = tx.QueryRow(ctx, selectMessageQuery, tenantID, id).Scan(
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
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serrors.E(op, ErrMessageNotFound)
		}
		return nil, serrors.E(op, err)
	}

	// Unmarshal JSONB fields
	if err := json.Unmarshal(toolCallsJSON, &msg.ToolCalls); err != nil {
		return nil, serrors.E(op, err)
	}

	if err := json.Unmarshal(citationsJSON, &msg.Citations); err != nil {
		return nil, serrors.E(op, err)
	}

	// Load code interpreter outputs
	codeOutputs, err := r.loadCodeOutputsForMessage(ctx, tenantID, msg.ID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	msg.CodeOutputs = codeOutputs

	return &msg, nil
}

// GetSessionMessages retrieves all messages for a session with pagination.
func (r *PostgresChatRepository) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]*types.Message, error) {
	const op serrors.Op = "PostgresChatRepository.GetSessionMessages"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, selectSessionMessagesQuery, tenantID, sessionID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	var messages []*types.Message
	for rows.Next() {
		var msg types.Message
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
			return nil, serrors.E(op, err)
		}

		// Unmarshal JSONB fields
		if err := json.Unmarshal(toolCallsJSON, &msg.ToolCalls); err != nil {
			return nil, serrors.E(op, err)
		}

		if err := json.Unmarshal(citationsJSON, &msg.Citations); err != nil {
			return nil, serrors.E(op, err)
		}

		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	// Load code interpreter outputs for all messages in a batch
	for _, msg := range messages {
		codeOutputs, err := r.loadCodeOutputsForMessage(ctx, tenantID, msg.ID)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		msg.CodeOutputs = codeOutputs
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

// Attachment operations

// SaveAttachment saves an attachment to the database.
func (r *PostgresChatRepository) SaveAttachment(ctx context.Context, attachment *domain.Attachment) error {
	const op serrors.Op = "PostgresChatRepository.SaveAttachment"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
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
		return serrors.E(op, err)
	}

	return nil
}

// GetAttachment retrieves an attachment by ID.
func (r *PostgresChatRepository) GetAttachment(ctx context.Context, id uuid.UUID) (*domain.Attachment, error) {
	const op serrors.Op = "PostgresChatRepository.GetAttachment"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	var attachment domain.Attachment
	err = tx.QueryRow(ctx, selectAttachmentQuery, tenantID, id).Scan(
		&attachment.ID,
		&attachment.MessageID,
		&attachment.FileName,
		&attachment.MimeType,
		&attachment.SizeBytes,
		&attachment.FilePath,
		&attachment.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serrors.E(op, ErrAttachmentNotFound)
		}
		return nil, serrors.E(op, err)
	}

	return &attachment, nil
}

// GetMessageAttachments retrieves all attachments for a message.
func (r *PostgresChatRepository) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]*domain.Attachment, error) {
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
			return nil, serrors.E(op, err)
		}
		attachments = append(attachments, &attachment)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	return attachments, nil
}

// DeleteAttachment deletes an attachment.
func (r *PostgresChatRepository) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "PostgresChatRepository.DeleteAttachment"

	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	result, err := tx.Exec(ctx, deleteAttachmentQuery, tenantID, id)
	if err != nil {
		return serrors.E(op, err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return serrors.E(op, ErrAttachmentNotFound)
	}

	return nil
}

// Helper methods

// loadCodeOutputsForMessage loads code interpreter outputs for a specific message.
func (r *PostgresChatRepository) loadCodeOutputsForMessage(ctx context.Context, tenantID uuid.UUID, messageID uuid.UUID) ([]types.CodeInterpreterOutput, error) {
	const op serrors.Op = "PostgresChatRepository.loadCodeOutputsForMessage"

	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	rows, err := tx.Query(ctx, selectMessageCodeOutputsQuery, tenantID, messageID)
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
