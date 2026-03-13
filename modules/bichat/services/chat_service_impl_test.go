package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustQuestionData(t *testing.T, checkpointID string) *types.QuestionData {
	t.Helper()

	qd, err := types.NewQuestionData(checkpointID, "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	return qd
}

func TestChatService_UnarchiveSession(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	svc := NewChatService(chatRepo, nil, nil, nil, nil)

	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("t"),
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
	svc := NewChatService(chatRepo, nil, nil, nil, nil)

	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("keep me"),
		withSessionPinned(true),
		withSessionLLMPreviousResponseID("resp_prev_clear"),
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
	svc := NewChatService(chatRepo, nil, model, nil, nil)

	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("to compact"),
		withSessionLLMPreviousResponseID("resp_prev_compact"),
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
	svc := NewChatService(chatRepo, nil, model, nil, nil)

	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("empty"),
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
	svc := NewChatService(chatRepo, nil, nil, nil, nil)

	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("replace"),
		withSessionLLMPreviousResponseID("resp_prev_replace"),
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
	svc := NewChatService(chatRepo, nil, nil, nil, nil)

	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("replace"),
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
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("resume interrupt"),
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
		resumeEvents: []agents.ExecutorEvent{
			{
				Type: agents.EventTypeInterrupt,
				ParsedInterrupt: &agents.ParsedInterrupt{
					CheckpointID:       "cp-next",
					AgentName:          "bi_agent",
					ProviderResponseID: "resp-next",
					Questions: []agents.Question{
						{
							ID:   "metric",
							Text: "Choose metric",
							Type: agents.QuestionTypeSingleChoice,
							Options: []agents.QuestionOption{
								{ID: "rev", Label: "Revenue"},
								{ID: "exp", Label: "Expense"},
							},
						},
					},
				},
			},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)
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

func TestChatService_ResumeWithAnswer_UsesCanonicalCheckpointAndNormalizesLabels(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("resume canonical checkpoint"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	qd, err := types.NewQuestionData("cp-canonical", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	pendingMsg := types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)
	require.NoError(t, chatRepo.SaveMessage(t.Context(), pendingMsg))

	agentSvc := &stubAgentService{
		resumeEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeDone},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	resp, err := svc.ResumeWithAnswer(t.Context(), bichatservices.ResumeRequest{
		SessionID:    session.ID(),
		CheckpointID: "cp-stale-from-client",
		Answers: map[string]string{
			"scope": "All policies",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "cp-canonical", agentSvc.resumeCheckpoint)
	require.Contains(t, agentSvc.resumeAnswers, "scope")
	assert.Equal(t, "all", agentSvc.resumeAnswers["scope"].String())

	messages, err := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, messages)
	updatedQuestionData := messages[0].QuestionData()
	require.NotNil(t, updatedQuestionData)
	assert.Equal(t, types.QuestionStatusAnswered, updatedQuestionData.Status)
	assert.Equal(t, "all", updatedQuestionData.Answers["scope"])
}

func TestChatService_ResumeWithAnswer_CheckpointNotFoundFinalizesAnswered(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("resume stale checkpoint"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	qd, err := types.NewQuestionData("cp-missing", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	pendingMsg := types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)
	require.NoError(t, chatRepo.SaveMessage(t.Context(), pendingMsg))

	agentSvc := &stubAgentService{
		resumeErr: agents.ErrCheckpointNotFound,
	}

	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	resp, err := svc.ResumeWithAnswer(t.Context(), bichatservices.ResumeRequest{
		SessionID:    session.ID(),
		CheckpointID: "cp-missing",
		Answers: map[string]string{
			"scope": "all",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, resp.AssistantMessage)

	messages, err := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, messages)
	updatedQuestionData := messages[0].QuestionData()
	require.NotNil(t, updatedQuestionData)
	assert.Equal(t, types.QuestionStatusAnswered, updatedQuestionData.Status)
	assert.Equal(t, "all", updatedQuestionData.Answers["scope"])
}

func TestChatService_RejectPendingQuestion_CheckpointNotFoundFinalizesRejected(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("reject stale checkpoint"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	qd, err := types.NewQuestionData("cp-missing", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	pendingMsg := types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)
	require.NoError(t, chatRepo.SaveMessage(t.Context(), pendingMsg))

	agentSvc := &stubAgentService{
		resumeErr: agents.ErrCheckpointNotFound,
	}

	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	resp, err := svc.RejectPendingQuestion(t.Context(), session.ID())
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Nil(t, resp.AssistantMessage)

	messages, err := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, messages)
	updatedQuestionData := messages[0].QuestionData()
	require.NotNil(t, updatedQuestionData)
	assert.Equal(t, types.QuestionStatusRejected, updatedQuestionData.Status)
	assert.False(t, messages[0].HasPendingQuestion())
}

func TestChatService_HITLDeferredCheckpointNotFoundFinalizesTerminalState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		sessionTitle   string
		checkpointID   string
		resumeRequest  *bichatservices.ResumeRequest
		expectStatus   types.QuestionStatus
		expectAnswers  map[string]string
		invoke         func(t *testing.T, svc *chatServiceImpl, sessionID uuid.UUID) *bichatservices.SendMessageResponse
		assertResponse bool
	}{
		{
			name:         "resume sync",
			sessionTitle: "resume deferred stale checkpoint",
			checkpointID: "cp-missing-answer-sync",
			resumeRequest: &bichatservices.ResumeRequest{
				CheckpointID: "cp-missing-answer-sync",
				Answers: map[string]string{
					"scope": "all",
				},
			},
			expectStatus:   types.QuestionStatusAnswered,
			expectAnswers:  map[string]string{"scope": "all"},
			assertResponse: true,
			invoke: func(t *testing.T, svc *chatServiceImpl, sessionID uuid.UUID) *bichatservices.SendMessageResponse {
				t.Helper()
				resp, err := svc.ResumeWithAnswer(t.Context(), bichatservices.ResumeRequest{
					SessionID:    sessionID,
					CheckpointID: "cp-missing-answer-sync",
					Answers: map[string]string{
						"scope": "all",
					},
				})
				require.NoError(t, err)
				return resp
			},
		},
		{
			name:           "reject sync",
			sessionTitle:   "reject deferred stale checkpoint",
			checkpointID:   "cp-missing-reject-sync",
			expectStatus:   types.QuestionStatusRejected,
			assertResponse: true,
			invoke: func(t *testing.T, svc *chatServiceImpl, sessionID uuid.UUID) *bichatservices.SendMessageResponse {
				t.Helper()
				resp, err := svc.RejectPendingQuestion(t.Context(), sessionID)
				require.NoError(t, err)
				return resp
			},
		},
		{
			name:         "resume async",
			sessionTitle: "resume async deferred stale checkpoint",
			checkpointID: "cp-missing-answer-async",
			expectStatus: types.QuestionStatusAnswered,
			expectAnswers: map[string]string{
				"scope": "all",
			},
			invoke: func(t *testing.T, svc *chatServiceImpl, sessionID uuid.UUID) *bichatservices.SendMessageResponse {
				t.Helper()
				_, err := svc.ResumeWithAnswerAsync(t.Context(), bichatservices.ResumeRequest{
					SessionID:    sessionID,
					CheckpointID: "cp-missing-answer-async",
					Answers: map[string]string{
						"scope": "all",
					},
				})
				require.NoError(t, err)
				return nil
			},
		},
		{
			name:         "reject async",
			sessionTitle: "reject async deferred stale checkpoint",
			checkpointID: "cp-missing-reject-async",
			expectStatus: types.QuestionStatusRejected,
			invoke: func(t *testing.T, svc *chatServiceImpl, sessionID uuid.UUID) *bichatservices.SendMessageResponse {
				t.Helper()
				_, err := svc.RejectPendingQuestionAsync(t.Context(), sessionID)
				require.NoError(t, err)
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

			chatRepo := newMockChatRepository()
			session := mustSession(t,
				withSessionTenantID(env.TenantID()),
				withSessionUserID(1),
				withSessionTitle(tt.sessionTitle),
			)
			require.NoError(t, chatRepo.CreateSession(env.Ctx, session))
			require.NoError(t, chatRepo.SaveMessage(env.Ctx, types.NewMessage(
				types.WithSessionID(session.ID()),
				types.WithRole(types.RoleAssistant),
				types.WithContent("Need scope"),
				types.WithQuestionData(mustQuestionData(t, tt.checkpointID)),
			)))

			agentSvc := &stubAgentService{resumeStreamErr: agents.ErrCheckpointNotFound}
			svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)

			resp := tt.invoke(t, svc, session.ID())
			if tt.assertResponse {
				require.NotNil(t, resp)
				assert.Nil(t, resp.AssistantMessage)
			}

			assertQuestionState := func() bool {
				messages, err := chatRepo.GetSessionMessages(env.Ctx, session.ID(), domain.ListOptions{})
				if err != nil || len(messages) == 0 || messages[0].QuestionData() == nil {
					return false
				}
				updatedQuestionData := messages[0].QuestionData()
				if updatedQuestionData.Status != tt.expectStatus {
					return false
				}
				return assert.ObjectsAreEqual(tt.expectAnswers, updatedQuestionData.Answers)
			}

			if tt.assertResponse {
				require.True(t, assertQuestionState())
			} else {
				require.Eventually(t, assertQuestionState, 2*time.Second, 20*time.Millisecond)
			}

			messages, err := chatRepo.GetSessionMessages(env.Ctx, session.ID(), domain.ListOptions{})
			require.NoError(t, err)
			require.NotEmpty(t, messages)
			updatedQuestionData := messages[0].QuestionData()
			require.NotNil(t, updatedQuestionData)
			assert.Equal(t, tt.expectStatus, updatedQuestionData.Status)
			assert.Equal(t, tt.expectAnswers, updatedQuestionData.Answers)
			assert.False(t, messages[0].HasPendingQuestion())
		})
	}
}

func TestChatService_ResumeWithAnswerAsync_PersistsSubmittedStateBeforeWorkerCompletes(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("resume async submitted"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	qd, err := types.NewQuestionData("cp-async-submit", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	pendingMsg := types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)
	require.NoError(t, chatRepo.SaveMessage(t.Context(), pendingMsg))

	release := make(chan struct{})
	started := make(chan struct{}, 1)
	agentSvc := &stubAgentService{
		resumeStarted: started,
		resumeRelease: release,
		resumeEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeDone},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	_, err = svc.ResumeWithAnswerAsync(t.Context(), bichatservices.ResumeRequest{
		SessionID:    session.ID(),
		CheckpointID: "cp-async-submit",
		Answers: map[string]string{
			"scope": "all",
		},
	})
	require.NoError(t, err)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected async resume worker to start")
	}

	messages, err := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, messages)
	updatedQuestionData := messages[0].QuestionData()
	require.NotNil(t, updatedQuestionData)
	assert.Equal(t, types.QuestionStatusAnswerSubmitted, updatedQuestionData.Status)
	assert.Equal(t, "all", updatedQuestionData.Answers["scope"])
	assert.False(t, messages[0].HasPendingQuestion())
	_, err = chatRepo.GetPendingQuestionMessage(t.Context(), session.ID())
	require.ErrorIs(t, err, domain.ErrNoPendingQuestion)

	close(release)
}

func TestChatService_RejectPendingQuestionAsync_PersistsSubmittedStateBeforeWorkerCompletes(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("reject async submitted"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	qd, err := types.NewQuestionData("cp-async-reject", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	pendingMsg := types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)
	require.NoError(t, chatRepo.SaveMessage(t.Context(), pendingMsg))

	release := make(chan struct{})
	started := make(chan struct{}, 1)
	agentSvc := &stubAgentService{
		resumeStarted: started,
		resumeRelease: release,
		resumeEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeDone},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	_, err = svc.RejectPendingQuestionAsync(t.Context(), session.ID())
	require.NoError(t, err)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected async reject worker to start")
	}

	messages, err := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, messages)
	updatedQuestionData := messages[0].QuestionData()
	require.NotNil(t, updatedQuestionData)
	assert.Equal(t, types.QuestionStatusRejectSubmitted, updatedQuestionData.Status)
	assert.False(t, messages[0].HasPendingQuestion())
	_, err = chatRepo.GetPendingQuestionMessage(t.Context(), session.ID())
	require.ErrorIs(t, err, domain.ErrNoPendingQuestion)

	close(release)
}

func TestChatService_ResumeWithAnswerAsync_ReusesExistingRunForDuplicateAnswers(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("resume async idempotent"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	qd, err := types.NewQuestionData("cp-async-idempotent-answer", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, chatRepo.SaveMessage(t.Context(), types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)))

	release := make(chan struct{})
	started := make(chan struct{}, 1)
	agentSvc := &stubAgentService{
		resumeStarted: started,
		resumeRelease: release,
		resumeEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeDone},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	firstAccepted, err := svc.ResumeWithAnswerAsync(t.Context(), bichatservices.ResumeRequest{
		SessionID:    session.ID(),
		CheckpointID: "cp-async-idempotent-answer",
		Answers: map[string]string{
			"scope": "all",
		},
	})
	require.NoError(t, err)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected async resume worker to start")
	}

	secondAccepted, err := svc.ResumeWithAnswerAsync(t.Context(), bichatservices.ResumeRequest{
		SessionID:    session.ID(),
		CheckpointID: "cp-async-idempotent-answer",
		Answers: map[string]string{
			"scope": "all",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, firstAccepted.RunID, secondAccepted.RunID)
	assert.Equal(t, 1, agentSvc.resumeCalls)

	close(release)
}

func TestChatService_RejectPendingQuestionAsync_ReusesExistingRunForDuplicateReject(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("reject async idempotent"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	qd, err := types.NewQuestionData("cp-async-idempotent-reject", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, chatRepo.SaveMessage(t.Context(), types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)))

	release := make(chan struct{})
	started := make(chan struct{}, 1)
	agentSvc := &stubAgentService{
		resumeStarted: started,
		resumeRelease: release,
		resumeEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeDone},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	firstAccepted, err := svc.RejectPendingQuestionAsync(t.Context(), session.ID())
	require.NoError(t, err)

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("expected async reject worker to start")
	}

	secondAccepted, err := svc.RejectPendingQuestionAsync(t.Context(), session.ID())
	require.NoError(t, err)

	assert.Equal(t, firstAccepted.RunID, secondAccepted.RunID)
	assert.Equal(t, 1, agentSvc.resumeCalls)

	close(release)
}

func TestChatService_ResumeWithAnswerAsync_MarksFailureStateWhenWorkerFails(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("resume async failure"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	qd, err := types.NewQuestionData("cp-async-failure-answer", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, chatRepo.SaveMessage(t.Context(), types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)))

	agentSvc := &stubAgentService{resumeErr: assert.AnError}
	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)

	_, err = svc.ResumeWithAnswerAsync(t.Context(), bichatservices.ResumeRequest{
		SessionID:    session.ID(),
		CheckpointID: "cp-async-failure-answer",
		Answers: map[string]string{
			"scope": "all",
		},
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		messages, messagesErr := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
		if messagesErr != nil || len(messages) == 0 || messages[0].QuestionData() == nil {
			return false
		}
		return messages[0].QuestionData().Status == types.QuestionStatusAnswerFailed
	}, 2*time.Second, 20*time.Millisecond)
}

