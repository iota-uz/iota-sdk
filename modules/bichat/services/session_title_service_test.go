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
	assert.NotEmpty(t, prompt)
}

func TestSessionTitleService_Sanitizer(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Sales report", cleanSessionTitle("  \"**Sales report**\"  "))
	assert.True(t, isValidSessionTitle("Quarterly Revenue Overview"))
	assert.False(t, isValidSessionTitle("title: hello"))
}

func TestSessionTitleService_Sanitizer_TruncatesUnicodeSafely(t *testing.T) {
	t.Parallel()

	longInput := strings.Repeat("А", maxTitleLength+10)
	invalidUTF8Short := string([]byte{0xFF, 0xFE, 0xFD}) // invalid UTF-8, replaced by ToValidUTF8 then short
	cases := []struct {
		name       string
		input      string
		wantSuffix string
	}{
		{"long_unicode_truncated", longInput, "..."},
		{"exactly_max_length", strings.Repeat("Б", maxTitleLength), ""},
		{"exactly_max_length_plus_one", strings.Repeat("В", maxTitleLength+1), "..."},
		{"invalid_utf8", invalidUTF8Short, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleaned := cleanSessionTitle(tc.input)
			assert.True(t, utf8.ValidString(cleaned))
			assert.LessOrEqual(t, utf8.RuneCountInString(cleaned), maxTitleLength)
			if tc.wantSuffix != "" {
				assert.True(t, strings.HasSuffix(cleaned, tc.wantSuffix), "expected truncation suffix %q", tc.wantSuffix)
			}
		})
	}
}

func TestSessionTitleService_Sanitizer_UppercasesFirstRuneInFallback(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		input      string
		wantPrefix string
	}{
		{"cyrillic_lowercase_first", "покажи продажи по регионам", "П"},
		{"already_uppercase", "Покажи продажи по регионам", "П"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fallback := extractFallbackSessionTitle(tc.input)
			assert.True(t, strings.HasPrefix(fallback, tc.wantPrefix))
			assert.True(t, utf8.ValidString(fallback))
		})
	}
}
