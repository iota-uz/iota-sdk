package services

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionTitleService_NoMessagesUsesDeterministicFallback(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	model := newMockModel()
	service, err := NewSessionTitleService(model, chatRepo, nil)
	require.NoError(t, err)

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle(""),
	)
	require.NoError(t, chatRepo.CreateSession(context.Background(), session))

	err = service.GenerateSessionTitle(context.Background(), session.ID())
	require.NoError(t, err)

	updated, getErr := chatRepo.GetSession(context.Background(), session.ID())
	require.NoError(t, getErr)
	assert.Equal(t, "Untitled Chat", updated.Title())
}

func TestSessionTitleService_ModelFailureFallsBackToExtractedTitle(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	model := newMockModel()
	model.err = assert.AnError
	service, err := NewSessionTitleService(model, chatRepo, nil)
	require.NoError(t, err)

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle("   "),
	)
	require.NoError(t, chatRepo.CreateSession(context.Background(), session))
	require.NoError(t, chatRepo.SaveMessage(context.Background(), types.UserMessage("monthly revenue by region", types.WithSessionID(session.ID()))))

	err = service.GenerateSessionTitle(context.Background(), session.ID())
	require.NoError(t, err)

	updated, getErr := chatRepo.GetSession(context.Background(), session.ID())
	require.NoError(t, getErr)
	assert.NotEmpty(t, updated.Title())
	assert.NotEqual(t, "Untitled Chat", updated.Title())
}

func TestSessionTitleService_NoUserMessageFallsBackToUntitled(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	model := newMockModel()
	service, err := NewSessionTitleService(model, chatRepo, nil)
	require.NoError(t, err)

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle(""),
	)
	require.NoError(t, chatRepo.CreateSession(context.Background(), session))
	require.NoError(t, chatRepo.SaveMessage(context.Background(), types.AssistantMessage("hello", types.WithSessionID(session.ID()))))

	err = service.GenerateSessionTitle(context.Background(), session.ID())
	require.NoError(t, err)

	updated, getErr := chatRepo.GetSession(context.Background(), session.ID())
	require.NoError(t, getErr)
	assert.Equal(t, "Untitled Chat", updated.Title())
}