func TestChatService_RejectPendingQuestionAsync_MarksFailureStateWhenWorkerFails(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("reject async failure"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	qd, err := types.NewQuestionData("cp-async-failure-reject", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, chatRepo.SaveMessage(t.Context(), types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)))

	agentSvc := &stubAgentService{resumeErr: assert.AnError}
	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)

	_, err = svc.RejectPendingQuestionAsync(t.Context(), session.ID())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		messages, messagesErr := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
		if messagesErr != nil || len(messages) == 0 || messages[0].QuestionData() == nil {
			return false
		}
		return messages[0].QuestionData().Status == types.QuestionStatusRejectFailed
	}, 2*time.Second, 20*time.Millisecond)
}

func TestChatService_ResumeWithAnswer_TriggersTitleGenerationAfterCompletion(t *testing.T) {
	t.Parallel()
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("Untitled Session"),
	)
	require.NoError(t, chatRepo.CreateSession(env.Ctx, session))

	qd, err := types.NewQuestionData("cp-title-resume", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	pendingMsg := types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)
	require.NoError(t, chatRepo.SaveMessage(t.Context(), pendingMsg))

	titleService := &captureTitleContextService{
		called: make(chan context.Context, 1),
	}
	agentSvc := &stubAgentService{
		resumeEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeDone},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, titleService, nil)
	_, err = svc.ResumeWithAnswer(env.Ctx, bichatservices.ResumeRequest{
		SessionID:    session.ID(),
		CheckpointID: "cp-title-resume",
		Answers: map[string]string{
			"scope": "all",
		},
	})
	require.NoError(t, err)

	select {
	case <-titleService.called:
	case <-time.After(2 * time.Second):
		t.Fatal("expected title generation after HITL resume completion")
	}
}

