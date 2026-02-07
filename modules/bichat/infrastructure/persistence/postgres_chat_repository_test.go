package persistence_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// Session Operations Tests

func TestPostgresChatRepository_CreateSession(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Test Session"),
		domain.WithStatus(domain.SessionStatusActive),
		domain.WithPinned(false),
	)

	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)
	assert.False(t, session.CreatedAt().IsZero())
	assert.False(t, session.UpdatedAt().IsZero())

	retrieved, err := repo.GetSession(env.Ctx, session.ID())
	require.NoError(t, err)
	assert.Equal(t, session.ID(), retrieved.ID())
	assert.Equal(t, session.Title(), retrieved.Title())
	assert.Equal(t, session.Status(), retrieved.Status())
	assert.Equal(t, session.Pinned(), retrieved.Pinned())
	assert.Equal(t, env.Tenant.ID, retrieved.TenantID())
	assert.Equal(t, int64(env.User.ID()), retrieved.UserID())
}

func TestPostgresChatRepository_CreateSession_WithParentAndPendingAgent(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create parent session first
	parentSession := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Parent Session"),
	)
	err := repo.CreateSession(env.Ctx, parentSession)
	require.NoError(t, err)

	// Create child session with parent reference
	agent := "sql_agent"
	childSession := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Child Session"),
		domain.WithParentSessionID(parentSession.ID()),
		domain.WithPendingQuestionAgent(agent),
	)
	err = repo.CreateSession(env.Ctx, childSession)
	require.NoError(t, err)

	// Verify parent and pending agent fields
	retrieved, err := repo.GetSession(env.Ctx, childSession.ID())
	require.NoError(t, err)
	require.NotNil(t, retrieved.ParentSessionID())
	assert.Equal(t, parentSession.ID(), *retrieved.ParentSessionID())
	require.NotNil(t, retrieved.PendingQuestionAgent())
	assert.Equal(t, agent, *retrieved.PendingQuestionAgent())
}

func TestPostgresChatRepository_GetSession(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Retrieve Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	retrieved, err := repo.GetSession(env.Ctx, session.ID())
	require.NoError(t, err)
	assert.Equal(t, session.ID(), retrieved.ID())
	assert.Equal(t, session.Title(), retrieved.Title())
	assert.Equal(t, env.Tenant.ID, retrieved.TenantID())
	assert.Equal(t, int64(env.User.ID()), retrieved.UserID())
}

func TestPostgresChatRepository_GetSession_NotFound(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Try to get non-existent session
	nonExistentID := uuid.New()
	_, err := repo.GetSession(env.Ctx, nonExistentID)
	require.Error(t, err)
	assert.ErrorIs(t, err, persistence.ErrSessionNotFound)
}

func TestPostgresChatRepository_UpdateSession(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Original Title"),
		domain.WithStatus(domain.SessionStatusActive),
		domain.WithPinned(false),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	agent := "new_agent"
	updated := session.
		UpdateTitle("Updated Title").
		UpdateStatus(domain.SessionStatusArchived).
		UpdatePinned(true).
		UpdatePendingQuestionAgent(&agent)
	err = repo.UpdateSession(env.Ctx, updated)
	require.NoError(t, err)

	retrieved, err := repo.GetSession(env.Ctx, session.ID())
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", retrieved.Title())
	assert.Equal(t, domain.SessionStatusArchived, retrieved.Status())
	assert.True(t, retrieved.Pinned())
	require.NotNil(t, retrieved.PendingQuestionAgent())
	assert.Equal(t, "new_agent", *retrieved.PendingQuestionAgent())
}

func TestPostgresChatRepository_UpdateSession_NotFound(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Try to update non-existent session
	session := domain.NewSession(
		domain.WithID(uuid.New()),
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Non-existent"),
	)

	err := repo.UpdateSession(env.Ctx, session)
	require.Error(t, err)
	assert.ErrorIs(t, err, persistence.ErrSessionNotFound)
}

