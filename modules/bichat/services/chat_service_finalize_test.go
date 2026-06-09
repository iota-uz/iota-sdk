package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingArtifactSaveRepo fails every artifact write but otherwise behaves like
// the in-memory mock. Used to drive the best-effort artifact branch of stream
// finalization.
type failingArtifactSaveRepo struct {
	*mockChatRepository
}

func (r *failingArtifactSaveRepo) SaveArtifact(_ context.Context, _ domain.Artifact) error {
	return errors.New("artifact store unavailable")
}

// failingAssistantSaveRepo fails the assistant message write (the critical
// persist step) while letting the user message through.
type failingAssistantSaveRepo struct {
	*mockChatRepository
}

func (r *failingAssistantSaveRepo) SaveMessage(ctx context.Context, msg types.Message) error {
	if msg.Role() == types.RoleAssistant {
		return errors.New("assistant message write failed")
	}
	return r.mockChatRepository.SaveMessage(ctx, msg)
}

// When only artifact persistence fails, the generated answer must still be
// saved and the run completed with a done chunk — never discarded (#2998).
func TestChatService_SendMessageStream_ArtifactPersistFailureKeepsAnswer(t *testing.T) {
	t.Parallel()

	baseRepo := newMockChatRepository()
	chatRepo := &failingArtifactSaveRepo{mockChatRepository: baseRepo}
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("artifact best-effort"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	agentSvc := &stubAgentService{
		processEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeContent, Content: "final answer"},
			{
				Type: agents.EventTypeToolEnd,
				Tool: &agents.ToolEvent{
					CallID:    "call-1",
					Name:      "sql_execute",
					Arguments: "{}",
					Result:    "ok",
					Artifacts: []types.ToolArtifact{
						{Type: "query-result", Name: "result.json", URL: "https://example.test/result.json"},
					},
				},
			},
			{Type: agents.EventTypeDone},
		},
	}

	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)

	var sawDone, sawError bool
	streamErr := svc.SendMessageStream(t.Context(), bichatservices.SendMessageRequest{
		SessionID: session.ID(),
		Content:   "hello",
	}, func(chunk bichatservices.StreamChunk) {
		switch chunk.Type {
		case bichatservices.ChunkTypeDone:
			sawDone = true
		case bichatservices.ChunkTypeError:
			sawError = true
		}
	})
	require.NoError(t, streamErr)

	assert.True(t, sawDone, "done chunk must be emitted despite artifact failure")
	assert.False(t, sawError, "no terminal error chunk when only artifacts fail")

	messages, msgErr := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, msgErr)
	require.Len(t, messages, 2)
	assert.Equal(t, types.RoleAssistant, messages[1].Role())
	assert.Equal(t, "final answer", messages[1].Content())
}

// ambiguousCommitRepo simulates the commit-ambiguity race: the first
// SaveMessage records the row (the COMMIT landed server-side) but reports a
// deadline error (the client never saw the ack). A correct retry must detect
// the already-persisted message via GetMessage and not insert it again — a
// blind re-insert would hit a duplicate key and wrongly discard the answer.
type ambiguousCommitRepo struct {
	*mockChatRepository
	saveCalls int
}

func (r *ambiguousCommitRepo) SaveMessage(ctx context.Context, msg types.Message) error {
	r.saveCalls++
	// Record on the first call so the message becomes queryable, mirroring a
	// server-side commit the client did not observe, then report the deadline.
	if r.saveCalls == 1 {
		_ = r.mockChatRepository.SaveMessage(ctx, msg)
		return context.DeadlineExceeded
	}
	return r.mockChatRepository.SaveMessage(ctx, msg)
}

// A deadline that fires after the message already committed must not trigger a
// duplicate insert on retry: the guard detects the persisted message and
// returns success, so the answer is preserved (#2998 / PR #800 review).
func TestChatService_persistAssistantMessageCritical_RetryIsIdempotent(t *testing.T) {
	t.Parallel()

	baseRepo := newMockChatRepository()
	chatRepo := &ambiguousCommitRepo{mockChatRepository: baseRepo}
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("idempotent retry"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	svc, err := NewChatService(chatRepo, &stubAgentService{}, nil, nil, nil)
	require.NoError(t, err)

	msg := types.NewMessage(
		types.WithMessageID(uuid.New()),
		types.WithSessionID(session.ID()),
		types.WithRole(types.RoleAssistant),
		types.WithContent("final answer"),
	)

	persistErr := svc.persistAssistantMessageCritical(t.Context(), msg, session)

	require.NoError(t, persistErr, "an answer committed before the deadline must not be discarded")
	assert.Equal(t, 1, chatRepo.saveCalls, "retry must skip the insert once the message exists")

	got, getErr := chatRepo.GetMessage(t.Context(), msg.ID())
	require.NoError(t, getErr)
	assert.Equal(t, "final answer", got.Content())
}

// When the critical persist (assistant message) fails, the run terminates with
// an error chunk and the answer is genuinely not persisted — the failure is now
// observable rather than silently swallowed.
func TestChatService_SendMessageStream_CriticalPersistFailureSurfacesError(t *testing.T) {
	t.Parallel()

	baseRepo := newMockChatRepository()
	chatRepo := &failingAssistantSaveRepo{mockChatRepository: baseRepo}
	session := mustSession(t,
		withSessionTenantID(uuid.New()),
		withSessionUserID(1),
		withSessionTitle("critical persist"),
	)
	require.NoError(t, chatRepo.CreateSession(t.Context(), session))

	agentSvc := &stubAgentService{
		processEvents: []agents.ExecutorEvent{
			{Type: agents.EventTypeContent, Content: "answer"},
			{Type: agents.EventTypeDone},
		},
	}

	svc, err := NewChatService(chatRepo, agentSvc, nil, nil, nil)
	require.NoError(t, err)

	var sawError, sawDone bool
	streamErr := svc.SendMessageStream(t.Context(), bichatservices.SendMessageRequest{
		SessionID: session.ID(),
		Content:   "hello",
	}, func(chunk bichatservices.StreamChunk) {
		switch chunk.Type {
		case bichatservices.ChunkTypeError:
			sawError = true
		case bichatservices.ChunkTypeDone:
			sawDone = true
		}
	})

	require.Error(t, streamErr)
	assert.True(t, sawError, "terminal error chunk expected when the answer cannot be persisted")
	assert.False(t, sawDone)

	messages, msgErr := chatRepo.GetSessionMessages(t.Context(), session.ID(), domain.ListOptions{})
	require.NoError(t, msgErr)
	require.Len(t, messages, 1)
	assert.Equal(t, types.RoleUser, messages[0].Role())
}