func TestChatService_ResumeWithAnswer_DoesNotTriggerTitleGenerationWhenInterruptContinues(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("Untitled Session"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	qd, err := types.NewQuestionData("cp-title-resume-continued", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	pendingMsg := types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)
	require.NoError(t, chatRepo.SaveMessage(t.Context(), pendingMsg))

	titleService := &captureTitleContextService{
		called: make(chan context.Context, 1),
	}
	agentSvc := &stubAgentService{
		resumeEvents: []agents.ExecutorEvent{
			{
				Type: agents.EventTypeInterrupt,
				ParsedInterrupt: &agents.ParsedInterrupt{
					CheckpointID: "cp-next-continued",
					AgentName:    "ali",
					Questions: []agents.Question{
						{
							ID:   "scope",
							Text: "Scope?",
							Type: agents.QuestionTypeSingleChoice,
							Options: []agents.QuestionOption{
								{ID: "sold", Label: "Sold only"},
								{ID: "all", Label: "All policies"},
							},
						},
					},
				},
			},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, titleService, nil)
	_, err = svc.ResumeWithAnswer(t.Context(), bichatservices.ResumeRequest{
		SessionID:    session.ID(),
		CheckpointID: "cp-title-resume-continued",
		Answers: map[string]string{
			"scope": "all",
		},
	})
	require.NoError(t, err)

	select {
	case <-titleService.called:
		t.Fatal("did not expect title generation while HITL interrupt continues")
	default:
	}
}

func TestChatService_RejectPendingQuestion_TriggersTitleGenerationAfterCompletion(t *testing.T) {
	t.Parallel()
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("Untitled Session"),
	)
	require.NoError(t, chatRepo.CreateSession(env.Ctx, session))

	qd, err := types.NewQuestionData("cp-title-reject", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	pendingMsg := types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)
	require.NoError(t, chatRepo.SaveMessage(t.Context(), pendingMsg))

	titleService := &captureTitleContextService{
		called: make(chan context.Context, 1),
	}
	agentSvc := &stubAgentService{
		resumeEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeDone},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, titleService, nil)
	_, err = svc.RejectPendingQuestion(env.Ctx, session.ID())
	require.NoError(t, err)

	select {
	case <-titleService.called:
	case <-time.After(2 * time.Second):
		t.Fatal("expected title generation after HITL reject completion")
	}
}

