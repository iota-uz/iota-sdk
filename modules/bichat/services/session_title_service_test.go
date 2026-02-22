package services

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

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
	svc, err := NewSessionTitleService(model, repo, nil)
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
	svc, err := NewSessionTitleService(model, repo, nil)
	require.NoError(t, err)

	session := domain.NewSession(
		domain.WithTenantID(uuid.New()),
		domain.WithUserID(1),
		domain.WithTitle("Old title"),
	)
	require.NoError(t, repo.CreateSession(context.Background(), session))
	require.NoError(t, repo.SaveMessage(context.Background(), types.UserMessage("monthly revenue", types.WithSessionID(session.ID()))))
	require.NoError(t, repo.SaveMessage(context.Background(), types.AssistantMessage("sure", types.WithSessionID(session.ID()))))

	err = svc.RegenerateSessionTitle(context.Background(), session.ID())
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

func TestSessionTitleService_Sanitizer_TruncatesUnicodeSafely(t *testing.T) {
	t.Parallel()

	input := "Аналитика продаж по регионам и страховым продуктам за длительный период времени"
	cleaned := cleanSessionTitle(input)

	assert.True(t, utf8.ValidString(cleaned))
	assert.LessOrEqual(t, utf8.RuneCountInString(cleaned), maxTitleLength)
}

func TestSessionTitleService_Sanitizer_UppercasesFirstRuneInFallback(t *testing.T) {
	t.Parallel()

	fallback := extractFallbackSessionTitle("покажи продажи по регионам")
	assert.True(t, strings.HasPrefix(fallback, "П"))
	assert.True(t, utf8.ValidString(fallback))
}
