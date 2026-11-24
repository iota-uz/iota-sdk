package telegram_test

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram/telegramtest"
)

// The contract test: what the connector relies on the Bot API for, run against the in-memory fake
// (CONTRACT_TARGET=fake, the default) and, by hand, against Telegram:
//
//	CONTRACT_TARGET=live TELEGRAM_CONTRACT_TOKEN=<bot token> TELEGRAM_CONTRACT_CHAT=<chat id the bot may write to> \
//	  go test ./pkg/automation/connectors/telegram -run TestBotAPIContract -v
//
// The live run sends a few messages to that chat and reads the bot's webhook; it changes no webhook. It is
// not part of CI.
type target struct {
	apiURL string
	token  string
	chat   int64
	// blocked is a chat the bot may not write to; 0: not checked.
	blocked int64
}

func contractTarget(t *testing.T) target {
	t.Helper()
	switch mode := os.Getenv("CONTRACT_TARGET"); mode {
	case "", "fake":
		api := telegramtest.New()
		t.Cleanup(api.Close)
		api.Chat(telegramtest.Chat{ID: 4242, Type: "private"})
		api.Chat(telegramtest.Chat{ID: 4243, Type: "private"})
		bot := api.Bot(tenantToken, "contract_bot")
		bot.Block(4243)
		return target{apiURL: api.URL(), token: tenantToken, chat: 4242, blocked: 4243}
	case "live":
		token, chat := os.Getenv("TELEGRAM_CONTRACT_TOKEN"), os.Getenv("TELEGRAM_CONTRACT_CHAT")
		if token == "" || chat == "" {
			t.Skip("CONTRACT_TARGET=live needs TELEGRAM_CONTRACT_TOKEN and TELEGRAM_CONTRACT_CHAT")
		}
		id, err := strconv.ParseInt(chat, 10, 64)
		require.NoError(t, err)
		return target{token: token, chat: id}
	default:
		t.Fatalf("CONTRACT_TARGET is fake or live, not %q", mode)
		return target{}
	}
}

func TestBotAPIContract(t *testing.T) {
	tg := contractTarget(t)
	c := telegram.New(telegram.Config{APIURL: tg.apiURL, PublicOrigin: origin})
	ctx := context.Background()
	creds := catalog.Credentials{"token": tg.token}

	t.Run("getMe is the identity", func(t *testing.T) {
		id, err := c.Test(ctx, creds)
		require.NoError(t, err)
		assert.Regexp(t, `^@\w+$`, id.Display)
		assert.NotEmpty(t, id.External)
	})
	t.Run("a refused token is unauthorized", func(t *testing.T) {
		_, err := c.Send(ctx, catalog.ConnectionInfo{}, catalog.Credentials{"token": "1:" + "x0000000000000000000000000000000000"}, telegram.Message{ChatID: tg.chat, Text: "x"})
		var ce *catalog.CallError
		require.ErrorAs(t, err, &ce)
		assert.Equal(t, catalog.CallUnauthorized, ce.Code)
	})
	t.Run("a text and its button reach the chat", func(t *testing.T) {
		ref, err := c.Send(ctx, catalog.ConnectionInfo{}, creds, telegram.Message{
			ChatID: tg.chat, Text: "contract: <b>html</b>", ParseMode: "HTML", Silent: true,
			Buttons: [][]telegram.Button{{{Text: "Open", URL: "https://example.com"}}, {{Text: "Press", CallbackData: "a:AbCdEfGhIjKl:0"}}},
		})
		require.NoError(t, err)
		assert.Equal(t, tg.chat, ref.ChatID)
		assert.NotZero(t, ref.MessageID)
	})
	t.Run("a message Telegram refuses is invalid_request", func(t *testing.T) {
		_, err := c.Send(ctx, catalog.ConnectionInfo{}, creds, telegram.Message{ChatID: tg.chat, Text: "<b>unclosed", ParseMode: "HTML"})
		var ce *catalog.CallError
		if tg.apiURL == "" { // only Telegram parses HTML
			require.ErrorAs(t, err, &ce)
			assert.Equal(t, catalog.CallInvalidRequest, ce.Code)
		}
	})
	t.Run("a chat the bot may not write to is rejected", func(t *testing.T) {
		chat := tg.blocked
		if chat == 0 {
			chat = 1 // a chat that does not exist for the bot
		}
		_, err := c.Send(ctx, catalog.ConnectionInfo{}, creds, telegram.Message{ChatID: chat, Text: "x"})
		var ce *catalog.CallError
		require.ErrorAs(t, err, &ce)
		assert.Equal(t, catalog.CallRejected, ce.Code)
	})
	t.Run("the webhook is readable", func(t *testing.T) {
		_, err := c.Advise(ctx, nil, creds)
		require.NoError(t, err)
	})
}