func TestChatService_ResumeWithAnswerAsync_TriggersTitleGenerationAfterCompletion(t *testing.T) {
	t.Parallel()
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("Untitled Session"),
	)
	require.NoError(t, chatRepo.CreateSession(env.Ctx, session))

	qd, err := types.NewQuestionData("cp-title-resume-async", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	pendingMsg := types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)
	require.NoError(t, chatRepo.SaveMessage(env.Ctx, pendingMsg))

	titleService := &captureTitleContextService{
		called: make(chan context.Context, 1),
	}
	agentSvc := &stubAgentService{
		resumeEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeDone},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, titleService, nil)
	_, err = svc.ResumeWithAnswerAsync(env.Ctx, bichatservices.ResumeRequest{
		SessionID:    session.ID(),
		CheckpointID: "cp-title-resume-async",
		Answers: map[string]string{
			"scope": "all",
		},
	})
	require.NoError(t, err)

	select {
	case <-titleService.called:
	case <-time.After(2 * time.Second):
		t.Fatal("expected title generation after async HITL resume completion")
	}
}

func TestChatService_RejectPendingQuestionAsync_TriggersTitleGenerationAfterCompletion(t *testing.T) {
	t.Parallel()
	env := itf.Setup(t, itf.WithModules(modules.BuiltInModules...))

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("Untitled Session"),
	)
	require.NoError(t, chatRepo.CreateSession(env.Ctx, session))

	qd, err := types.NewQuestionData("cp-title-reject-async", "ali", []types.QuestionDataItem{
		{
			ID:   "scope",
			Text: "Scope?",
			Type: "single_choice",
			Options: []types.QuestionDataOption{
				{ID: "sold", Label: "Sold only"},
				{ID: "all", Label: "All policies"},
			},
		},
	})
	require.NoError(t, err)

	pendingMsg := types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(qd),
	)
	require.NoError(t, chatRepo.SaveMessage(env.Ctx, pendingMsg))

	titleService := &captureTitleContextService{
		called: make(chan context.Context, 1),
	}
	agentSvc := &stubAgentService{
		resumeEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeDone},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, titleService, nil)
	_, err = svc.RejectPendingQuestionAsync(env.Ctx, session.ID())
	require.NoError(t, err)

	select {
	case <-titleService.called:
	case <-time.After(2 * time.Second):
		t.Fatal("expected title generation after async HITL reject completion")
	}
}

