package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/assert"
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

func TestChatService_ClearSessionHistory(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	svc := NewChatService(chatRepo, nil, nil, nil)

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle("keep me"),
		domain.WithPinned(true),
		domain.WithPendingQuestionAgent("sql_agent"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	require.NoError(t, chatRepo.SaveMessage(t.Context(), types.UserMessage("hello", types.WithSessionID(session.ID()))))
	require.NoError(t, chatRepo.SaveMessage(t.Context(), types.AssistantMessage("world", types.WithSessionID(session.ID()))))

	artifactOne := domain.NewArtifact(
		domain.WithArtifactTenantID(session.TenantID()),
		domain.WithArtifactSessionID(session.ID()),
		domain.WithArtifactName("one"),
	)
	artifactTwo := domain.NewArtifact(
		domain.WithArtifactTenantID(session.TenantID()),
		domain.WithArtifactSessionID(session.ID()),
		domain.WithArtifactName("two"),
	)
	require.NoError(t, chatRepo.SaveArtifact(t.Context(), artifactOne))
	require.NoError(t, chatRepo.SaveArtifact(t.Context(), artifactTwo))

	result, err := svc.ClearSessionHistory(t.Context(), session.ID())
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, int64(2), result.DeletedMessages)
	assert.Equal(t, int64(2), result.DeletedArtifacts)

	updatedSession, err := chatRepo.GetSession(t.Context(), session.ID())
	require.NoError(t, err)
	assert.Equal(t, session.Title(), updatedSession.Title())
	assert.Equal(t, session.Pinned(), updatedSession.Pinned())
	assert.Nil(t, updatedSession.PendingQuestionAgent())

	messages, err := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, messages)

	artifacts, err := chatRepo.GetSessionArtifacts(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	assert.Empty(t, artifacts)
}

func TestChatService_CompactSessionHistory(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	model := newMockModel()
	model.response.Message = types.AssistantMessage("## Conversation Summary\nCompacted response")
	svc := NewChatService(chatRepo, nil, model, nil)

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle("to compact"),
		domain.WithPendingQuestionAgent("sql_agent"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	require.NoError(t, chatRepo.SaveMessage(t.Context(), types.UserMessage("question", types.WithSessionID(session.ID()))))
	require.NoError(t, chatRepo.SaveMessage(t.Context(), types.AssistantMessage("answer", types.WithSessionID(session.ID()))))

	artifact := domain.NewArtifact(
		domain.WithArtifactTenantID(session.TenantID()),
		domain.WithArtifactSessionID(session.ID()),
		domain.WithArtifactName("report"),
	)
	require.NoError(t, chatRepo.SaveArtifact(t.Context(), artifact))

	result, err := svc.CompactSessionHistory(t.Context(), session.ID())
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, int64(2), result.DeletedMessages)
	assert.Equal(t, int64(1), result.DeletedArtifacts)
	assert.NotEmpty(t, result.Summary)

	messages, err := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.Equal(t, types.RoleUser, messages[0].Role())
	assert.Equal(t, "/compact", messages[0].Content())
	assert.Equal(t, types.RoleAssistant, messages[1].Role())
	assert.Equal(t, result.Summary, messages[1].Content())

	updatedSession, err := chatRepo.GetSession(t.Context(), session.ID())
	require.NoError(t, err)
	assert.Nil(t, updatedSession.PendingQuestionAgent())
}

func TestChatService_CompactSessionHistory_EmptyHistory(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	model := newMockModel()
	svc := NewChatService(chatRepo, nil, model, nil)

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle("empty"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	result, err := svc.CompactSessionHistory(t.Context(), session.ID())
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, int64(0), result.DeletedMessages)
	assert.Equal(t, int64(0), result.DeletedArtifacts)
	assert.NotEmpty(t, result.Summary)

	messages, err := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.Equal(t, "/compact", messages[0].Content())
	assert.Equal(t, result.Summary, messages[1].Content())
}

type captureTitleContextService struct {
	called chan context.Context
}

func (s *captureTitleContextService) GenerateSessionTitle(ctx context.Context, _ uuid.UUID) error {
	select {
	case s.called <- ctx:
	default:
	}
	return nil
}

func TestChatService_MaybeGenerateTitleAsync_PreservesTenantContext(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	titleService := &captureTitleContextService{
		called: make(chan context.Context, 1),
	}
	svc := &chatServiceImpl{
		titleService: titleService,
	}

	reqCtx := composables.WithTenantID(context.Background(), tenantID)
	svc.maybeGenerateTitleAsync(reqCtx, uuid.New())

	select {
	case titleCtx := <-titleService.called:
		gotTenantID, err := composables.UseTenantID(titleCtx)
		require.NoError(t, err)
		assert.Equal(t, tenantID, gotTenantID)
	case <-time.After(2 * time.Second):
		t.Fatal("expected async title generation to be invoked")
	}
}
