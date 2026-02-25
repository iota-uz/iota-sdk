package services

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/stretchr/testify/require"
)

type sessionOption func(*sessionFixture)

type sessionFixture struct {
	id                    uuid.UUID
	tenantID              uuid.UUID
	userID                int64
	title                 string
	titleSet              bool
	status                domain.SessionStatus
	statusSet             bool
	pinned                bool
	pinnedSet             bool
	parentSessionID       *uuid.UUID
	llmPreviousResponseID *string
	createdAt             time.Time
	updatedAt             time.Time
}

func mustSession(t *testing.T, opts ...sessionOption) domain.Session {
	t.Helper()

	f := sessionFixture{
		status: domain.SessionStatusActive,
		title:  "Session",
	}
	for _, opt := range opts {
		opt(&f)
	}

	if f.tenantID == uuid.Nil {
		f.tenantID = uuid.New()
	}
	if f.userID <= 0 {
		f.userID = 1
	}

	if !f.titleSet {
		f.title = "Session"
	}

	needsRehydrate := f.statusSet || f.pinnedSet
	if !needsRehydrate {
		spec := domain.SessionSpec{
			ID:                    f.id,
			TenantID:              f.tenantID,
			OwnerUserID:           f.userID,
			Title:                 f.title,
			ParentSessionID:       f.parentSessionID,
			LLMPreviousResponseID: f.llmPreviousResponseID,
			CreatedAt:             f.createdAt,
			UpdatedAt:             f.updatedAt,
		}
		var (
			session domain.Session
			err     error
		)
		if strings.TrimSpace(f.title) == "" {
			session, err = domain.NewUntitledSession(spec)
		} else {
			session, err = domain.NewSession(spec)
		}
		require.NoError(t, err)
		return session
	}

	id := f.id
	if id == uuid.Nil {
		id = uuid.New()
	}
	state := domain.SessionState{
		ID:                    id,
		TenantID:              f.tenantID,
		OwnerUserID:           f.userID,
		Title:                 f.title,
		Status:                f.status,
		Pinned:                f.pinned,
		ParentSessionID:       f.parentSessionID,
		LLMPreviousResponseID: f.llmPreviousResponseID,
		CreatedAt:             f.createdAt,
		UpdatedAt:             f.updatedAt,
	}
	session, err := domain.RehydrateSession(state)
	require.NoError(t, err)
	return session
}

func withSessionID(id uuid.UUID) sessionOption {
	return func(f *sessionFixture) { f.id = id }
}

func withSessionTenantID(tenantID uuid.UUID) sessionOption {
	return func(f *sessionFixture) { f.tenantID = tenantID }
}

func withSessionUserID(userID int64) sessionOption {
	return func(f *sessionFixture) { f.userID = userID }
}

func withSessionTitle(title string) sessionOption {
	return func(f *sessionFixture) {
		f.title = title
		f.titleSet = true
	}
}

func withSessionPinned(pinned bool) sessionOption {
	return func(f *sessionFixture) {
		f.pinned = pinned
		f.pinnedSet = true
	}
}

func withSessionLLMPreviousResponseID(responseID string) sessionOption {
	return func(f *sessionFixture) {
		id := responseID
		f.llmPreviousResponseID = &id
	}
}