func TestChatService_SendMessageStream_StreamErrorStillTriggersTitleGeneration(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("Generating..."),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	titleService := &captureTitleContextService{
		called: make(chan context.Context, 1),
	}
	agentSvc := &stubAgentService{
		processEvents: []agents.ExecutorEvent{
			{
				Type:    agents.EventTypeContent,
				Content: "partial assistant response",
			},
		},
		processStreamErr: assert.AnError,
	}

	svc := NewChatService(chatRepo, agentSvc, nil, titleService, nil)
	err := svc.SendMessageStream(t.Context(), bichatservices.SendMessageRequest{
		SessionID: session.ID(),
		Content:   "first user message",
	}, func(_ bichatservices.StreamChunk) {})

	require.ErrorIs(t, err, assert.AnError)

	messages, msgErr := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, msgErr)
	require.Len(t, messages, 2)
	assert.Equal(t, types.RoleUser, messages[0].Role())
	assert.Equal(t, types.RoleAssistant, messages[1].Role())
	assert.Equal(t, "partial assistant response", messages[1].Content())

	select {
	case <-titleService.called:
	case <-time.After(2 * time.Second):
		t.Fatal("expected async title generation to be invoked")
	}
}

type delayedAssistantSaveChatRepository struct {
	*mockChatRepository
	delay time.Duration
}

func (r *delayedAssistantSaveChatRepository) SaveMessage(ctx context.Context, msg types.Message) error {
	if msg.Role() == types.RoleAssistant && r.delay > 0 {
		time.Sleep(r.delay)
	}
	return r.mockChatRepository.SaveMessage(ctx, msg)
}

