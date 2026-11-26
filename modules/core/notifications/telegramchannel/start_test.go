package telegramchannel_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/notifications/telegramchannel"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
)

// linker is a ChannelLinkService over one token.
type linker struct {
	tenant     uuid.UUID
	token      string
	connection string
	language   string
	used       string // the update key that used the token
	completed  []string
	fail       error
}

func (l *linker) StartLink(context.Context, notifications.Channel) (notifications.LinkStart, error) {
	return notifications.LinkStart{}, nil
}
func (l *linker) CompleteLink(ctx context.Context, connectionID, token, address, display string) error {
	_, err := l.CompleteLinkUpdate(ctx, connectionID, token, address, display, "")
	return err
}
func (l *linker) TenantOf(_ context.Context, token string) (uuid.UUID, bool, error) {
	if token == l.token && l.used == "" {
		return l.tenant, true, nil
	}
	return uuid.Nil, false, nil
}
func (l *linker) Unlink(context.Context, notifications.Channel) error { return nil }
func (l *linker) CompleteLinkUpdate(_ context.Context, connectionID, token, address, display, updateKey string) (notifications.LinkCompletion, error) {
	if l.fail != nil {
		return notifications.LinkCompletion{}, l.fail
	}
	if token != l.token || connectionID != l.connection {
		return notifications.LinkCompletion{}, notifications.ErrLinkTokenInvalid
	}
	if l.used != "" {
		if l.used == updateKey {
			return notifications.LinkCompletion{TenantID: l.tenant, Replayed: true}, nil
		}
		return notifications.LinkCompletion{}, notifications.ErrLinkTokenInvalid
	}
	l.used = updateKey
	l.completed = append(l.completed, address+" "+display)
	return notifications.LinkCompletion{TenantID: l.tenant}, nil
}
func (l *linker) TenantOfUpdate(_ context.Context, updateKey string) (uuid.UUID, bool, error) {
	return l.tenant, updateKey != "" && updateKey == l.used, nil
}
func (l *linker) LanguageOf(_ context.Context, token string) string {
	if token == l.token {
		return l.language
	}
	return ""
}

func update(payload, chatType, key string) telegram.StartUpdate {
	return telegram.StartUpdate{ConnectionID: "platform:telegram_bot", ChatID: 42, UserID: 42, ChatType: chatType, Username: "ada", Payload: payload, UpdateKey: key}
}

func TestStartHandler_LinksOnceAndAnswersInTheUsersLanguage(t *testing.T) {
	ctx := context.Background()
	l := &linker{tenant: uuid.New(), token: "tok", connection: "platform:telegram_bot", language: "uz-Cyrl"}
	h := telegramchannel.NewTelegramStartHandler(l)
	assert.Equal(t, "u_", h.Prefix())
	tenant, ok, err := h.TenantOf(ctx, "u_tok")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, l.tenant, tenant)

	reply, err := h.Handle(ctx, update("u_tok", "private", "1:100"))
	require.NoError(t, err)
	assert.Equal(t, "Telegram уланди. Билдиришномалар шу чатга келади.", reply)
	assert.Equal(t, []string{"42 @ada"}, l.completed)

	// The redelivery of the same update gets the same answer and changes nothing.
	reply, err = h.Handle(ctx, update("u_tok", "private", "1:100"))
	require.NoError(t, err)
	assert.Contains(t, reply, "уланди")
	assert.Len(t, l.completed, 1)

	// Another /start with the used link: «the link has expired», nothing changes.
	reply, err = h.Handle(ctx, update("u_tok", "private", "1:101"))
	require.NoError(t, err)
	assert.Equal(t, "Ҳавола эскирган. Профилингизда янгисини олинг.", reply)
	assert.Len(t, l.completed, 1)
	got, ok, err := h.TenantOfUpdate(ctx, "1:100")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, l.tenant, got)
}

func TestStartHandler_IgnoresGroupsAndRefusesOtherConnections(t *testing.T) {
	ctx := context.Background()
	l := &linker{tenant: uuid.New(), token: "tok", connection: "platform:telegram_bot", language: "ru"}
	h := telegramchannel.NewTelegramStartHandler(l)
	reply, err := h.Handle(ctx, update("u_tok", "group", "1:1"))
	require.NoError(t, err)
	assert.Empty(t, reply, "a /start from a group is ignored")
	assert.Empty(t, l.completed)

	other := update("u_tok", "private", "2:1")
	other.ConnectionID = uuid.NewString() // another tenant's bot
	reply, err = h.Handle(ctx, other)
	require.NoError(t, err)
	assert.Equal(t, "Ссылка устарела. Получите новую в профиле.", reply)
	assert.Empty(t, l.completed)
}

func TestStartHandler_UnknownTokenAndFailures(t *testing.T) {
	ctx := context.Background()
	l := &linker{tenant: uuid.New(), token: "tok", connection: "platform:telegram_bot"}
	h := telegramchannel.NewTelegramStartHandler(l).WithLanguage("ru")
	reply, err := h.Handle(ctx, update("u_nope", "private", "1:1"))
	require.NoError(t, err)
	assert.Equal(t, "Ссылка устарела. Получите новую в профиле.", reply, "an unknown token is answered in the default language, without ids")

	l.fail = errors.New("db down")
	_, err = h.Handle(ctx, update("u_tok", "private", "1:2"))
	require.Error(t, err, "a failure is returned so Telegram redelivers the update")
}

func TestPhrases_AllLanguages(t *testing.T) {
	ctx := context.Background()
	for lang, want := range map[string]string{
		"en":      "Telegram is connected. Notifications will arrive in this chat.",
		"ru":      "Telegram подключён. Уведомления будут приходить в этот чат.",
		"uz":      "Telegram ulandi. Bildirishnomalar shu chatga keladi.",
		"uz-Cyrl": "Telegram уланди. Билдиришномалар шу чатга келади.",
		"zh":      "Telegram is connected. Notifications will arrive in this chat.",
	} {
		l := &linker{tenant: uuid.New(), token: "tok", connection: "platform:telegram_bot", language: lang}
		reply, err := telegramchannel.NewTelegramStartHandler(l).Handle(ctx, update("u_tok", "private", "1:"+lang))
		require.NoError(t, err)
		assert.Equal(t, want, reply, lang)
	}
}
