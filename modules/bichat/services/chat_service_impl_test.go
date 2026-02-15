package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errStubQueryNotImplemented = errors.New("stub query not implemented")

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
		domain.WithLLMPreviousResponseID("resp_prev_clear"),
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
	assert.Nil(t, updatedSession.LLMPreviousResponseID())

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
		domain.WithLLMPreviousResponseID("resp_prev_compact"),
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
	require.Len(t, messages, 1)
	assert.Equal(t, types.RoleSystem, messages[0].Role())
	assert.Equal(t, result.Summary, messages[0].Content())

	updatedSession, err := chatRepo.GetSession(t.Context(), session.ID())
	require.NoError(t, err)
	assert.Nil(t, updatedSession.LLMPreviousResponseID())
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
	require.Len(t, messages, 1)
	assert.Equal(t, types.RoleSystem, messages[0].Role())
	assert.Equal(t, result.Summary, messages[0].Content())
}

func TestChatService_MaybeReplaceHistoryFromMessage_TruncatesFromUserMessage(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	svc := NewChatService(chatRepo, nil, nil, nil).(*chatServiceImpl)

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle("replace"),
		domain.WithLLMPreviousResponseID("resp_prev_replace"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	base := time.Now().Add(-5 * time.Minute)
	userOne := types.UserMessage(
		"first",
		types.WithSessionID(session.ID()),
		types.WithCreatedAt(base),
	)
	assistantOne := types.AssistantMessage(
		"first response",
		types.WithSessionID(session.ID()),
		types.WithCreatedAt(base.Add(time.Second)),
	)
	userTwo := types.UserMessage(
		"second",
		types.WithSessionID(session.ID()),
		types.WithCreatedAt(base.Add(2*time.Second)),
	)
	assistantTwo := types.AssistantMessage(
		"second response",
		types.WithSessionID(session.ID()),
		types.WithCreatedAt(base.Add(3*time.Second)),
	)
	require.NoError(t, chatRepo.SaveMessage(t.Context(), userOne))
	require.NoError(t, chatRepo.SaveMessage(t.Context(), assistantOne))
	require.NoError(t, chatRepo.SaveMessage(t.Context(), userTwo))
	require.NoError(t, chatRepo.SaveMessage(t.Context(), assistantTwo))

	replaceFromID := userTwo.ID()
	updated, err := svc.maybeReplaceHistoryFromMessage(t.Context(), session, &replaceFromID)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Nil(t, updated.LLMPreviousResponseID())

	messages, err := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	require.Len(t, messages, 2)
	assert.Equal(t, userOne.ID(), messages[0].ID())
	assert.Equal(t, assistantOne.ID(), messages[1].ID())
}

func TestChatService_MaybeReplaceHistoryFromMessage_RejectsNonUserMessage(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	svc := NewChatService(chatRepo, nil, nil, nil).(*chatServiceImpl)

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle("replace"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	assistant := types.AssistantMessage("answer", types.WithSessionID(session.ID()))
	require.NoError(t, chatRepo.SaveMessage(t.Context(), assistant))

	replaceFromID := assistant.ID()
	_, err := svc.maybeReplaceHistoryFromMessage(t.Context(), session, &replaceFromID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "replaceFromMessageId must point to a user message")
}

func TestChatService_ResumeWithAnswer_InterruptPersistsPendingState(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle("resume interrupt"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	// Save an assistant message with pending question data (simulates initial interrupt)
	qd, err := types.NewQuestionData("cp-prev", "bi_agent", []types.QuestionDataItem{
		{
			ID:   "metric",
			Text: "Choose metric",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "rev", Label: "Revenue"},
				{ID: "exp", Label: "Expense"},
			},
		},
	})
	require.NoError(t, err)
	pendingMsg := types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole("assistant"),
		types.WithContent("I need more information."),
		types.WithQuestionData(qd),
	)
	require.NoError(t, chatRepo.SaveMessage(t.Context(), pendingMsg))

	agentSvc := &stubAgentService{
		resumeEvents: []bichatservices.Event{
			{
				Type: bichatservices.EventTypeInterrupt,
				Interrupt: &bichatservices.InterruptEvent{
					CheckpointID:       "cp-next",
					AgentName:          "bi_agent",
					ProviderResponseID: "resp-next",
					Questions: []bichatservices.Question{
						{
							ID:   "metric",
							Text: "Choose metric",
							Type: bichatservices.QuestionTypeSingleChoice,
							Options: []bichatservices.QuestionOption{
								{ID: "rev", Label: "Revenue"},
								{ID: "exp", Label: "Expense"},
							},
						},
					},
				},
			},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, nil)
	resp, err := svc.ResumeWithAnswer(t.Context(), bichatservices.ResumeRequest{
		SessionID:    session.ID(),
		CheckpointID: "cp-prev",
		Answers: map[string]string{
			"metric": "rev",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.AssistantMessage, "resume always saves an assistant continuation message")
	require.NotNil(t, resp.Interrupt)
	require.Equal(t, "cp-next", resp.Interrupt.CheckpointID)

	updatedSession, err := chatRepo.GetSession(t.Context(), session.ID())
	require.NoError(t, err)
	require.NotNil(t, updatedSession.LLMPreviousResponseID())
	assert.Equal(t, "resp-next", *updatedSession.LLMPreviousResponseID())

	messages, err := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	// Original pending message + new continuation message
	assert.Len(t, messages, 2)
}

type captureTitleContextService struct {
	called chan context.Context
}

type stubRepoTx struct{}

type stubAgentService struct {
	resumeEvents []bichatservices.Event
}

func (s *stubAgentService) ProcessMessage(ctx context.Context, sessionID uuid.UUID, content string, attachments []domain.Attachment) (types.Generator[bichatservices.Event], error) {
	return nil, assert.AnError
}

func (s *stubAgentService) ResumeWithAnswer(ctx context.Context, sessionID uuid.UUID, checkpointID string, answers map[string]types.Answer) (types.Generator[bichatservices.Event], error) {
	events := append([]bichatservices.Event{}, s.resumeEvents...)
	return types.NewGenerator(ctx, func(ctx context.Context, yield func(bichatservices.Event) bool) error {
		for _, ev := range events {
			if !yield(ev) {
				return nil
			}
		}
		return nil
	}), nil
}

func (s *captureTitleContextService) GenerateSessionTitle(ctx context.Context, _ uuid.UUID) error {
	select {
	case s.called <- ctx:
	default:
	}
	return nil
}

func (stubRepoTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

func (stubRepoTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults {
	return nil
}

func (stubRepoTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	var tag pgconn.CommandTag
	return tag, nil
}

func (stubRepoTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errStubQueryNotImplemented
}

func (stubRepoTx) QueryRow(context.Context, string, ...any) pgx.Row {
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

func TestBuildTitleGenerationContext_PreservesDatabaseExecutionContext(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	tx := stubRepoTx{}

	reqCtx := composables.WithTenantID(context.Background(), tenantID)
	reqCtx = context.WithValue(reqCtx, constants.TxKey, tx)

	titleCtx := buildTitleGenerationContext(reqCtx)

	gotTenantID, err := composables.UseTenantID(titleCtx)
	require.NoError(t, err)
	assert.Equal(t, tenantID, gotTenantID)

	gotTx, err := composables.UseTx(titleCtx)
	require.NoError(t, err)
	assert.Equal(t, tx, gotTx)

	_, err = composables.UsePool(titleCtx)
	require.ErrorIs(t, err, composables.ErrNoPool)
}
