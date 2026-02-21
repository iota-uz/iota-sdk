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
		WHERE tenant_id = $3 AND id = $4 AND btrim(title) = ''
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

	// Message queries
	insertMessageQuery = `
		INSERT INTO bichat.messages (
			id, session_id, role, content, tool_calls, tool_call_id, citations, debug_trace, question_data, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	selectMessageQuery = `
		SELECT m.id, m.session_id, m.role, m.content, m.tool_calls, m.tool_call_id, m.citations, m.debug_trace, m.question_data, m.created_at
		FROM bichat.messages m
		JOIN bichat.sessions s ON m.session_id = s.id
		WHERE s.tenant_id = $1 AND m.id = $2
	`
	selectSessionMessagesQuery = `
		SELECT m.id, m.session_id, m.role, m.content, m.tool_calls, m.tool_call_id, m.citations, m.debug_trace, m.question_data, m.created_at
		FROM bichat.messages m
		JOIN bichat.sessions s ON m.session_id = s.id
		WHERE s.tenant_id = $1 AND m.session_id = $2
		ORDER BY m.created_at ASC
		LIMIT $3 OFFSET $4
	`
	truncateMessagesFromQuery = `
		DELETE FROM bichat.messages m
		USING bichat.sessions s
		WHERE m.session_id = s.id
		  AND s.tenant_id = $1
		  AND m.session_id = $2
		  AND m.created_at >= $3
	`
	updateMessageQuestionDataQuery = `
		UPDATE bichat.messages m
		SET question_data = $1
		FROM bichat.sessions s
		WHERE m.session_id = s.id
		  AND s.tenant_id = $2
		  AND m.id = $3
	`
	selectPendingQuestionMessageQuery = `
		SELECT m.id, m.session_id, m.role, m.content, m.tool_calls, m.tool_call_id, m.citations, m.debug_trace, m.question_data, m.created_at
		FROM bichat.messages m
		JOIN bichat.sessions s ON m.session_id = s.id
		WHERE s.tenant_id = $1 AND m.session_id = $2
		  AND m.question_data->>'status' = 'PENDING'
		ORDER BY m.created_at DESC, m.id DESC
		LIMIT 1
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

	createdAt := session.CreatedAt()
	updatedAt := session.UpdatedAt()
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	_, err = tx.Exec(ctx, insertSessionQuery,
		session.ID(),
		tenantID,
		session.UserID(),
		session.Title(),
		session.Status(),
		session.Pinned(),
		session.ParentSessionID(),
		session.LLMPreviousResponseID(),
		createdAt,
		updatedAt,
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

	var (
		sid                   uuid.UUID
		tenantIDRow           uuid.UUID
		userID                int64
		title                 string
		status                domain.SessionStatus
		pinned                bool
		parentSessionID       *uuid.UUID
		llmPreviousResponseID *string
		createdAt             time.Time
		updatedAt             time.Time
	)
	err = tx.QueryRow(ctx, selectSessionQuery, tenantID, id).Scan(
		&sid, &tenantIDRow, &userID, &title, &status, &pinned,
		&parentSessionID, &llmPreviousResponseID, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serrors.E(op, ErrSessionNotFound)
		}
		return nil, serrors.E(op, err)
	}

	opts := []domain.SessionOption{
		domain.WithID(sid),
		domain.WithTenantID(tenantIDRow),
		domain.WithUserID(userID),
		domain.WithTitle(title),
		domain.WithStatus(status),
		domain.WithPinned(pinned),
		domain.WithCreatedAt(createdAt),
		domain.WithUpdatedAt(updatedAt),
	}
	if parentSessionID != nil {
		opts = append(opts, domain.WithParentSessionID(*parentSessionID))
	}
	if llmPreviousResponseID != nil {
		opts = append(opts, domain.WithLLMPreviousResponseID(*llmPreviousResponseID))
	}
	return domain.NewSession(opts...), nil
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

	result, err := tx.Exec(ctx, updateSessionQuery,
		session.Title(),
		session.Status(),
		session.Pinned(),
		session.ParentSessionID(),
		session.LLMPreviousResponseID(),
		session.UpdatedAt(),
		tenantID,
		session.ID(),
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

// UpdateSessionTitleIfEmpty updates a session title only when it is currently blank.
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
		var (
			sid                   uuid.UUID
			tenantIDRow           uuid.UUID
			userIDRow             int64
			title                 string
			status                domain.SessionStatus
			pinned                bool
			parentSessionID       *uuid.UUID
			llmPreviousResponseID *string
			createdAt             time.Time
			updatedAt             time.Time
		)
		err := rows.Scan(
			&sid, &tenantIDRow, &userIDRow, &title, &status, &pinned,
			&parentSessionID, &llmPreviousResponseID, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		opts := []domain.SessionOption{
			domain.WithID(sid),
			domain.WithTenantID(tenantIDRow),
			domain.WithUserID(userIDRow),
			domain.WithTitle(title),
			domain.WithStatus(status),
			domain.WithPinned(pinned),
			domain.WithCreatedAt(createdAt),
			domain.WithUpdatedAt(updatedAt),
		}
		if parentSessionID != nil {
			opts = append(opts, domain.WithParentSessionID(*parentSessionID))
		}
		if llmPreviousResponseID != nil {
			opts = append(opts, domain.WithLLMPreviousResponseID(*llmPreviousResponseID))
		}
		sessions = append(sessions, domain.NewSession(opts...))
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

	// Marshal JSONB fields
	toolCallsJSON, err := json.Marshal(msg.ToolCalls())
	if err != nil {
		return serrors.E(op, err)
	}

	citationsJSON, err := json.Marshal(msg.Citations())
	if err != nil {
		return serrors.E(op, err)
	}

	debugTraceJSON, err := json.Marshal(msg.DebugTrace())
	if err != nil {
		return serrors.E(op, err)
	}

	questionDataJSON, err := json.Marshal(msg.QuestionData())
	if err != nil {
		return serrors.E(op, err)
	}

	createdAt := msg.CreatedAt()
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err = tx.Exec(ctx, insertMessageQuery,
		msg.ID(),
		msg.SessionID(),
		msg.Role(),
		msg.Content(),
		toolCallsJSON,
		msg.ToolCallID(),
		citationsJSON,
		debugTraceJSON,
		questionDataJSON,
		createdAt,
	)
	if err != nil {
		return serrors.E(op, err)
	}

	if len(msg.CodeOutputs()) > 0 {
		msgID := msg.ID()
		for _, output := range msg.CodeOutputs() {
			outputCreatedAt := output.CreatedAt
			if outputCreatedAt.IsZero() {
				outputCreatedAt = createdAt
			}

			codeOutputArtifact := domain.NewArtifact(
				domain.WithArtifactID(output.ID),
				domain.WithArtifactTenantID(tenantID),
				domain.WithArtifactSessionID(msg.SessionID()),
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

	var (
		msgID            uuid.UUID
		sessionID        uuid.UUID
		role             types.Role
		content          string
		toolCallsJSON    []byte
		toolCallID       *string
		citationsJSON    []byte
		debugTraceJSON   []byte
		questionDataJSON []byte
		createdAt        time.Time
	)

	err = tx.QueryRow(ctx, selectMessageQuery, tenantID, id).Scan(
		&msgID,
		&sessionID,
		&role,
		&content,
		&toolCallsJSON,
		&toolCallID,
		&citationsJSON,
		&debugTraceJSON,
		&questionDataJSON,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serrors.E(op, ErrMessageNotFound)
		}
		return nil, serrors.E(op, err)
	}

	// Unmarshal JSONB fields
	var toolCalls []types.ToolCall
	if err := json.Unmarshal(toolCallsJSON, &toolCalls); err != nil {
		return nil, serrors.E(op, err)
	}

	var citations []types.Citation
	if err := json.Unmarshal(citationsJSON, &citations); err != nil {
		return nil, serrors.E(op, err)
	}

	var debugTrace *types.DebugTrace
	if len(debugTraceJSON) > 0 && string(debugTraceJSON) != "null" {
		var trace types.DebugTrace
		if err := json.Unmarshal(debugTraceJSON, &trace); err != nil {
			return nil, serrors.E(op, err)
		}
		debugTrace = &trace
	}

	var questionData *types.QuestionData
	if len(questionDataJSON) > 0 && string(questionDataJSON) != "null" {
		var qd types.QuestionData
		if err := json.Unmarshal(questionDataJSON, &qd); err != nil {
			return nil, serrors.E(op, err)
		}
		questionData = &qd
	}

	// Load code interpreter outputs
	codeOutputs, err := r.loadCodeOutputsForMessage(ctx, tenantID, msgID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	domainAttachments, err := r.GetMessageAttachments(ctx, msgID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	attachments := convertDomainAttachmentsToTypes(domainAttachments)

	// Build message options
	opts := []types.MessageOption{
		types.WithMessageID(msgID),
		types.WithSessionID(sessionID),
		types.WithRole(role),
		types.WithContent(content),
		types.WithCreatedAt(createdAt),
	}
	if len(toolCalls) > 0 {
		opts = append(opts, types.WithToolCalls(toolCalls...))
	}
	if toolCallID != nil {
		opts = append(opts, types.WithToolCallID(*toolCallID))
	}
	if len(citations) > 0 {
		opts = append(opts, types.WithCitations(citations...))
	}
	if len(codeOutputs) > 0 {
		opts = append(opts, types.WithCodeOutputs(codeOutputs...))
	}
	if len(attachments) > 0 {
		opts = append(opts, types.WithAttachments(attachments...))
	}
	if debugTrace != nil {
		opts = append(opts, types.WithDebugTrace(debugTrace))
	}
	if questionData != nil {
		opts = append(opts, types.WithQuestionData(questionData))
	}

	return types.NewMessage(opts...), nil
}

// messageData holds intermediate message data before code outputs are loaded.
type messageData struct {
	msgID        uuid.UUID
	sessID       uuid.UUID
	role         types.Role
	content      string
	toolCalls    []types.ToolCall
	toolCallID   *string
	citations    []types.Citation
	debugTrace   *types.DebugTrace
	questionData *types.QuestionData
	createdAt    time.Time
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

	rows, err := tx.Query(ctx, selectSessionMessagesQuery, tenantID, sessionID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	// First pass: collect all message data without nested queries
	// This avoids "conn busy" error from nested queries on the same connection
	var messagesData []messageData
	for rows.Next() {
		var (
			msgID            uuid.UUID
			sessID           uuid.UUID
			role             types.Role
			content          string
			toolCallsJSON    []byte
			toolCallID       *string
			citationsJSON    []byte
			debugTraceJSON   []byte
			questionDataJSON []byte
			createdAt        time.Time
		)

		err := rows.Scan(
			&msgID,
			&sessID,
			&role,
			&content,
			&toolCallsJSON,
			&toolCallID,
			&citationsJSON,
			&debugTraceJSON,
			&questionDataJSON,
			&createdAt,
		)
		if err != nil {
			return nil, serrors.E(op, err)
		}

		// Unmarshal JSONB fields
		var toolCalls []types.ToolCall
		if err := json.Unmarshal(toolCallsJSON, &toolCalls); err != nil {
			return nil, serrors.E(op, err)
		}

		var citations []types.Citation
		if err := json.Unmarshal(citationsJSON, &citations); err != nil {
			return nil, serrors.E(op, err)
		}

		var debugTrace *types.DebugTrace
		if len(debugTraceJSON) > 0 && string(debugTraceJSON) != "null" {
			var trace types.DebugTrace
			if err := json.Unmarshal(debugTraceJSON, &trace); err != nil {
				return nil, serrors.E(op, err)
			}
			debugTrace = &trace
		}

		var questionData *types.QuestionData
		if len(questionDataJSON) > 0 && string(questionDataJSON) != "null" {
			var qd types.QuestionData
			if err := json.Unmarshal(questionDataJSON, &qd); err != nil {
				return nil, serrors.E(op, err)
			}
			questionData = &qd
		}

		messagesData = append(messagesData, messageData{
			msgID:        msgID,
			sessID:       sessID,
			role:         role,
			content:      content,
			toolCalls:    toolCalls,
			toolCallID:   toolCallID,
			citations:    citations,
			debugTrace:   debugTrace,
			questionData: questionData,
			createdAt:    createdAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	// Second pass: load code outputs for each message (rows are now closed)
	messages := make([]types.Message, 0, len(messagesData))
	for _, md := range messagesData {
		codeOutputs, err := r.loadCodeOutputsForMessage(ctx, tenantID, md.msgID)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		domainAttachments, err := r.GetMessageAttachments(ctx, md.msgID)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		attachments := convertDomainAttachmentsToTypes(domainAttachments)

		// Build message options
		msgOpts := []types.MessageOption{
			types.WithMessageID(md.msgID),
			types.WithSessionID(md.sessID),
			types.WithRole(md.role),
			types.WithContent(md.content),
			types.WithCreatedAt(md.createdAt),
		}
		if len(md.toolCalls) > 0 {
			msgOpts = append(msgOpts, types.WithToolCalls(md.toolCalls...))
		}
		if md.toolCallID != nil {
			msgOpts = append(msgOpts, types.WithToolCallID(*md.toolCallID))
		}
		if len(md.citations) > 0 {
			msgOpts = append(msgOpts, types.WithCitations(md.citations...))
		}
		if len(codeOutputs) > 0 {
			msgOpts = append(msgOpts, types.WithCodeOutputs(codeOutputs...))
		}
		if len(attachments) > 0 {
			msgOpts = append(msgOpts, types.WithAttachments(attachments...))
		}
		if md.debugTrace != nil {
			msgOpts = append(msgOpts, types.WithDebugTrace(md.debugTrace))
		}
		if md.questionData != nil {
			msgOpts = append(msgOpts, types.WithQuestionData(md.questionData))
		}

		messages = append(messages, types.NewMessage(msgOpts...))
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

	var (
		msgID            uuid.UUID
		sessID           uuid.UUID
		role             types.Role
		content          string
		toolCallsJSON    []byte
		toolCallID       *string
		citationsJSON    []byte
		debugTraceJSON   []byte
		questionDataJSON []byte
		createdAt        time.Time
	)

	err = tx.QueryRow(ctx, selectPendingQuestionMessageQuery, tenantID, sessionID).Scan(
		&msgID,
		&sessID,
		&role,
		&content,
		&toolCallsJSON,
		&toolCallID,
		&citationsJSON,
		&debugTraceJSON,
		&questionDataJSON,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, serrors.E(op, domain.ErrNoPendingQuestion)
		}
		return nil, serrors.E(op, err)
	}

	// Unmarshal JSONB fields
	var toolCalls []types.ToolCall
	if err := json.Unmarshal(toolCallsJSON, &toolCalls); err != nil {
		return nil, serrors.E(op, err)
	}

	var citations []types.Citation
	if err := json.Unmarshal(citationsJSON, &citations); err != nil {
		return nil, serrors.E(op, err)
	}

	var debugTrace *types.DebugTrace
	if len(debugTraceJSON) > 0 && string(debugTraceJSON) != "null" {
		var trace types.DebugTrace
		if err := json.Unmarshal(debugTraceJSON, &trace); err != nil {
			return nil, serrors.E(op, err)
		}
		debugTrace = &trace
	}

	var questionData *types.QuestionData
	if len(questionDataJSON) > 0 && string(questionDataJSON) != "null" {
		var qd types.QuestionData
		if err := json.Unmarshal(questionDataJSON, &qd); err != nil {
			return nil, serrors.E(op, err)
		}
		questionData = &qd
	}

	// Load code interpreter outputs
	codeOutputs, err := r.loadCodeOutputsForMessage(ctx, tenantID, msgID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	domainAttachments, err := r.GetMessageAttachments(ctx, msgID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	attachments := convertDomainAttachmentsToTypes(domainAttachments)

	// Build message options
	opts := []types.MessageOption{
		types.WithMessageID(msgID),
		types.WithSessionID(sessID),
		types.WithRole(role),
		types.WithContent(content),
		types.WithCreatedAt(createdAt),
	}
	if len(toolCalls) > 0 {
		opts = append(opts, types.WithToolCalls(toolCalls...))
	}
	if toolCallID != nil {
		opts = append(opts, types.WithToolCallID(*toolCallID))
	}
	if len(citations) > 0 {
		opts = append(opts, types.WithCitations(citations...))
	}
	if len(codeOutputs) > 0 {
		opts = append(opts, types.WithCodeOutputs(codeOutputs...))
	}
	if len(attachments) > 0 {
		opts = append(opts, types.WithAttachments(attachments...))
	}
	if debugTrace != nil {
		opts = append(opts, types.WithDebugTrace(debugTrace))
	}
	if questionData != nil {
		opts = append(opts, types.WithQuestionData(questionData))
	}

	return types.NewMessage(opts...), nil
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

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
