package telegramchannel_test

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/notifications/telegramchannel"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram/telegramtest"
)

const token = "123456789:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// connections answers the connection of the channel per tenant.
type connections map[uuid.UUID]string

func (c connections) Connection(_ context.Context, tenantID uuid.UUID, _ notifications.ChannelSpec) (string, bool, error) {
	ref, ok := c[tenantID]
	return ref, ok, nil
}

// credentials opens the platform bot, or a tenant connection by its ID.
type credentials struct{ err error }

func (c credentials) Open(_ context.Context, _ uuid.UUID, ref string) (catalog.ConnectionInfo, connectors.Credentials, error) {
	if c.err != nil {
		return catalog.ConnectionInfo{}, nil, c.err
	}
	return catalog.ConnectionInfo{ID: ref, Kind: telegram.Kind, Platform: strings.HasPrefix(ref, "platform:")}, connectors.Credentials{"token": token}, nil
}

type fixture struct {
	srv     *telegramtest.Server
	bot     *telegramtest.Bot
	channel *telegramchannel.Channel
	tenant  uuid.UUID
}

func setup(t *testing.T, creds credentials) fixture {
	t.Helper()
	srv := telegramtest.New()
	t.Cleanup(srv.Close)
	bot := srv.Bot(token, "acme_bot")
	srv.Chat(telegramtest.Chat{ID: 42})
	tg := telegram.New(telegram.Config{APIURL: srv.URL()})
	tenant := uuid.New()
	ch := telegramchannel.New(telegramchannel.Config{
		Client: tg, Bots: tg, Credentials: creds, Connections: connections{tenant: "platform:telegram_bot"},
		PublicOrigin: "https://erp.example.com/",
	})
	return fixture{srv: srv, bot: bot, channel: ch, tenant: tenant}
}

func TestSpec(t *testing.T) {
	f := setup(t, credentials{})
	spec := f.channel.Spec()
	require.NoError(t, spec.Validate())
	assert.Equal(t, notifications.ChannelTelegram, spec.Channel)
	assert.Equal(t, notifications.AddressLinked, spec.Address)
	assert.Equal(t, "telegram_bot", spec.ConnectorKind)
	assert.False(t, spec.OptIn, "telegram is not opt-in: without a link there is no address")
}

func TestSend_RendersHTMLWithButton(t *testing.T) {
	f := setup(t, credentials{})
	receipt, err := f.channel.Send(context.Background(), notifications.Message{
		TenantID: f.tenant, Address: "42", Language: "ru",
		Title: "Счёт <№1>", Body: "Оплатите & подтвердите", ActionURL: "/finance/invoices/1",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, receipt.ProviderMessageID)
	sent := f.bot.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, "<b>Счёт &lt;№1&gt;</b>\n\nОплатите &amp; подтвердите", sent[0].Text)
	assert.Equal(t, "HTML", sent[0].Parse)
	require.Len(t, sent[0].Markup, 1)
	assert.Equal(t, "Открыть", sent[0].Markup[0][0].Text)
	assert.Equal(t, "https://erp.example.com/finance/invoices/1", sent[0].Markup[0][0].URL)
}

func TestRender_CutsTo4096WithEllipsis(t *testing.T) {
	text := telegramchannel.Render("Title", strings.Repeat("<&>", 3000))
	assert.LessOrEqual(t, utf8.RuneCountInString(text), 4096)
	assert.True(t, strings.HasSuffix(text, "…"))
	assert.NotContains(t, text[len(text)-12:], "&am…", "an entity is never cut in half")
	long := telegramchannel.Render(strings.Repeat("т", 5000), "body")
	assert.LessOrEqual(t, utf8.RuneCountInString(long), 4096)
	assert.True(t, strings.HasSuffix(long, "…</b>"))
}

