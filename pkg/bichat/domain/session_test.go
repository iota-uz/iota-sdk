package domain_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
)

func TestSession_Creation(t *testing.T) {
	t.Parallel()

	t.Run("basic creation with defaults", func(t *testing.T) {
		session := domain.NewSession()

		if session.ID() == uuid.Nil {
			t.Error("Expected non-nil UUID")
		}
		if session.Status() != domain.SessionStatusActive {
			t.Errorf("Expected status Active, got %s", session.Status())
		}
		if session.Pinned() {
			t.Error("Expected Pinned to be false by default")
		}
		if session.CreatedAt().IsZero() {
			t.Error("Expected CreatedAt to be set")
		}
		if session.UpdatedAt().IsZero() {
			t.Error("Expected UpdatedAt to be set")
		}
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

		if session.TenantID() != tenantID {
			t.Errorf("Expected TenantID %s, got %s", tenantID, session.TenantID())
		}
		if session.UserID() != userID {
			t.Errorf("Expected UserID %d, got %d", userID, session.UserID())
		}
		if session.Title() != title {
			t.Errorf("Expected Title '%s', got '%s'", title, session.Title())
		}
		if !session.Pinned() {
			t.Error("Expected Pinned to be true")
		}
		if session.ParentSessionID() == nil || *session.ParentSessionID() != parentID {
			t.Error("Expected ParentSessionID to be set")
		}
		if session.Status() != domain.SessionStatusArchived {
			t.Errorf("Expected status Archived, got %s", session.Status())
		}
	})

	t.Run("creation with custom ID", func(t *testing.T) {
		customID := uuid.New()
		session := domain.NewSession(domain.WithID(customID))

		if session.ID() != customID {
			t.Errorf("Expected ID %s, got %s", customID, session.ID())
		}
	})

	t.Run("creation with pending question agent", func(t *testing.T) {
		agentName := "test-agent"
		session := domain.NewSession(
			domain.WithPendingQuestionAgent(agentName),
		)

		if session.PendingQuestionAgent() == nil {
			t.Fatal("Expected PendingQuestionAgent to be set")
		}
		if *session.PendingQuestionAgent() != agentName {
			t.Errorf("Expected agent '%s', got '%s'", agentName, *session.PendingQuestionAgent())
		}
	})
}

func TestSession_Validation(t *testing.T) {
	t.Parallel()

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(123),
		domain.WithTitle("Test Session"),
	)

	if session.TenantID() == uuid.Nil {
		t.Error("Expected non-nil TenantID")
	}
	if session.UserID() == 0 {
		t.Error("Expected non-zero UserID")
	}
	if session.Title() == "" {
		t.Error("Expected non-empty Title")
	}
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
		if tt.status.String() != tt.expected {
			t.Errorf("Expected status string '%s', got '%s'", tt.expected, tt.status.String())
		}
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
		if tt.status.IsActive() != tt.expected {
			t.Errorf("IsActive() for %s: expected %v, got %v", tt.status, tt.expected, tt.status.IsActive())
		}
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
		if tt.status.IsArchived() != tt.expected {
			t.Errorf("IsArchived() for %s: expected %v, got %v", tt.status, tt.expected, tt.status.IsArchived())
		}
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
		if tt.status.Valid() != tt.expected {
			t.Errorf("Valid() for '%s': expected %v, got %v", tt.status, tt.expected, tt.status.Valid())
		}
	}
}

func TestSession_IsActive(t *testing.T) {
	t.Parallel()

	activeSession := domain.NewSession(domain.WithStatus(domain.SessionStatusActive))
	archivedSession := domain.NewSession(domain.WithStatus(domain.SessionStatusArchived))

	if !activeSession.IsActive() {
		t.Error("Expected active session to return true for IsActive()")
	}
	if archivedSession.IsActive() {
		t.Error("Expected archived session to return false for IsActive()")
	}
}

func TestSession_IsArchived(t *testing.T) {
	t.Parallel()

	activeSession := domain.NewSession(domain.WithStatus(domain.SessionStatusActive))
	archivedSession := domain.NewSession(domain.WithStatus(domain.SessionStatusArchived))

	if activeSession.IsArchived() {
		t.Error("Expected active session to return false for IsArchived()")
	}
	if !archivedSession.IsArchived() {
		t.Error("Expected archived session to return true for IsArchived()")
	}
}

func TestSession_IsPinned(t *testing.T) {
	t.Parallel()

	pinnedSession := domain.NewSession(domain.WithPinned(true))
	unpinnedSession := domain.NewSession(domain.WithPinned(false))

	if !pinnedSession.IsPinned() {
		t.Error("Expected pinned session to return true for IsPinned()")
	}
	if unpinnedSession.IsPinned() {
		t.Error("Expected unpinned session to return false for IsPinned()")
	}
}

func TestSession_HasParent(t *testing.T) {
	t.Parallel()

	parentID := uuid.New()
	sessionWithParent := domain.NewSession(domain.WithParentSessionID(parentID))
	sessionWithoutParent := domain.NewSession()

	if !sessionWithParent.HasParent() {
		t.Error("Expected session with parent to return true for HasParent()")
	}
	if sessionWithoutParent.HasParent() {
		t.Error("Expected session without parent to return false for HasParent()")
	}
}

func TestSession_HasPendingQuestion(t *testing.T) {
	t.Parallel()

	sessionWithQuestion := domain.NewSession(domain.WithPendingQuestionAgent("agent"))
	sessionWithoutQuestion := domain.NewSession()

	if !sessionWithQuestion.HasPendingQuestion() {
		t.Error("Expected session with pending question to return true")
	}
	if sessionWithoutQuestion.HasPendingQuestion() {
		t.Error("Expected session without pending question to return false")
	}
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

	if session.TenantID() != tenantID {
		t.Error("TenantID not set correctly")
	}
	if session.UserID() != userID {
		t.Error("UserID not set correctly")
	}
	if session.Title() != title {
		t.Error("Title not set correctly")
	}
	if !session.Pinned() {
		t.Error("Pinned not set correctly")
	}
	if !session.HasParent() || *session.ParentSessionID() != parentID {
		t.Error("ParentSessionID not set correctly")
	}
	if !session.HasPendingQuestion() || *session.PendingQuestionAgent() != agent {
		t.Error("PendingQuestionAgent not set correctly")
	}
	if session.Status() != domain.SessionStatusArchived {
		t.Error("Status not set correctly")
	}
}