func TestChatService_SendMessageStream_DoneEmittedAfterAssistantPersistence(t *testing.T) {
	t.Parallel()

	baseRepo := newMockChatRepository()
	chatRepo := &delayedAssistantSaveChatRepository{
		mockChatRepository: baseRepo,
		delay:              120 * time.Millisecond,
	}
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("stream ordering"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	agentSvc := &stubAgentService{
		processEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeContent, Content: "assistant response"},
			{Type: agents.EventTypeDone},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)

	doneSawPersistedAssistant := false
	err := svc.SendMessageStream(t.Context(), bichatservices.SendMessageRequest{
		SessionID: session.ID(),
		Content:   "hello",
	}, func(chunk bichatservices.StreamChunk) {
		if chunk.Type != bichatservices.ChunkTypeDone {
			return
		}
		messages, msgErr := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
		require.NoError(t, msgErr)
		doneSawPersistedAssistant = len(messages) >= 2 && messages[len(messages)-1].Role() == types.RoleAssistant
	})

	require.NoError(t, err)
	require.True(t, doneSawPersistedAssistant, "done must be emitted only after assistant message is persisted")
}

type assistantFailsWhenTxPresentRepo struct {
	*mockChatRepository
}

func (r *assistantFailsWhenTxPresentRepo) SaveMessage(ctx context.Context, msg types.Message) error {
	if msg.Role() == types.RoleAssistant && ctx.Value(constants.TxKey) != nil {
		return errors.New("assistant save must not reuse request transaction")
	}
	return r.mockChatRepository.SaveMessage(ctx, msg)
}

func TestChatService_SendMessageStream_ClearsRequestTxForAsyncPersistence(t *testing.T) {
	t.Parallel()

	baseRepo := newMockChatRepository()
	chatRepo := &assistantFailsWhenTxPresentRepo{mockChatRepository: baseRepo}
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("tx isolation"),
	)
	require.NoError(t, chatRepo.CreateSession(context.Background(), session))

	agentSvc := &stubAgentService{
		processEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeContent, Content: "assistant response"},
			{Type: agents.EventTypeDone},
		},
	}
	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)

	ctx := context.WithValue(context.Background(), constants.TxKey, struct{}{})
	err := svc.SendMessageStream(ctx, bichatservices.SendMessageRequest{
		SessionID: session.ID(),
		UserID:    1,
		Content:   "hello",
	}, func(_ bichatservices.StreamChunk) {})
	require.NoError(t, err)

	messages, msgErr := chatRepo.GetSessionMessages(context.Background(), session.ID(), domain.ListOptions{})
	require.NoError(t, msgErr)
	require.Len(t, messages, 2)
	assert.Equal(t, types.RoleAssistant, messages[1].Role())
}

type captureTitleContextService struct {
	called      chan context.Context
	regenerated chan context.Context
}

type stubTitleJobQueue struct {
	err       error
	callCount int
	tenantID  uuid.UUID
	sessionID uuid.UUID
}

type stubAgentService struct {
	processEvents    []agents.ExecutorEvent
	processErr       error
	processStreamErr error
	resumeEvents     []agents.ExecutorEvent
	resumeErr        error
	resumeStreamErr  error
	resumeCalls      int
	resumeCheckpoint string
	resumeAnswers    map[string]types.Answer
	resumeStarted    chan struct{}
	resumeRelease    <-chan struct{}
}

func (s *stubAgentService) ProcessMessage(ctx context.Context, sessionID uuid.UUID, content string, attachments []domain.Attachment) (types.Generator[agents.ExecutorEvent], error) {
	if s.processErr != nil {
		return nil, s.processErr
	}
	if len(s.processEvents) == 0 && s.processStreamErr == nil {
		return nil, assert.AnError
	}

	evs := append([]agents.ExecutorEvent{}, s.processEvents...)
	streamErr := s.processStreamErr
	return types.NewGenerator(ctx, func(ctx context.Context, yield func(agents.ExecutorEvent) bool) error {
		for _, ev := range evs {
			if !yield(ev) {
				return nil
			}
		}
		return streamErr
	}), nil
}

func (s *stubAgentService) ResumeWithAnswer(ctx context.Context, sessionID uuid.UUID, checkpointID string, answers map[string]types.Answer) (types.Generator[agents.ExecutorEvent], error) {
	s.resumeCalls++
	s.resumeCheckpoint = checkpointID
	if answers != nil {
		s.resumeAnswers = make(map[string]types.Answer, len(answers))
		for k, v := range answers {
			s.resumeAnswers[k] = v
		}
	} else {
		s.resumeAnswers = nil
	}
	if s.resumeErr != nil {
		return nil, s.resumeErr
	}
	evs := append([]agents.ExecutorEvent{}, s.resumeEvents...)
	streamErr := s.resumeStreamErr
	return types.NewGenerator(ctx, func(ctx context.Context, yield func(agents.ExecutorEvent) bool) error {
		if s.resumeStarted != nil {
			select {
			case s.resumeStarted <- struct{}{}:
			default:
			}
		}
		if s.resumeRelease != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-s.resumeRelease:
			}
		}
		for _, ev := range evs {
			if !yield(ev) {
				return nil
			}
		}
		return streamErr
	}), nil
}