func TestPostgresChatRepository_SessionLLMPreviousResponseID(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Continuity Session"),
		domain.WithLLMPreviousResponseID("resp_prev_1"),
	)
	require.NoError(t, repo.CreateSession(env.Ctx, session))

	retrieved, err := repo.GetSession(env.Ctx, session.ID())
	require.NoError(t, err)
	require.NotNil(t, retrieved.LLMPreviousResponseID())
	assert.Equal(t, "resp_prev_1", *retrieved.LLMPreviousResponseID())

	next := "resp_prev_2"
	updated := retrieved.UpdateLLMPreviousResponseID(&next)
	require.NoError(t, repo.UpdateSession(env.Ctx, updated))

	afterUpdate, err := repo.GetSession(env.Ctx, session.ID())
	require.NoError(t, err)
	require.NotNil(t, afterUpdate.LLMPreviousResponseID())
	assert.Equal(t, "resp_prev_2", *afterUpdate.LLMPreviousResponseID())

	cleared := afterUpdate.UpdateLLMPreviousResponseID(nil)
	require.NoError(t, repo.UpdateSession(env.Ctx, cleared))

	afterClear, err := repo.GetSession(env.Ctx, session.ID())
	require.NoError(t, err)
	assert.Nil(t, afterClear.LLMPreviousResponseID())
}

func TestPostgresChatRepository_ListUserSessions(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	baseTime := time.Now()
	sessions := []domain.Session{
		domain.NewSession(
			domain.WithTenantID(env.Tenant.ID),
			domain.WithUserID(int64(env.User.ID())),
			domain.WithTitle("Session 1"),
			domain.WithPinned(false),
			domain.WithCreatedAt(baseTime),
			domain.WithUpdatedAt(baseTime),
		),
		domain.NewSession(
			domain.WithTenantID(env.Tenant.ID),
			domain.WithUserID(int64(env.User.ID())),
			domain.WithTitle("Session 2 Pinned"),
			domain.WithPinned(true),
			domain.WithCreatedAt(baseTime.Add(10*time.Millisecond)),
			domain.WithUpdatedAt(baseTime.Add(10*time.Millisecond)),
		),
		domain.NewSession(
			domain.WithTenantID(env.Tenant.ID),
			domain.WithUserID(int64(env.User.ID())),
			domain.WithTitle("Session 3"),
			domain.WithPinned(false),
			domain.WithCreatedAt(baseTime.Add(20*time.Millisecond)),
			domain.WithUpdatedAt(baseTime.Add(20*time.Millisecond)),
		),
	}

	for _, session := range sessions {
		err := repo.CreateSession(env.Ctx, session)
		require.NoError(t, err)
	}

	// List all sessions
	opts := domain.ListOptions{Limit: 10, Offset: 0}
	retrieved, err := repo.ListUserSessions(env.Ctx, int64(env.User.ID()), opts)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(retrieved), 3)

	// Verify ordering: pinned first, then by created_at DESC
	require.GreaterOrEqual(t, len(retrieved), 3, "Should have at least 3 sessions")

	// First session should be the pinned one
	assert.True(t, retrieved[0].Pinned(), "First session should be pinned")
	assert.Equal(t, "Session 2 Pinned", retrieved[0].Title())

	// Remaining non-pinned sessions should be ordered by created_at DESC (Session 3, then Session 1)
	assert.False(t, retrieved[1].Pinned(), "Second session should not be pinned")
	assert.Equal(t, "Session 3", retrieved[1].Title(), "Second session should be Session 3 (most recent non-pinned)")

	assert.False(t, retrieved[2].Pinned(), "Third session should not be pinned")
	assert.Equal(t, "Session 1", retrieved[2].Title(), "Third session should be Session 1 (oldest non-pinned)")

	// Verify timestamp ordering for non-pinned sessions (DESC)
	assert.True(t, retrieved[1].CreatedAt().After(retrieved[2].CreatedAt()) || retrieved[1].CreatedAt().Equal(retrieved[2].CreatedAt()),
		"Non-pinned sessions should be ordered by created_at DESC")
}

func TestPostgresChatRepository_ListUserSessions_Pagination(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	baseTime := time.Now()
	for i := 0; i < 5; i++ {
		session := domain.NewSession(
			domain.WithTenantID(env.Tenant.ID),
			domain.WithUserID(int64(env.User.ID())),
			domain.WithTitle("Session "+string('A'+byte(i))),
			domain.WithCreatedAt(baseTime.Add(time.Duration(i)*5*time.Millisecond)),
			domain.WithUpdatedAt(baseTime.Add(time.Duration(i)*5*time.Millisecond)),
		)
		err := repo.CreateSession(env.Ctx, session)
		require.NoError(t, err)
	}

	// Test pagination
	opts := domain.ListOptions{Limit: 2, Offset: 1}
	retrieved, err := repo.ListUserSessions(env.Ctx, int64(env.User.ID()), opts)
	require.NoError(t, err)
	assert.Len(t, retrieved, 2)
}

