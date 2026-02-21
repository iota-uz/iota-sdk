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

func TestSessionTitleService_AutoSkipsExistingTitle(t *testing.T) {
	t.Parallel()

	repo := newMockChatRepository()
	model := newMockModel()
	svc, err := NewTitleGenerationService(model, repo, nil)
	require.NoError(t, err)

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle("Existing title"),
	)
	require.NoError(t, repo.CreateSession(context.Background(), session))
	require.NoError(t, repo.SaveMessage(context.Background(), types.UserMessage("monthly revenue", types.WithSessionID(session.ID()))))

	err = svc.GenerateSessionTitle(context.Background(), session.ID())
	require.NoError(t, err)

	updated, err := repo.GetSession(context.Background(), session.ID())
	require.NoError(t, err)
	assert.Equal(t, "Existing title", updated.Title())
}

func TestSessionTitleService_RegenerateOverwritesExistingTitle(t *testing.T) {
	t.Parallel()

	repo := newMockChatRepository()
	model := newMockModel()
	model.response.Message = types.AssistantMessage("Fresh title")
	svc, err := NewTitleGenerationService(model, repo, nil)
	require.NoError(t, err)

	regenerator, ok := svc.(SessionTitleRegenerationService)
	require.True(t, ok)

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle("Old title"),
	)
	require.NoError(t, repo.CreateSession(context.Background(), session))
	require.NoError(t, repo.SaveMessage(context.Background(), types.UserMessage("monthly revenue", types.WithSessionID(session.ID()))))
	require.NoError(t, repo.SaveMessage(context.Background(), types.AssistantMessage("sure", types.WithSessionID(session.ID()))))

	err = regenerator.RegenerateSessionTitle(context.Background(), session.ID())
	require.NoError(t, err)

	updated, err := repo.GetSession(context.Background(), session.ID())
	require.NoError(t, err)
	assert.Equal(t, "Fresh title", updated.Title())
}

func TestSessionTitleService_RenderPrompt(t *testing.T) {
	t.Parallel()

	prompt, err := renderSessionTitlePrompt("user asks", "assistant answers")
	require.NoError(t, err)
	assert.Contains(t, prompt, "User's first message:")
	assert.Contains(t, prompt, "user asks")
	assert.Contains(t, prompt, "Assistant's response:")
	assert.Contains(t, prompt, "assistant answers")
}

func TestSessionTitleService_Sanitizer(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Sales report", cleanSessionTitle("  \"**Sales report**\"  "))
	assert.True(t, isValidSessionTitle("Quarterly Revenue Overview"))
	assert.False(t, isValidSessionTitle("title: hello"))
}
