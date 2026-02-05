package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
)

func TestSession_Creation(t *testing.T) {
	t.Parallel()

	t.Run("basic creation with defaults", func(t *testing.T) {
		session := domain.NewSession()

		require.NotEqual(t, uuid.Nil, session.ID(), "Expected non-nil UUID")
		require.Equal(t, domain.SessionStatusActive, session.Status(), "Expected status Active")
		require.False(t, session.Pinned(), "Expected Pinned to be false by default")
		require.False(t, session.CreatedAt().IsZero(), "Expected CreatedAt to be set")
		require.False(t, session.UpdatedAt().IsZero(), "Expected UpdatedAt to be set")
	})

	t.Run("creation with options", func(t *testing.T) {
		tenantID := uuid.New()
		userID := int64(123)
		title := "Test Session"
		parentID := uuid.New()

		session := domain.NewSession(
			domain.WithTenantID(tenantID),
			domain.WithUserID(userID),
			domain.WithTitle(title),
			domain.WithPinned(true),
			domain.WithParentSessionID(parentID),
			domain.WithStatus(domain.SessionStatusArchived),
		)

		assert.Equal(t, tenantID, session.TenantID(), "TenantID mismatch")
		assert.Equal(t, userID, session.UserID(), "UserID mismatch")
		assert.Equal(t, title, session.Title(), "Title mismatch")
		assert.True(t, session.Pinned(), "Expected Pinned to be true")
		require.NotNil(t, session.ParentSessionID(), "Expected ParentSessionID to be set")
		assert.Equal(t, parentID, *session.ParentSessionID(), "ParentSessionID mismatch")
		assert.Equal(t, domain.SessionStatusArchived, session.Status(), "Status mismatch")
	})

	t.Run("creation with custom ID", func(t *testing.T) {
		customID := uuid.New()
		session := domain.NewSession(domain.WithID(customID))

		assert.Equal(t, customID, session.ID(), "ID mismatch")
	})

	t.Run("creation with pending question agent", func(t *testing.T) {
		agentName := "test-agent"
		session := domain.NewSession(
			domain.WithPendingQuestionAgent(agentName),
		)

		require.NotNil(t, session.PendingQuestionAgent(), "Expected PendingQuestionAgent to be set")
		assert.Equal(t, agentName, *session.PendingQuestionAgent(), "Agent name mismatch")
	})
}

func TestSession_Validation(t *testing.T) {
	t.Parallel()

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(123),
		domain.WithTitle("Test Session"),
	)

	assert.NotEqual(t, uuid.Nil, session.TenantID(), "Expected non-nil TenantID")
	assert.NotZero(t, session.UserID(), "Expected non-zero UserID")
	assert.NotEmpty(t, session.Title(), "Expected non-empty Title")
}

func TestSessionStatus_Values(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status   domain.SessionStatus
		expected string
	}{
		{domain.SessionStatusActive, "ACTIVE"},
		{domain.SessionStatusArchived, "ARCHIVED"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.status.String(), "Status string mismatch for %s", tt.status)
	}
}

func TestSessionStatus_IsActive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status   domain.SessionStatus
		expected bool
	}{
		{domain.SessionStatusActive, true},
		{domain.SessionStatusArchived, false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.status.IsActive(), "IsActive() mismatch for %s", tt.status)
	}
}

func TestSessionStatus_IsArchived(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status   domain.SessionStatus
		expected bool
	}{
		{domain.SessionStatusActive, false},
		{domain.SessionStatusArchived, true},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.status.IsArchived(), "IsArchived() mismatch for %s", tt.status)
	}
}

func TestSessionStatus_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status   domain.SessionStatus
		expected bool
	}{
		{domain.SessionStatusActive, true},
		{domain.SessionStatusArchived, true},
		{domain.SessionStatus("invalid"), false},
		{domain.SessionStatus(""), false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.status.Valid(), "Valid() mismatch for '%s'", tt.status)
	}
}

func TestSession_IsActive(t *testing.T) {
	t.Parallel()

	activeSession := domain.NewSession(domain.WithStatus(domain.SessionStatusActive))
	archivedSession := domain.NewSession(domain.WithStatus(domain.SessionStatusArchived))

	assert.True(t, activeSession.IsActive(), "Expected active session to return true for IsActive()")
	assert.False(t, archivedSession.IsActive(), "Expected archived session to return false for IsActive()")
}

func TestSession_IsArchived(t *testing.T) {
	t.Parallel()

	activeSession := domain.NewSession(domain.WithStatus(domain.SessionStatusActive))
	archivedSession := domain.NewSession(domain.WithStatus(domain.SessionStatusArchived))

	assert.False(t, activeSession.IsArchived(), "Expected active session to return false for IsArchived()")
	assert.True(t, archivedSession.IsArchived(), "Expected archived session to return true for IsArchived()")
}

func TestSession_IsPinned(t *testing.T) {
	t.Parallel()

	pinnedSession := domain.NewSession(domain.WithPinned(true))
	unpinnedSession := domain.NewSession(domain.WithPinned(false))

	assert.True(t, pinnedSession.IsPinned(), "Expected pinned session to return true for IsPinned()")
	assert.False(t, unpinnedSession.IsPinned(), "Expected unpinned session to return false for IsPinned()")
}

func TestSession_HasParent(t *testing.T) {
	t.Parallel()

	parentID := uuid.New()
	sessionWithParent := domain.NewSession(domain.WithParentSessionID(parentID))
	sessionWithoutParent := domain.NewSession()

	assert.True(t, sessionWithParent.HasParent(), "Expected session with parent to return true for HasParent()")
	assert.False(t, sessionWithoutParent.HasParent(), "Expected session without parent to return false for HasParent()")
}

func TestSession_HasPendingQuestion(t *testing.T) {
	t.Parallel()

	sessionWithQuestion := domain.NewSession(domain.WithPendingQuestionAgent("agent"))
	sessionWithoutQuestion := domain.NewSession()

	assert.True(t, sessionWithQuestion.HasPendingQuestion(), "Expected session with pending question to return true")
	assert.False(t, sessionWithoutQuestion.HasPendingQuestion(), "Expected session without pending question to return false")
}

func TestSession_MultipleOptions(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	userID := int64(456)
	title := "Complex Session"
	parentID := uuid.New()
	agent := "test-agent"

	session := domain.NewSession(
		domain.WithTenantID(tenantID),
		domain.WithUserID(userID),
		domain.WithTitle(title),
		domain.WithPinned(true),
		domain.WithParentSessionID(parentID),
		domain.WithPendingQuestionAgent(agent),
		domain.WithStatus(domain.SessionStatusArchived),
	)

	assert.Equal(t, tenantID, session.TenantID(), "TenantID not set correctly")
	assert.Equal(t, userID, session.UserID(), "UserID not set correctly")
	assert.Equal(t, title, session.Title(), "Title not set correctly")
	assert.True(t, session.Pinned(), "Pinned not set correctly")
	require.True(t, session.HasParent(), "ParentSessionID not set correctly")
	assert.Equal(t, parentID, *session.ParentSessionID(), "ParentSessionID value mismatch")
	require.True(t, session.HasPendingQuestion(), "PendingQuestionAgent not set correctly")
	assert.Equal(t, agent, *session.PendingQuestionAgent(), "PendingQuestionAgent value mismatch")
	assert.Equal(t, domain.SessionStatusArchived, session.Status(), "Status not set correctly")
}
