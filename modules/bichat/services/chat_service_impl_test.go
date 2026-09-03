package services

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	streamingsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/streaming"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingCompleteRunStore struct {
	generationRunStore
	completeCalls int
	failures      int
}

func (s *failingCompleteRunStore) CompleteRun(
	ctx context.Context,
	tenantID, sessionID, runID uuid.UUID,
) error {
	s.completeCalls++
	if s.completeCalls <= s.failures {
		return errors.New("temporary redis finalization failure")
	}
	return s.generationRunStore.CompleteRun(ctx, tenantID, sessionID, runID)
}

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

func mustQuestionDataWithStatus(t *testing.T, checkpointID string, status types.QuestionStatus) *types.QuestionData {
	t.Helper()

	qd := mustQuestionData(t, checkpointID)
	switch status {
	case types.QuestionStatusPending:
		return qd
	case types.QuestionStatusAnswerSubmitted:
		submitted, err := qd.SubmitAnswers(map[string]string{"scope": "all"})
		require.NoError(t, err)
		return submitted
	case types.QuestionStatusRejectSubmitted,
		types.QuestionStatusAnswerFailed,
		types.QuestionStatusRejectFailed,
		types.QuestionStatusAnswered,
		types.QuestionStatusRejected:
		require.Failf(t, "unsupported question status for test", "status %s is not supported for open-question send guards", status)
		return nil
	default:
		require.Failf(t, "unknown question status for test", "status %s is not recognized", status)
		return nil
	}
}