func TestSend_ErrorsMapToDeliverySemantics(t *testing.T) {
	ctx := context.Background()
	t.Run("blocked bot is a permanent loss of the target", func(t *testing.T) {
		f := setup(t, credentials{})
		f.bot.Block(42)
		_, err := f.channel.Send(ctx, notifications.Message{TenantID: f.tenant, Address: "42", Title: "x"})
		require.ErrorIs(t, err, notifications.ErrPermanent)
		require.ErrorIs(t, err, notifications.ErrTargetGone)
	})
	t.Run("chat not found is a permanent loss of the target", func(t *testing.T) {
		f := setup(t, credentials{})
		f.bot.Fail(telegramtest.Failure{Method: "sendMessage", Code: 400, Description: "Bad Request: chat not found"})
		_, err := f.channel.Send(ctx, notifications.Message{TenantID: f.tenant, Address: "42", Title: "x"})
		require.ErrorIs(t, err, notifications.ErrTargetGone)
	})
	t.Run("429 is retried after retry_after", func(t *testing.T) {
		f := setup(t, credentials{})
		f.bot.Fail(telegramtest.Failure{Method: "sendMessage", Code: 429, Description: "Too Many Requests", RetryAfter: 7})
		_, err := f.channel.Send(ctx, notifications.Message{TenantID: f.tenant, Address: "42", Title: "x"})
		var later *notifications.RetryAfterError
		require.ErrorAs(t, err, &later)
		assert.Equal(t, 7*time.Second, later.After)
		assert.NotErrorIs(t, err, notifications.ErrPermanent)
	})
	t.Run("5xx is retried", func(t *testing.T) {
		f := setup(t, credentials{})
		f.bot.Fail(telegramtest.Failure{Method: "sendMessage", Code: 502, Description: "Bad Gateway"})
		_, err := f.channel.Send(ctx, notifications.Message{TenantID: f.tenant, Address: "42", Title: "x"})
		require.Error(t, err)
		require.NotErrorIs(t, err, notifications.ErrPermanent)
		require.NotErrorIs(t, err, notifications.ErrAmbiguous)
	})
	t.Run("a connection that was never made is retried", func(t *testing.T) {
		f := setup(t, credentials{})
		srv := telegramtest.New()
		srv.Close() // nothing listens: the connection is never made
		tg := telegram.New(telegram.Config{APIURL: srv.URL()})
		ch := telegramchannel.New(telegramchannel.Config{Client: tg, Credentials: credentials{}, Connections: connections{f.tenant: "platform:telegram_bot"}})
		_, err := ch.Send(ctx, notifications.Message{TenantID: f.tenant, Address: "42", Title: "x"})
		require.Error(t, err)
		assert.NotErrorIs(t, err, notifications.ErrAmbiguous, "a refused connection did not send anything")
	})
	t.Run("a tenant without a connection has no channel", func(t *testing.T) {
		f := setup(t, credentials{})
		_, err := f.channel.Send(ctx, notifications.Message{TenantID: uuid.New(), Address: "42", Title: "x"})
		require.ErrorIs(t, err, notifications.ErrChannelUnavailable)
	})
	t.Run("a connection that stopped working is no channel", func(t *testing.T) {
		f := setup(t, credentials{err: connectors.ErrNeedsReauth})
		_, err := f.channel.Send(ctx, notifications.Message{TenantID: f.tenant, Address: "42", Title: "x"})
		require.ErrorIs(t, err, notifications.ErrChannelUnavailable)
	})
	t.Run("an address that is not a chat id is permanent", func(t *testing.T) {
		f := setup(t, credentials{})
		_, err := f.channel.Send(ctx, notifications.Message{TenantID: f.tenant, Address: "@ada", Title: "x"})
		require.ErrorIs(t, err, notifications.ErrPermanent)
	})
}

func TestLinkURL_NamesTheBot(t *testing.T) {
	f := setup(t, credentials{})
	url, err := f.channel.LinkURL(context.Background(), f.tenant, "platform:telegram_bot", "tok")
	require.NoError(t, err)
	assert.Equal(t, "https://t.me/acme_bot?start=u_tok", url)
	_, err = setup(t, credentials{err: connectors.ErrNotFound}).channel.LinkURL(context.Background(), f.tenant, "x", "tok")
	require.ErrorIs(t, err, notifications.ErrChannelUnavailable)
}
