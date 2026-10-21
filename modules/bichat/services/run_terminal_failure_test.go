package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	streamingsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/streaming"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Falsely green if the HTTP reader drains away the second terminal event after seeing an error.
func TestRunExecutor_InterruptThenGeneratorErrorDoesNotEmitSuccess(t *testing.T) {
	repo := newMockChatRepository()
	session := mustSession(t, withSessionTenantID(uuid.New()), withSessionUserID(1))
	require.NoError(t, repo.CreateSession(t.Context(), session))
	agent := &stubAgentService{processStreamErr: assert.AnError, processEvents: []agents.ExecutorEvent{{Type: agents.EventTypeInterrupt, ParsedInterrupt: &agents.ParsedInterrupt{CheckpointID: "checkpoint", AgentName: "agent", Questions: []agents.Question{{ID: "period", Text: "Period?", Type: agents.QuestionTypeSingleChoice, Options: []agents.QuestionOption{{ID: "month", Label: "Month"}}}}}}}}
	svc, err := NewChatService(repo, agent, nil, nil, nil)
	require.NoError(t, err)
	job := RunJobPayload{TenantID: session.TenantID(), SessionID: session.ID(), RunID: uuid.New(), UserMessageID: uuid.New(), Content: "ask"}
	active := streamingsvc.NewActiveRun(job.RunID, job.SessionID, func() {}, time.Now())
	chunks := make(chan bichatservices.StreamChunk, 32)
	active.AddSubscriber(chunks)
	svc.runRegistry.Add(active)
	require.NoError(t, svc.runExecutor.Execute(t.Context(), job))
	failures, successes := 0, 0
	for chunk := range chunks {
		if chunk.Type == bichatservices.ChunkTypeError {
			failures++
			require.ErrorIs(t, chunk.Error, assert.AnError)
		}
		if chunk.Type == bichatservices.ChunkTypeDone {
			successes++
		}
	}
	require.Equal(t, 1, failures)
	require.Zero(t, successes)
	messages, err := repo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, err)
	for _, message := range messages {
		require.Nil(t, message.QuestionData())
	}
}