func TestChatService_UnarchiveSession(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	svc, err := NewChatService(chatRepo, nil, nil, nil, nil)
	require.NoError(t, err)

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
	svc, err := NewChatService(chatRepo, nil, nil, nil, nil)
	require.NoError(t, err)

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
	svc, err := NewChatService(chatRepo, nil, model, nil, nil)
	require.NoError(t, err)

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
	svc, err := NewChatService(chatRepo, nil, model, nil, nil)
	require.NoError(t, err)

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
	svc, err := NewChatService(chatRepo, nil, nil, nil, nil)
	require.NoError(t, err)

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

func TestChatService_SendMessage_RejectsWhileQuestionOpen(t *testing.T) {
	t.Parallel()

	for _, status := range []types.QuestionStatus{
		types.QuestionStatusPending,
		types.QuestionStatusAnswerSubmitted,
	} {
		t.Run(string(status), func(t *testing.T) {
			chatRepo := newMockChatRepository()
			svc, err := NewChatService(chatRepo, nil, nil, nil, nil)
			require.NoError(t, err)

			session := mustSession(t,
				withSessionTenantID(uuid.New()),
				withSessionUserID(1),
				withSessionTitle("pending question"),
			)
			require.NoError(t, chatRepo.CreateSession(t.Context(), session))

			require.NoError(t, chatRepo.SaveMessage(t.Context(), types.AssistantMessage(
				"Need clarification",
				types.WithSessionID(session.ID()),
				types.WithQuestionData(mustQuestionDataWithStatus(t, "cp-open", status)),
			)))

			var sendErr error
			_, sendErr = svc.SendMessage(t.Context(), bichatservices.SendMessageRequest{
				SessionID: session.ID(),
				UserID:    1,
				Content:   "continue",
			})
			require.Error(t, sendErr)
			require.ErrorContains(t, sendErr, errHITLPendingQuestionOpen.Error())

			messages, msgErr := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
			require.NoError(t, msgErr)
			require.Len(t, messages, 1)
		})
	}
}

func TestChatService_SendMessageStream_RejectsWhileQuestionOpen(t *testing.T) {
	t.Parallel()

	for _, status := range []types.QuestionStatus{
		types.QuestionStatusPending,
		types.QuestionStatusAnswerSubmitted,
	} {
		t.Run(string(status), func(t *testing.T) {
			chatRepo := newMockChatRepository()
			svc, err := NewChatService(chatRepo, nil, nil, nil, nil)
			require.NoError(t, err)

			session := mustSession(t,
				withSessionTenantID(uuid.New()),
				withSessionUserID(1),
				withSessionTitle("pending question"),
			)
			require.NoError(t, chatRepo.CreateSession(t.Context(), session))

			require.NoError(t, chatRepo.SaveMessage(t.Context(), types.AssistantMessage(
				"Need clarification",
				types.WithSessionID(session.ID()),
				types.WithQuestionData(mustQuestionDataWithStatus(t, "cp-open-stream", status)),
			)))

			streamErr := svc.SendMessageStream(t.Context(), bichatservices.SendMessageRequest{
				SessionID: session.ID(),
				UserID:    1,
				Content:   "continue",
			}, func(bichatservices.StreamChunk) {})
			require.Error(t, streamErr)
			require.ErrorContains(t, streamErr, errHITLPendingQuestionOpen.Error())

			messages, msgErr := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
			require.NoError(t, msgErr)
			require.Len(t, messages, 1)
		})
	}
}

func TestChatService_MaybeReplaceHistoryFromMessage_RejectsNonUserMessage(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	svc, err := NewChatService(chatRepo, nil, nil, nil, nil)
	require.NoError(t, err)

	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("replace"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	assistant := types.AssistantMessage("answer", types.WithSessionID(session.ID()))
	require.NoError(t, chatRepo.SaveMessage(t.Context(), assistant))

	replaceFromID := assistant.ID()
	_, replaceErr := svc.maybeReplaceHistoryFromMessage(t.Context(), session, &replaceFromID)
	require.Error(t, replaceErr)
	assert.Contains(t, replaceErr.Error(), "replaceFromMessageId must point to a user message")
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

	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
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

	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
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

	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
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

	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
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
			env := itf.Setup(t, itf.WithComponents(modules.Components()...))

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
			svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
			require.NoError(t, err)

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

	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
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

func TestChatService_CompleteRunState_RetriesRedisFinalization(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	baseStore, err := newRedisGenerationRunStore(redisGenerationRunStoreConfig{Client: client})
	require.NoError(t, err)
	store := &failingCompleteRunStore{generationRunStore: baseStore, failures: 1}

	tenantID := uuid.New()
	sessionID := uuid.New()
	run, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		SessionID: sessionID,
		TenantID:  tenantID,
		UserID:    1,
	})
	require.NoError(t, err)

	chatRepo := newMockChatRepository()
	require.NoError(t, chatRepo.CreateRun(t.Context(), run))
	require.NoError(t, baseStore.CreateRun(t.Context(), run))

	svc, err := NewChatService(chatRepo, nil, nil, nil, nil)
	require.NoError(t, err)
	svc.runState = streamingsvc.NewRunStateManager(store)

	// Falsely green if the store never fails between SQL completion and Redis cleanup.
	require.NoError(t, svc.completeRunState(t.Context(), tenantID, sessionID, run.ID()))
	assert.Equal(t, 2, store.completeCalls)

	databaseRun, err := chatRepo.GetRunByID(t.Context(), run.ID())
	require.NoError(t, err)
	assert.Equal(t, domain.GenerationRunStatusCompleted, databaseRun.Status())
	_, err = baseStore.GetActiveRunBySession(t.Context(), tenantID, sessionID)
	require.ErrorIs(t, err, domain.ErrNoActiveRun)
}

func TestChatService_CompleteRunState_LogsTerminalRedisFailure(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	baseStore, err := newRedisGenerationRunStore(redisGenerationRunStoreConfig{Client: client})
	require.NoError(t, err)
	store := &failingCompleteRunStore{
		generationRunStore: baseStore,
		failures:           runStateFinalizationAttempts,
	}

	tenantID := uuid.New()
	sessionID := uuid.New()
	run, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		SessionID: sessionID,
		TenantID:  tenantID,
		UserID:    1,
	})
	require.NoError(t, err)

	chatRepo := newMockChatRepository()
	require.NoError(t, chatRepo.CreateRun(t.Context(), run))
	require.NoError(t, baseStore.CreateRun(t.Context(), run))

	var logs bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logs)
	logger.SetFormatter(&logrus.JSONFormatter{})
	svc, err := NewChatService(chatRepo, nil, nil, nil, nil)
	require.NoError(t, err)
	svc.WithLogger(logger)
	svc.runState = streamingsvc.NewRunStateManager(store)

	// Falsely green if terminal Redis errors are swallowed without identifiers.
	require.Error(t, svc.completeRunState(t.Context(), tenantID, sessionID, run.ID()))
	assert.Equal(t, runStateFinalizationAttempts, store.completeCalls)
	assert.Contains(t, logs.String(), "failed to finalize generation run state")
	assert.Contains(t, logs.String(), tenantID.String())
	assert.Contains(t, logs.String(), sessionID.String())
	assert.Contains(t, logs.String(), run.ID().String())
}