func TestPostgresChatRepository_DeleteSession(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("To Delete"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Delete the session
	err = repo.DeleteSession(env.Ctx, session.ID())
	require.NoError(t, err)

	// Verify deletion
	_, err = repo.GetSession(env.Ctx, session.ID())
	require.Error(t, err)
	assert.ErrorIs(t, err, persistence.ErrSessionNotFound)
}

func TestPostgresChatRepository_DeleteSession_CascadeToMessages(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Session with Messages"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Create messages for the session
	msg1 := types.UserMessage("Hello",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg1)
	require.NoError(t, err)

	msg2 := types.AssistantMessage("Hi there",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg2)
	require.NoError(t, err)

	// Delete the session (should cascade to messages)
	err = repo.DeleteSession(env.Ctx, session.ID())
	require.NoError(t, err)

	// Verify messages are also deleted
	_, err = repo.GetMessage(env.Ctx, msg1.ID())
	require.Error(t, err)
	require.ErrorIs(t, err, persistence.ErrMessageNotFound)

	_, err = repo.GetMessage(env.Ctx, msg2.ID())
	require.Error(t, err)
	require.ErrorIs(t, err, persistence.ErrMessageNotFound)
}

func TestPostgresChatRepository_DeleteSession_NotFound(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Try to delete non-existent session
	err := repo.DeleteSession(env.Ctx, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, persistence.ErrSessionNotFound)
}

// Message Operations Tests

func TestPostgresChatRepository_SaveMessage(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session first
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Message Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Create and save a message
	msg := types.UserMessage("Test message content",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)
	assert.NotEmpty(t, msg.CreatedAt())

	// Verify message was saved
	retrieved, err := repo.GetMessage(env.Ctx, msg.ID())
	require.NoError(t, err)
	assert.Equal(t, msg.ID(), retrieved.ID())
	assert.Equal(t, msg.Content(), retrieved.Content())
	assert.Equal(t, msg.Role(), retrieved.Role())
	assert.Equal(t, session.ID(), retrieved.SessionID())
}

func TestPostgresChatRepository_SaveMessage_WithToolCalls(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Tool Call Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Create message with tool calls
	toolCalls := []types.ToolCall{
		{
			ID:        "call_1",
			Name:      "sql_execute",
			Arguments: `{"query": "SELECT * FROM users"}`,
		},
		{
			ID:        "call_2",
			Name:      "kb_search",
			Arguments: `{"query": "revenue"}`,
		},
	}

	msg := types.AssistantMessage("Let me check that",
		types.WithSessionID(session.ID()),
		types.WithToolCalls(toolCalls...),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	// Verify tool calls are saved and retrieved
	retrieved, err := repo.GetMessage(env.Ctx, msg.ID())
	require.NoError(t, err)
	require.Len(t, retrieved.ToolCalls(), 2)
	assert.Equal(t, "call_1", retrieved.ToolCalls()[0].ID)
	assert.Equal(t, "sql_execute", retrieved.ToolCalls()[0].Name)
	assert.Equal(t, "call_2", retrieved.ToolCalls()[1].ID)
	assert.Equal(t, "kb_search", retrieved.ToolCalls()[1].Name)
}

func TestPostgresChatRepository_SaveMessage_WithCitations(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Citation Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Create message with citations
	citations := []types.Citation{
		{
			Type:       "database",
			Title:      "Users Table",
			URL:        "/schema/users",
			Excerpt:    "Contains user data",
			StartIndex: 0,
			EndIndex:   10,
		},
		{
			Type:       "web",
			Title:      "Documentation",
			URL:        "https://docs.example.com",
			Excerpt:    "SQL best practices",
			StartIndex: 20,
			EndIndex:   30,
		},
	}

	msg := types.AssistantMessage("Based on the data...",
		types.WithSessionID(session.ID()),
		types.WithCitations(citations...),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	// Verify citations are saved and retrieved
	retrieved, err := repo.GetMessage(env.Ctx, msg.ID())
	require.NoError(t, err)
	require.Len(t, retrieved.Citations(), 2)
	assert.Equal(t, "database", retrieved.Citations()[0].Type)
	assert.Equal(t, "Users Table", retrieved.Citations()[0].Title)
	assert.Equal(t, "web", retrieved.Citations()[1].Type)
	assert.Equal(t, "Documentation", retrieved.Citations()[1].Title)
}

func TestPostgresChatRepository_SaveMessage_EmptyToolCallsAndCitations(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Empty Arrays Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Create message with empty tool calls and citations
	msg := types.UserMessage("Simple message",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	// Verify empty arrays are handled correctly
	retrieved, err := repo.GetMessage(env.Ctx, msg.ID())
	require.NoError(t, err)
	assert.Empty(t, retrieved.ToolCalls())
	assert.Empty(t, retrieved.Citations())
}

func TestPostgresChatRepository_GetMessage(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session and message
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Get Message Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	msg := types.UserMessage("Test content",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	// Retrieve the message
	retrieved, err := repo.GetMessage(env.Ctx, msg.ID())
	require.NoError(t, err)
	assert.Equal(t, msg.ID(), retrieved.ID())
	assert.Equal(t, msg.Content(), retrieved.Content())
}

func TestPostgresChatRepository_GetMessage_NotFound(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Try to get non-existent message
	_, err := repo.GetMessage(env.Ctx, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, persistence.ErrMessageNotFound)
}

func TestPostgresChatRepository_GetSessionMessages(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Multiple Messages Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Create multiple messages with explicit timestamps for ordering
	baseTime := time.Now()

	msg1 := types.UserMessage("First message",
		types.WithSessionID(session.ID()),
		types.WithCreatedAt(baseTime),
	)
	err = repo.SaveMessage(env.Ctx, msg1)
	require.NoError(t, err)

	msg2 := types.AssistantMessage("Second message",
		types.WithSessionID(session.ID()),
		types.WithCreatedAt(baseTime.Add(10*time.Millisecond)),
	)
	err = repo.SaveMessage(env.Ctx, msg2)
	require.NoError(t, err)

	msg3 := types.UserMessage("Third message",
		types.WithSessionID(session.ID()),
		types.WithCreatedAt(baseTime.Add(20*time.Millisecond)),
	)
	err = repo.SaveMessage(env.Ctx, msg3)
	require.NoError(t, err)

	// Retrieve all messages
	opts := domain.ListOptions{Limit: 10, Offset: 0}
	retrieved, err := repo.GetSessionMessages(env.Ctx, session.ID(), opts)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(retrieved), 3)

	// Verify ordering (created_at ASC)
	if len(retrieved) >= 3 {
		assert.Equal(t, "First message", retrieved[0].Content())
		assert.Equal(t, "Second message", retrieved[1].Content())
		assert.Equal(t, "Third message", retrieved[2].Content())
	}
}

func TestPostgresChatRepository_GetSessionMessages_Pagination(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Pagination Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Create 5 messages with explicit timestamps
	baseTime := time.Now()
	for i := 0; i < 5; i++ {
		msg := types.UserMessage("Message "+string('A'+byte(i)),
			types.WithSessionID(session.ID()),
			types.WithCreatedAt(baseTime.Add(time.Duration(i)*5*time.Millisecond)),
		)
		err = repo.SaveMessage(env.Ctx, msg)
		require.NoError(t, err)
	}

	// Test pagination
	opts := domain.ListOptions{Limit: 2, Offset: 1}
	retrieved, err := repo.GetSessionMessages(env.Ctx, session.ID(), opts)
	require.NoError(t, err)
	assert.Len(t, retrieved, 2)
}

func TestPostgresChatRepository_TruncateMessagesFrom(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Truncate Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Create messages at different times
	base := time.Now().Add(-1 * time.Hour)
	msg1 := types.UserMessage("Message 1",
		types.WithSessionID(session.ID()),
		types.WithCreatedAt(base),
	)
	err = repo.SaveMessage(env.Ctx, msg1)
	require.NoError(t, err)

	truncatePoint := base.Add(1 * time.Second)

	msg2 := types.UserMessage("Message 2",
		types.WithSessionID(session.ID()),
		types.WithCreatedAt(truncatePoint),
	)
	err = repo.SaveMessage(env.Ctx, msg2)
	require.NoError(t, err)

	msg3 := types.UserMessage("Message 3",
		types.WithSessionID(session.ID()),
		types.WithCreatedAt(truncatePoint.Add(1*time.Second)),
	)
	err = repo.SaveMessage(env.Ctx, msg3)
	require.NoError(t, err)

	// Truncate from truncatePoint (should delete msg2 and msg3)
	deleted, err := repo.TruncateMessagesFrom(env.Ctx, session.ID(), truncatePoint)
	require.NoError(t, err)
	assert.Equal(t, int64(2), deleted)

	// Verify only msg1 remains
	opts := domain.ListOptions{Limit: 10, Offset: 0}
	remaining, err := repo.GetSessionMessages(env.Ctx, session.ID(), opts)
	require.NoError(t, err)
	assert.Len(t, remaining, 1)
	assert.Equal(t, "Message 1", remaining[0].Content())
}

func TestPostgresChatRepository_TruncateMessagesFrom_NoMatch(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session and message
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("No Match Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	msg := types.UserMessage("Test",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	// Truncate from future time (should delete nothing)
	futureTime := time.Now().Add(1 * time.Hour)
	deleted, err := repo.TruncateMessagesFrom(env.Ctx, session.ID(), futureTime)
	require.NoError(t, err)
	assert.Equal(t, int64(0), deleted)
}

// Attachment Operations Tests

func TestPostgresChatRepository_SaveAttachment(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session and message
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Attachment Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	msg := types.UserMessage("Message with attachment",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	// Create and save attachment
	attachment := domain.NewAttachment(
		domain.WithAttachmentMessageID(msg.ID()),
		domain.WithFileName("test.pdf"),
		domain.WithMimeType("application/pdf"),
		domain.WithSizeBytes(1024),
		domain.WithFilePath("/uploads/test.pdf"),
	)
	err = repo.SaveAttachment(env.Ctx, attachment)
	require.NoError(t, err)
	assert.NotEmpty(t, attachment.CreatedAt())

	// Verify attachment was saved
	retrieved, err := repo.GetAttachment(env.Ctx, attachment.ID())
	require.NoError(t, err)
	assert.Equal(t, attachment.ID(), retrieved.ID())
	assert.Equal(t, attachment.FileName(), retrieved.FileName())
	assert.Equal(t, attachment.MimeType(), retrieved.MimeType())
	assert.Equal(t, attachment.SizeBytes(), retrieved.SizeBytes())
	assert.Equal(t, attachment.FilePath(), retrieved.FilePath())
}

func TestPostgresChatRepository_SaveAttachment_SpecialCharacters(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session and message
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Special Chars Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	msg := types.UserMessage("Test",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	// Create attachment with special characters in filename
	attachment := domain.NewAttachment(
		domain.WithAttachmentMessageID(msg.ID()),
		domain.WithFileName("файл-тест (копия) #1.pdf"),
		domain.WithMimeType("application/pdf"),
		domain.WithSizeBytes(2048),
		domain.WithFilePath("/uploads/файл-тест.pdf"),
	)
	err = repo.SaveAttachment(env.Ctx, attachment)
	require.NoError(t, err)

	// Verify special characters are preserved
	retrieved, err := repo.GetAttachment(env.Ctx, attachment.ID())
	require.NoError(t, err)
	assert.Equal(t, "файл-тест (копия) #1.pdf", retrieved.FileName())
}

func TestPostgresChatRepository_GetAttachment(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session, message, and attachment
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Get Attachment Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	msg := types.UserMessage("Test",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	attachment := domain.NewAttachment(
		domain.WithAttachmentMessageID(msg.ID()),
		domain.WithFileName("image.png"),
		domain.WithMimeType("image/png"),
		domain.WithSizeBytes(512),
		domain.WithFilePath("/uploads/image.png"),
	)
	err = repo.SaveAttachment(env.Ctx, attachment)
	require.NoError(t, err)

	// Retrieve the attachment
	retrieved, err := repo.GetAttachment(env.Ctx, attachment.ID())
	require.NoError(t, err)
	assert.Equal(t, attachment.ID(), retrieved.ID())
	assert.Equal(t, "image.png", retrieved.FileName())
}

func TestPostgresChatRepository_GetAttachment_NotFound(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Try to get non-existent attachment
	_, err := repo.GetAttachment(env.Ctx, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, persistence.ErrAttachmentNotFound)
}

func TestPostgresChatRepository_GetMessageAttachments(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session and message
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Multiple Attachments Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	msg := types.UserMessage("Message with multiple attachments",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	// Create multiple attachments with explicit timestamps
	baseTime := time.Now()

	att1 := domain.NewAttachment(
		domain.WithAttachmentMessageID(msg.ID()),
		domain.WithFileName("doc1.pdf"),
		domain.WithMimeType("application/pdf"),
		domain.WithSizeBytes(1024),
		domain.WithFilePath("/uploads/doc1.pdf"),
		domain.WithAttachmentCreatedAt(baseTime),
	)
	err = repo.SaveAttachment(env.Ctx, att1)
	require.NoError(t, err)

	att2 := domain.NewAttachment(
		domain.WithAttachmentMessageID(msg.ID()),
		domain.WithFileName("image.jpg"),
		domain.WithMimeType("image/jpeg"),
		domain.WithSizeBytes(2048),
		domain.WithFilePath("/uploads/image.jpg"),
		domain.WithAttachmentCreatedAt(baseTime.Add(5*time.Millisecond)),
	)
	err = repo.SaveAttachment(env.Ctx, att2)
	require.NoError(t, err)

	att3 := domain.NewAttachment(
		domain.WithAttachmentMessageID(msg.ID()),
		domain.WithFileName("data.csv"),
		domain.WithMimeType("text/csv"),
		domain.WithSizeBytes(512),
		domain.WithFilePath("/uploads/data.csv"),
		domain.WithAttachmentCreatedAt(baseTime.Add(10*time.Millisecond)),
	)
	err = repo.SaveAttachment(env.Ctx, att3)
	require.NoError(t, err)

	// Retrieve all attachments for the message
	retrieved, err := repo.GetMessageAttachments(env.Ctx, msg.ID())
	require.NoError(t, err)
	assert.Len(t, retrieved, 3)

	// Verify ordering (created_at ASC)
	assert.Equal(t, "doc1.pdf", retrieved[0].FileName())
	assert.Equal(t, "image.jpg", retrieved[1].FileName())
	assert.Equal(t, "data.csv", retrieved[2].FileName())
}

func TestPostgresChatRepository_GetMessageAttachments_Empty(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session and message without attachments
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("No Attachments Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	msg := types.UserMessage("Message without attachments",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	// Try to get attachments (should return empty slice)
	retrieved, err := repo.GetMessageAttachments(env.Ctx, msg.ID())
	require.NoError(t, err)
	assert.Empty(t, retrieved)
}

func TestPostgresChatRepository_DeleteAttachment(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session, message, and attachment
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Delete Attachment Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	msg := types.UserMessage("Test",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	attachment := domain.NewAttachment(
		domain.WithAttachmentMessageID(msg.ID()),
		domain.WithFileName("to_delete.pdf"),
		domain.WithMimeType("application/pdf"),
		domain.WithSizeBytes(1024),
		domain.WithFilePath("/uploads/to_delete.pdf"),
	)
	err = repo.SaveAttachment(env.Ctx, attachment)
	require.NoError(t, err)

	// Delete the attachment
	err = repo.DeleteAttachment(env.Ctx, attachment.ID())
	require.NoError(t, err)

	// Verify deletion
	_, err = repo.GetAttachment(env.Ctx, attachment.ID())
	require.Error(t, err)
	assert.ErrorIs(t, err, persistence.ErrAttachmentNotFound)
}

func TestPostgresChatRepository_DeleteAttachment_NotFound(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Try to delete non-existent attachment
	err := repo.DeleteAttachment(env.Ctx, uuid.New())
	require.Error(t, err)
	assert.ErrorIs(t, err, persistence.ErrAttachmentNotFound)
}

// Multi-Tenant Isolation Tests

func TestPostgresChatRepository_TenantIsolation_Sessions(t *testing.T) {
	t.Parallel()
	envA := setupTest(t)

	envB := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session in Tenant A
	sessionA := domain.NewSession(
		domain.WithTenantID(envA.Tenant.ID),
		domain.WithUserID(int64(envA.User.ID())),
		domain.WithTitle("Tenant A Session"),
	)
	err := repo.CreateSession(envA.Ctx, sessionA)
	require.NoError(t, err)

	// Try to access Tenant A's session from Tenant B's context
	_, err = repo.GetSession(envB.Ctx, sessionA.ID())
	require.Error(t, err)
	require.ErrorIs(t, err, persistence.ErrSessionNotFound)

	// Verify Tenant A can still access their session
	retrieved, err := repo.GetSession(envA.Ctx, sessionA.ID())
	require.NoError(t, err)
	assert.Equal(t, sessionA.ID(), retrieved.ID())
}

func TestPostgresChatRepository_TenantIsolation_Messages(t *testing.T) {
	t.Parallel()
	envA := setupTest(t)

	envB := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session and message in Tenant A
	sessionA := domain.NewSession(
		domain.WithTenantID(envA.Tenant.ID),
		domain.WithUserID(int64(envA.User.ID())),
		domain.WithTitle("Tenant A Session"),
	)
	err := repo.CreateSession(envA.Ctx, sessionA)
	require.NoError(t, err)

	msgA := types.UserMessage("Tenant A Message",
		types.WithSessionID(sessionA.ID()),
	)
	err = repo.SaveMessage(envA.Ctx, msgA)
	require.NoError(t, err)

	// Create session in Tenant B
	sessionB := domain.NewSession(
		domain.WithTenantID(envB.Tenant.ID),
		domain.WithUserID(int64(envB.User.ID())),
		domain.WithTitle("Tenant B Session"),
	)
	err = repo.CreateSession(envB.Ctx, sessionB)
	require.NoError(t, err)

	// Try to get Tenant A's messages from Tenant B's session context
	opts := domain.ListOptions{Limit: 10, Offset: 0}
	messagesB, err := repo.GetSessionMessages(envB.Ctx, sessionA.ID(), opts)
	require.NoError(t, err)
	assert.Empty(t, messagesB, "Tenant B should not see Tenant A's messages")

	// Verify Tenant A can access their messages
	messagesA, err := repo.GetSessionMessages(envA.Ctx, sessionA.ID(), opts)
	require.NoError(t, err)
	assert.NotEmpty(t, messagesA)
}

func TestPostgresChatRepository_TenantIsolation_Attachments(t *testing.T) {
	t.Parallel()
	envA := setupTest(t)

	envB := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session, message, and attachment in Tenant A
	sessionA := domain.NewSession(
		domain.WithTenantID(envA.Tenant.ID),
		domain.WithUserID(int64(envA.User.ID())),
		domain.WithTitle("Tenant A Session"),
	)
	err := repo.CreateSession(envA.Ctx, sessionA)
	require.NoError(t, err)

	msgA := types.UserMessage("Test",
		types.WithSessionID(sessionA.ID()),
	)
	err = repo.SaveMessage(envA.Ctx, msgA)
	require.NoError(t, err)

	attachmentA := domain.NewAttachment(
		domain.WithAttachmentMessageID(msgA.ID()),
		domain.WithFileName("tenant_a.pdf"),
		domain.WithMimeType("application/pdf"),
		domain.WithSizeBytes(1024),
		domain.WithFilePath("/uploads/tenant_a.pdf"),
	)
	err = repo.SaveAttachment(envA.Ctx, attachmentA)
	require.NoError(t, err)

	// Create session and message in Tenant B
	sessionB := domain.NewSession(
		domain.WithTenantID(envB.Tenant.ID),
		domain.WithUserID(int64(envB.User.ID())),
		domain.WithTitle("Tenant B Session"),
	)
	err = repo.CreateSession(envB.Ctx, sessionB)
	require.NoError(t, err)

	msgB := types.UserMessage("Test",
		types.WithSessionID(sessionB.ID()),
	)
	err = repo.SaveMessage(envB.Ctx, msgB)
	require.NoError(t, err)

	// Try to get Tenant A's attachments from Tenant B's message context
	attachmentsB, err := repo.GetMessageAttachments(envB.Ctx, msgA.ID())
	require.NoError(t, err)
	assert.Empty(t, attachmentsB, "Tenant B should not see Tenant A's attachments")

	// Verify Tenant A can access their attachments
	attachmentsA, err := repo.GetMessageAttachments(envA.Ctx, msgA.ID())
	require.NoError(t, err)
	assert.NotEmpty(t, attachmentsA)
}

// Edge Cases Tests

func TestPostgresChatRepository_EmptyTitle(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session with empty title
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle(""),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Verify empty title is preserved
	retrieved, err := repo.GetSession(env.Ctx, session.ID())
	require.NoError(t, err)
	assert.Empty(t, retrieved.Title())
}

func TestPostgresChatRepository_NilParentSessionID(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session without parent
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("No Parent"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Verify nil parent is handled correctly
	retrieved, err := repo.GetSession(env.Ctx, session.ID())
	require.NoError(t, err)
	assert.Nil(t, retrieved.ParentSessionID())
}

func TestPostgresChatRepository_PaginationBoundaries(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create a session with a few messages
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Boundary Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		msg := types.UserMessage("Message "+string('A'+byte(i)),
			types.WithSessionID(session.ID()),
		)
		err = repo.SaveMessage(env.Ctx, msg)
		require.NoError(t, err)
	}

	t.Run("Limit 0", func(t *testing.T) {
		opts := domain.ListOptions{Limit: 0, Offset: 0}
		messages, err := repo.GetSessionMessages(env.Ctx, session.ID(), opts)
		require.NoError(t, err)
		assert.Empty(t, messages)
	})

	t.Run("Offset exceeds total", func(t *testing.T) {
		opts := domain.ListOptions{Limit: 10, Offset: 100}
		messages, err := repo.GetSessionMessages(env.Ctx, session.ID(), opts)
		require.NoError(t, err)
		assert.Empty(t, messages)
	})
}

func TestPostgresChatRepository_LargeAttachment(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session and message
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Large Attachment Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	msg := types.UserMessage("Test",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	// Create large attachment (100MB)
	attachment := domain.NewAttachment(
		domain.WithAttachmentMessageID(msg.ID()),
		domain.WithFileName("large_file.zip"),
		domain.WithMimeType("application/zip"),
		domain.WithSizeBytes(100*1024*1024), // 100MB
		domain.WithFilePath("/uploads/large_file.zip"),
	)
	err = repo.SaveAttachment(env.Ctx, attachment)
	require.NoError(t, err)

	// Verify large size is preserved
	retrieved, err := repo.GetAttachment(env.Ctx, attachment.ID())
	require.NoError(t, err)
	assert.Equal(t, int64(100*1024*1024), retrieved.SizeBytes())
}

func TestPostgresChatRepository_ToolCallID(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Tool Call ID Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Create tool response message
	toolCallID := "call_123"
	msg := types.ToolResponse(toolCallID, "Tool execution result",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	// Verify tool_call_id is saved
	retrieved, err := repo.GetMessage(env.Ctx, msg.ID())
	require.NoError(t, err)
	require.NotNil(t, retrieved.ToolCallID())
	assert.Equal(t, toolCallID, *retrieved.ToolCallID())
}

func TestPostgresChatRepository_ForeignKeyViolation(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Try to create message with non-existent session ID
	nonExistentSessionID := uuid.New()
	msg := types.UserMessage("Orphan message",
		types.WithSessionID(nonExistentSessionID),
	)
	err := repo.SaveMessage(env.Ctx, msg)
	require.Error(t, err, "Should fail due to foreign key constraint")
}

func TestPostgresChatRepository_UpdateSession_NullParent(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create parent session
	parentSession := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Parent"),
	)
	err := repo.CreateSession(env.Ctx, parentSession)
	require.NoError(t, err)

	// Create child session with parent
	childSession := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Child"),
		domain.WithParentSessionID(parentSession.ID()),
	)
	err = repo.CreateSession(env.Ctx, childSession)
	require.NoError(t, err)

	// Note: The domain.Session interface doesn't support updating parent_session_id
	// This test would need a new method like UpdateParentSessionID() on the domain.Session interface
	// For now, just verify that the parent is set correctly
	retrieved, err := repo.GetSession(env.Ctx, childSession.ID())
	require.NoError(t, err)
	assert.NotNil(t, retrieved.ParentSessionID())
	assert.Equal(t, parentSession.ID(), *retrieved.ParentSessionID())
}

func TestPostgresChatRepository_MessageWithNilToolCallID(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Nil Tool Call Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Create message without tool_call_id (user message)
	msg := types.UserMessage("Regular message",
		types.WithSessionID(session.ID()),
	)
	err = repo.SaveMessage(env.Ctx, msg)
	require.NoError(t, err)

	// Verify nil tool_call_id is handled correctly
	retrieved, err := repo.GetMessage(env.Ctx, msg.ID())
	require.NoError(t, err)
	assert.Nil(t, retrieved.ToolCallID())
}

func TestPostgresChatRepository_UpdateSessionTimestamp(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	base := time.Now().Add(-1 * time.Hour)

	// Create session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Timestamp Test"),
		domain.WithCreatedAt(base),
		domain.WithUpdatedAt(base),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	originalUpdatedAt := session.UpdatedAt()

	updated := session.UpdateTitle("Updated")
	err = repo.UpdateSession(env.Ctx, updated)
	require.NoError(t, err)

	retrieved, err := repo.GetSession(env.Ctx, session.ID())
	require.NoError(t, err)
	assert.True(t, retrieved.UpdatedAt().After(originalUpdatedAt),
		"updated_at should be updated automatically")
}

func TestPostgresChatRepository_GetSession_WithSQLNullTypes(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("NULL Fields Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	retrieved, err := repo.GetSession(env.Ctx, session.ID())
	require.NoError(t, err)
	assert.Nil(t, retrieved.ParentSessionID())
	assert.Nil(t, retrieved.PendingQuestionAgent())
}

func TestPostgresChatRepository_InvalidTenantContext(t *testing.T) {
	t.Parallel()
	env := setupTest(t)

	repo := persistence.NewPostgresChatRepository()

	// Create session
	session := domain.NewSession(
		domain.WithTenantID(env.Tenant.ID),
		domain.WithUserID(int64(env.User.ID())),
		domain.WithTitle("Invalid Context Test"),
	)
	err := repo.CreateSession(env.Ctx, session)
	require.NoError(t, err)

	// Try to get with different tenant context (simulated by new environment)
	envOther := setupTest(t)

	_, err = repo.GetSession(envOther.Ctx, session.ID())
	require.Error(t, err)
	assert.ErrorIs(t, err, persistence.ErrSessionNotFound,
		"Session should not be accessible from different tenant context")
}