func (s *captureTitleContextService) GenerateSessionTitle(ctx context.Context, _ uuid.UUID) error {
	select {
	case s.called <- ctx:
	default:
	}
	return nil
}

func (s *captureTitleContextService) RegenerateSessionTitle(ctx context.Context, _ uuid.UUID) error {
	if s.regenerated == nil {
		return s.GenerateSessionTitle(ctx, uuid.Nil)
	}
	select {
	case s.regenerated <- ctx:
	default:
	}
	return nil
}

func (s *stubTitleJobQueue) Enqueue(_ context.Context, tenantID uuid.UUID, sessionID uuid.UUID) error {
	s.callCount++
	s.tenantID = tenantID
	s.sessionID = sessionID
	return s.err
}

func TestChatService_MaybeGenerateTitleAsync_PreservesTenantContext(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	titleService := &captureTitleContextService{
		called: make(chan context.Context, 1),
	}
	queue := &stubTitleJobQueue{}
	svc := &chatServiceImpl{
		titleService: titleService,
		titleQueue:   queue,
	}

	sessionID := uuid.New()
	reqCtx := composables.WithTenantID(context.Background(), tenantID)
	reqCtx = context.WithValue(reqCtx, constants.TxKey, "should-not-leak")
	svc.maybeGenerateTitleAsync(reqCtx, sessionID)

	require.Equal(t, 1, queue.callCount)
	assert.Equal(t, tenantID, queue.tenantID)
	assert.Equal(t, sessionID, queue.sessionID)

	select {
	case <-titleService.called:
		t.Fatal("title service should not be called when queue enqueue succeeds")
	default:
	}
}

func TestChatService_MaybeGenerateTitleAsync_IgnoresNilWrappedQueue(t *testing.T) {
	t.Parallel()

	env := itf.Setup(
		t,
		itf.WithModules(modules.BuiltInModules...),
	)
	titleService := &captureTitleContextService{
		called: make(chan context.Context, 1),
	}
	var queue *RedisTitleJobQueue
	svc := &chatServiceImpl{
		titleService: titleService,
		titleQueue:   queue,
	}

	sessionID := uuid.New()
	svc.maybeGenerateTitleAsync(env.Ctx, sessionID)

	select {
	case titleCtx := <-titleService.called:
		gotTenantID, err := composables.UseTenantID(titleCtx)
		require.NoError(t, err)
		assert.Equal(t, env.Tenant.ID, gotTenantID)
	default:
		t.Fatal("expected sync fallback title generation to be invoked")
	}
}

func TestChatService_GenerateSessionTitle_UsesRegenerationService(t *testing.T) {
	t.Parallel()

	titleService := &captureTitleContextService{
		called:      make(chan context.Context, 1),
		regenerated: make(chan context.Context, 1),
	}
	svc := &chatServiceImpl{
		titleService: titleService,
	}

	err := svc.GenerateSessionTitle(context.Background(), uuid.New())
	require.NoError(t, err)

	select {
	case <-titleService.regenerated:
	default:
		t.Fatal("expected regenerate method to be called")
	}
	select {
	case <-titleService.called:
		t.Fatal("expected auto-generation method not to be called")
	default:
	}
}

func TestChatService_MaybeGenerateTitleAsync_FallbackWhenQueueEnqueueFails(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()
	titleService := &captureTitleContextService{
		called: make(chan context.Context, 1),
	}
	queue := &stubTitleJobQueue{err: assert.AnError}
	svc := &chatServiceImpl{
		titleService: titleService,
		titleQueue:   queue,
	}

	reqCtx := composables.WithTenantID(context.Background(), tenantID)
	reqCtx = context.WithValue(reqCtx, constants.TxKey, "should-not-leak")
	svc.maybeGenerateTitleAsync(reqCtx, uuid.New())

	select {
	case titleCtx := <-titleService.called:
		gotTenantID, err := composables.UseTenantID(titleCtx)
		require.NoError(t, err)
		assert.Equal(t, tenantID, gotTenantID)
	case <-time.After(2 * time.Second):
		t.Fatal("expected sync fallback title generation to be invoked")
	}
}