func TestChatService_ResumeWithAnswerAsync_RecoversOrphanedRedisRun(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store, err := newRedisGenerationRunStore(redisGenerationRunStoreConfig{Client: client})
	require.NoError(t, err)

	tenantID := uuid.New()
	session := mustSession(t,
		withSessionTenantID(tenantID),
		withSessionUserID(1),
		withSessionTitle("resume after orphaned redis run"),
	)
	orphanedRun, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		SessionID: session.ID(),
		TenantID:  tenantID,
		UserID:    session.UserID(),
	})
	require.NoError(t, err)
	require.NoError(t, store.CreateRun(t.Context(), orphanedRun))

	chatRepo := newMockChatRepository()
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))
	require.NoError(t, chatRepo.SaveMessage(t.Context(), types.NewMessage(
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("Need scope"),
		types.WithQuestionData(mustQuestionData(t, "cp-orphaned-redis-run")),
	)))

	agentSvc := &stubAgentService{resumeEvents: []agents.ExecutorEvent{{Type: agents.EventTypeDone}}}
	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
	svc.runState = streamingsvc.NewRunStateManager(store)

	// Falsely green if resume bypasses the real Redis SETNX conflict.
	accepted, err := svc.ResumeWithAnswerAsync(t.Context(), bichatservices.ResumeRequest{
		SessionID:    session.ID(),
		CheckpointID: "cp-orphaned-redis-run",
		Answers:      map[string]string{"scope": "all"},
	})
	require.NoError(t, err)
	assert.NotEqual(t, orphanedRun.ID(), accepted.RunID)

	recovered, err := store.GetRunByID(t.Context(), tenantID, orphanedRun.ID())
	require.NoError(t, err)
	assert.Equal(t, domain.GenerationRunStatusFailed, recovered.Status())
}

