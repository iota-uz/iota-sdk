package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/stretchr/testify/require"
)

func TestChatService_UnarchiveSession(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	svc := NewChatService(chatRepo, nil, nil, nil)

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle("t"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	archived, err := svc.ArchiveSession(t.Context(), session.ID())
	require.NoError(t, err)
	require.Equal(t, domain.SessionStatusArchived, archived.Status())

	active, err := svc.UnarchiveSession(t.Context(), session.ID())
	require.NoError(t, err)
	require.Equal(t, domain.SessionStatusActive, active.Status())

	stored, err := chatRepo.GetSession(t.Context(), session.ID())
	require.NoError(t, err)
	require.Equal(t, domain.SessionStatusActive, stored.Status())
}