func TestBuildTitleGenerationContext_DoesNotCarryTx(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()

	reqCtx := composables.WithTenantID(context.Background(), tenantID)
	reqCtx = context.WithValue(reqCtx, constants.TxKey, "should-not-leak")

	titleCtx := buildTitleGenerationContext(reqCtx)

	gotTenantID, err := composables.UseTenantID(titleCtx)
	require.NoError(t, err)
	assert.Equal(t, tenantID, gotTenantID)

	_, err = composables.UseTx(titleCtx)
	require.ErrorIs(t, err, composables.ErrNoPool)
}

func TestChatService_StopGeneration_NoErrorWhenNoActiveStream(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	svc := NewChatService(chatRepo, nil, nil, nil, nil)

	err := svc.StopGeneration(context.Background(), uuid.New())
	require.NoError(t, err)
}

func TestChatService_SendMessageStream_ClientDisconnectStillPersistsAssistant(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("stop test"),
	)
	require.NoError(t, chatRepo.CreateSession(context.Background(), session))

	agentSvc := &stubAgentService{
		processEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeContent, Content: "persist me"},
			{Type: agents.EventTypeDone},
		},
	}

	svc := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		_ = svc.SendMessageStream(ctx, bichatservices.SendMessageRequest{
			SessionID: session.ID(),
			UserID:    1,
			Content:   "hello",
		}, func(chunk bichatservices.StreamChunk) {
			if chunk.Type == bichatservices.ChunkTypeContent {
				cancel()
			}
		})
	}()

	<-streamDone
	var (
		messages []types.Message
		err      error
	)
	deadline := time.Now().Add(2 * time.Second)
	for {
		messages, err = chatRepo.GetSessionMessages(context.Background(), session.ID(), domain.ListOptions{})
		require.NoError(t, err)
		if len(messages) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for assistant persistence, got %d messages", len(messages))
		}
		time.Sleep(20 * time.Millisecond)
	}

	require.Len(t, messages, 2, "assistant message should be persisted after client disconnect")
	assert.Equal(t, types.RoleUser, messages[0].Role())
	assert.Equal(t, types.RoleAssistant, messages[1].Role())
	assert.Equal(t, "persist me", messages[1].Content())
}

func TestChatService_SendMessageStream_StopGenerationDoesNotPersistAssistant(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("stop test"),
	)
	require.NoError(t, chatRepo.CreateSession(context.Background(), session))

	cancelAgent := &cancelAwareAgentService{
		events: []agents.ExecutorEvent{
			{Type: agents.EventTypeContent, Content: "partial"},
		},
	}

	svc := NewChatService(chatRepo, cancelAgent, nil, nil, nil)
	ctx := context.Background()
	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		_ = svc.SendMessageStream(ctx, bichatservices.SendMessageRequest{
			SessionID: session.ID(),
			UserID:    1,
			Content:   "hello",
		}, func(_ bichatservices.StreamChunk) {})
	}()

	time.Sleep(50 * time.Millisecond)
	require.NoError(t, svc.StopGeneration(context.Background(), session.ID()))
	<-streamDone

	messages, err := chatRepo.GetSessionMessages(context.Background(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	require.Len(t, messages, 1, "only user message should be persisted when stream is explicitly stopped")
	assert.Equal(t, types.RoleUser, messages[0].Role())
}

type cancelAwareAgentService struct {
	events []agents.ExecutorEvent
}

func (s *cancelAwareAgentService) ProcessMessage(ctx context.Context, _ uuid.UUID, _ string, _ []domain.Attachment) (types.Generator[agents.ExecutorEvent], error) {
	evs := append([]agents.ExecutorEvent{}, s.events...)
	return types.NewGenerator(ctx, func(ctx context.Context, yield func(agents.ExecutorEvent) bool) error {
		for _, ev := range evs {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if !yield(ev) {
					return nil
				}
			}
		}
		<-ctx.Done()
		return ctx.Err()
	}), nil
}

func (s *cancelAwareAgentService) ResumeWithAnswer(context.Context, uuid.UUID, string, map[string]types.Answer) (types.Generator[agents.ExecutorEvent], error) {
	return nil, assert.AnError
}

func TestChatService_GetStreamStatus_ReturnsInactiveWhenNoRun(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	svc := NewChatService(chatRepo, nil, nil, nil, nil)

	sessionID := uuid.New()
	status, err := svc.GetStreamStatus(context.Background(), sessionID)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.False(t, status.Active)
}

func TestChatService_ResumeStream_ReturnsErrWhenRunNotFound(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	svc := NewChatService(chatRepo, nil, nil, nil, nil)

	sessionID := uuid.New()
	runID := uuid.New()
	err := svc.ResumeStream(context.Background(), sessionID, runID, func(bichatservices.StreamChunk) {})
	require.ErrorIs(t, err, bichatservices.ErrRunNotFoundOrFinished)
}