func TestChatService_SendMessageStream_RecoversOrphanedRedisRun(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store, err := newRedisGenerationRunStore(redisGenerationRunStoreConfig{Client: client})
	require.NoError(t, err)

	tenantID := uuid.New()
	session := mustSession(t,
		withSessionTenantID(tenantID),
		withSessionUserID(1),
		withSessionTitle("send after orphaned redis run"),
	)
	orphanedRun, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		SessionID: session.ID(),
		TenantID:  tenantID,
		UserID:    session.UserID(),
	})
	require.NoError(t, err)
	require.NoError(t, store.CreateRun(t.Context(), orphanedRun))

	chatRepo := newMockChatRepository()
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))
	agentSvc := &stubAgentService{processEvents: []agents.ExecutorEvent{
		{Type: agents.EventTypeContent, Content: "recovered response"},
		{Type: agents.EventTypeDone},
	}}
	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
	svc.runState = streamingsvc.NewRunStateManager(store)

	var replacementRunID uuid.UUID
	// Falsely green if SendMessageStream bypasses PostgreSQL run creation or
	// the real Redis SETNX conflict that guards one active run per session.
	err = svc.SendMessageStream(t.Context(), bichatservices.SendMessageRequest{
		SessionID: session.ID(),
		Content:   "continue",
	}, func(chunk bichatservices.StreamChunk) {
		if chunk.Type == bichatservices.ChunkTypeStreamStarted {
			replacementRunID = uuid.MustParse(chunk.RunID)
		}
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, replacementRunID)
	assert.NotEqual(t, orphanedRun.ID(), replacementRunID)

	recovered, err := store.GetRunByID(t.Context(), tenantID, orphanedRun.ID())
	require.NoError(t, err)
	assert.Equal(t, domain.GenerationRunStatusFailed, recovered.Status())
	replacement, err := chatRepo.GetRunByID(t.Context(), replacementRunID)
	require.NoError(t, err)
	assert.Equal(t, domain.GenerationRunStatusCompleted, replacement.Status())
	_, err = store.GetActiveRunBySession(t.Context(), tenantID, session.ID())
	require.ErrorIs(t, err, domain.ErrNoActiveRun)
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

	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
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

	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
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

	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
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
	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)

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
	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)

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
	env := itf.Setup(t, itf.WithComponents(modules.Components()...))

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

	svc, err := NewChatService(chatRepo, agentSvc, nil, titleService, nil)
	require.NoError(t, err)
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

	svc, err := NewChatService(chatRepo, agentSvc, nil, titleService, nil)
	require.NoError(t, err)
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
	env := itf.Setup(t, itf.WithComponents(modules.Components()...))

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

	svc, err := NewChatService(chatRepo, agentSvc, nil, titleService, nil)
	require.NoError(t, err)
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
	env := itf.Setup(t, itf.WithComponents(modules.Components()...))

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

	svc, err := NewChatService(chatRepo, agentSvc, nil, titleService, nil)
	require.NoError(t, err)
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
	env := itf.Setup(t, itf.WithComponents(modules.Components()...))

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

	svc, err := NewChatService(chatRepo, agentSvc, nil, titleService, nil)
	require.NoError(t, err)
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

	svc, err := NewChatService(chatRepo, agentSvc, nil, titleService, nil)
	require.NoError(t, err)
	streamErr := svc.SendMessageStream(t.Context(), bichatservices.SendMessageRequest{
		SessionID: session.ID(),
		Content:   "first user message",
	}, func(_ bichatservices.StreamChunk) {})

	require.ErrorIs(t, streamErr, assert.AnError)

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

	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)

	doneSawPersistedAssistant := false
	streamErr := svc.SendMessageStream(t.Context(), bichatservices.SendMessageRequest{
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

	require.NoError(t, streamErr)
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
	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), constants.TxKey, struct{}{})
	streamErr := svc.SendMessageStream(ctx, bichatservices.SendMessageRequest{
		SessionID: session.ID(),
		UserID:    1,
		Content:   "hello",
	}, func(_ bichatservices.StreamChunk) {})
	require.NoError(t, streamErr)

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
	processEvents       []agents.ExecutorEvent
	processErr          error
	processStreamErr    error
	resumeEvents        []agents.ExecutorEvent
	resumeErr           error
	resumeStreamErr     error
	resumeCalls         int
	resumeCheckpoint    string
	resumeAnswers       map[string]types.Answer
	resumeStarted       chan struct{}
	resumeRelease       <-chan struct{}
	continuationEvents  []agents.ExecutorEvent
	continuationErr     error
	continuationStarted chan bichatservices.ContinuationEvent
	continuationRelease <-chan struct{}
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

func (s *stubAgentService) ProcessContinuation(
	ctx context.Context,
	_ uuid.UUID,
	event bichatservices.ContinuationEvent,
) (types.Generator[agents.ExecutorEvent], error) {
	if s.continuationStarted != nil {
		select {
		case s.continuationStarted <- event:
		default:
		}
	}
	if s.continuationErr != nil {
		return nil, s.continuationErr
	}
	evs := append([]agents.ExecutorEvent(nil), s.continuationEvents...)
	return types.NewGenerator(ctx, func(ctx context.Context, yield func(agents.ExecutorEvent) bool) error {
		if s.continuationRelease != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-s.continuationRelease:
			}
		}
		for _, ev := range evs {
			if !yield(ev) {
				return nil
			}
		}
		return nil
	}), nil
}

func TestChatService_ContinueSession_DeduplicatesConcurrentDelivery(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("concurrent continuation"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	started := make(chan bichatservices.ContinuationEvent, 2)
	release := make(chan struct{})
	agentSvc := &stubAgentService{
		continuationStarted: started,
		continuationRelease: release,
		continuationEvents:  []agents.ExecutorEvent{{Type: agents.EventTypeDone}},
	}
	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
	request := bichatservices.ContinueSessionRequest{
		SessionID: session.ID(),
		Event: bichatservices.ContinuationEvent{
			Trigger:       "background_task_completed",
			CorrelationID: "snapshot-concurrent",
		},
		IdempotencyKey: "workflow-1/concurrent",
	}

	first, err := svc.ContinueSession(t.Context(), request)
	require.NoError(t, err)
	select {
	case <-started:
	case <-time.After(time.Second):
		require.Fail(t, "first continuation did not start")
	}

	second, err := svc.ContinueSession(t.Context(), request)
	require.NoError(t, err)
	assert.Equal(t, first.RunID, second.RunID)
	select {
	case <-started:
		require.Fail(t, "duplicate continuation started a second agent turn")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
}

func TestChatService_ContinueSession_PersistsAssistantWithoutUserMessage(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("durable workflow"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	started := make(chan bichatservices.ContinuationEvent, 1)
	agentSvc := &stubAgentService{
		continuationStarted: started,
		continuationEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeContent, Content: "analysis resumed"},
			{Type: agents.EventTypeDone},
		},
	}
	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)

	accepted, err := svc.ContinueSession(t.Context(), bichatservices.ContinueSessionRequest{
		SessionID: session.ID(),
		Event: bichatservices.ContinuationEvent{
			Trigger:       "background_task_completed",
			CorrelationID: "snapshot-42",
			Payload:       []byte(`{"artifact_id":"artifact-7"}`),
		},
		IdempotencyKey: "workflow-1/snapshot-42",
	})
	require.NoError(t, err)
	assert.True(t, accepted.Accepted)
	assert.Equal(t, bichatservices.AsyncRunOperationContinuation, accepted.Operation)
	assert.NotEqual(t, uuid.Nil, accepted.RunID)

	select {
	case event := <-started:
		assert.Equal(t, "background_task_completed", event.Trigger)
	case <-time.After(time.Second):
		require.Fail(t, "continuation did not start")
	}

	require.Eventually(t, func() bool {
		messages, msgErr := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
		if msgErr != nil || len(messages) != 1 {
			return false
		}
		return messages[0].Role() == types.RoleAssistant && messages[0].Content() == "analysis resumed"
	}, time.Second, 10*time.Millisecond)
}

func TestChatService_ContinueSession_DeduplicatesCompletedDelivery(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("idempotent continuation"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	started := make(chan bichatservices.ContinuationEvent, 2)
	agentSvc := &stubAgentService{
		continuationStarted: started,
		continuationEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeContent, Content: "one durable result"},
			{Type: agents.EventTypeDone},
		},
	}
	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
	request := bichatservices.ContinueSessionRequest{
		SessionID: session.ID(),
		Event: bichatservices.ContinuationEvent{
			Trigger:       "background_task_completed",
			CorrelationID: "snapshot-42",
			Payload:       []byte(`{"artifact_id":"artifact-7"}`),
		},
		IdempotencyKey: "workflow-1/snapshot-42",
	}

	first, err := svc.ContinueSession(t.Context(), request)
	require.NoError(t, err)
	select {
	case <-started:
	case <-time.After(time.Second):
		require.Fail(t, "first continuation did not start")
	}
	require.Eventually(t, func() bool {
		run, runErr := chatRepo.GetRunByID(t.Context(), first.RunID)
		return runErr == nil && run.Status() == domain.GenerationRunStatusCompleted
	}, time.Second, 10*time.Millisecond)
	runStatus, err := svc.GetContinuationRun(t.Context(), first.RunID)
	require.NoError(t, err)
	assert.Equal(t, first.RunID, runStatus.ID)
	assert.Equal(t, session.ID(), runStatus.SessionID)
	assert.Equal(t, string(domain.GenerationRunStatusCompleted), runStatus.Status)
	assert.False(t, runStatus.UpdatedAt.IsZero())

	second, err := svc.ContinueSession(t.Context(), request)
	require.NoError(t, err)
	assert.True(t, second.Accepted)
	assert.Equal(t, first.RunID, second.RunID)
	select {
	case <-started:
		require.Fail(t, "duplicate continuation started a second agent turn")
	case <-time.After(100 * time.Millisecond):
	}

	messages, err := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "one durable result", messages[0].Content())
}

func TestChatService_ContinueSession_PersistsTerminalFailureReason(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("failed continuation"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	agentSvc := &stubAgentService{
		continuationErr: errors.New("provider request rejected"),
	}
	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
	request := bichatservices.ContinueSessionRequest{
		SessionID: session.ID(),
		Event: bichatservices.ContinuationEvent{
			Trigger:       "background_task_completed",
			CorrelationID: "snapshot-42",
		},
		IdempotencyKey: "workflow-1/failed",
	}
	accepted, err := svc.ContinueSession(t.Context(), request)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		run, runErr := svc.GetContinuationRun(t.Context(), accepted.RunID)
		return runErr == nil &&
			run.Status == string(domain.GenerationRunStatusFailed) &&
			run.Error == "provider request rejected"
	}, time.Second, 10*time.Millisecond)

	// A failed delivery can be retried with the same key and deterministic id.
	retryService, err := NewChatService(
		chatRepo,
		&stubAgentService{continuationEvents: []agents.ExecutorEvent{{Type: agents.EventTypeDone}}},
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	retried, err := retryService.ContinueSession(t.Context(), request)
	require.NoError(t, err)
	assert.Equal(t, accepted.RunID, retried.RunID)
	require.Eventually(t, func() bool {
		run, runErr := retryService.GetContinuationRun(t.Context(), retried.RunID)
		return runErr == nil && run.Status == string(domain.GenerationRunStatusCompleted)
	}, time.Second, 10*time.Millisecond)
}

func TestChatService_ContinueSession_ReplacesStaleDatabaseRun(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("restart-safe continuation"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	staleAt := time.Now().Add(-domain.GenerationRunStaleAfter - time.Minute)
	staleRun, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		ID:            uuid.New(),
		SessionID:     session.ID(),
		TenantID:      session.TenantID(),
		UserID:        session.UserID(),
		Status:        domain.GenerationRunStatusStreaming,
		StartedAt:     staleAt,
		LastUpdatedAt: staleAt,
	})
	require.NoError(t, err)
	require.NoError(t, chatRepo.CreateRun(t.Context(), staleRun))

	started := make(chan bichatservices.ContinuationEvent, 1)
	agentSvc := &stubAgentService{
		continuationStarted: started,
		continuationEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeDone},
		},
	}
	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)

	accepted, err := svc.ContinueSession(t.Context(), bichatservices.ContinueSessionRequest{
		SessionID: session.ID(),
		Event: bichatservices.ContinuationEvent{
			Trigger:       "background_task_completed",
			CorrelationID: "snapshot-after-restart",
		},
		IdempotencyKey: "workflow-1/generation-2",
	})
	require.NoError(t, err)
	assert.True(t, accepted.Accepted)
	assert.NotEqual(t, staleRun.ID(), accepted.RunID)

	select {
	case <-started:
	case <-time.After(time.Second):
		require.Fail(t, "replacement continuation did not start")
	}

	reaped, err := chatRepo.GetRunByID(t.Context(), staleRun.ID())
	require.NoError(t, err)
	assert.Equal(t, domain.GenerationRunStatusCancelled, reaped.Status())
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
		itf.WithComponents(modules.Components()...),
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
	svc, err := NewChatService(chatRepo, nil, nil, nil, nil)
	require.NoError(t, err)

	stopErr := svc.StopGeneration(context.Background(), uuid.New())
	require.NoError(t, stopErr)
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

	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)
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
	var messages []types.Message
	deadline := time.Now().Add(2 * time.Second)
	for {
		var msgErr error
		messages, msgErr = chatRepo.GetSessionMessages(context.Background(), session.ID(), domain.ListOptions{})
		require.NoError(t, msgErr)
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

	svc, err := NewChatService(chatRepo, cancelAgent, nil, nil, nil)
	require.NoError(t, err)
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
	svc, err := NewChatService(chatRepo, nil, nil, nil, nil)
	require.NoError(t, err)

	sessionID := uuid.New()
	status, err := svc.GetStreamStatus(context.Background(), sessionID)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.False(t, status.Active)
}

func TestChatService_ResumeStream_ReturnsErrWhenRunNotFound(t *testing.T) {
	t.Parallel()

	chatRepo := newMockChatRepository()
	svc, err := NewChatService(chatRepo, nil, nil, nil, nil)
	require.NoError(t, err)

	sessionID := uuid.New()
	runID := uuid.New()
	resumeErr := svc.ResumeStream(context.Background(), sessionID, runID, func(bichatservices.StreamChunk) {})
	require.ErrorIs(t, resumeErr, bichatservices.ErrRunNotFoundOrFinished)
}
