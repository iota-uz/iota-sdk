package telegram_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram/telegramtest"
)

func (r *rig) codeCall(op, args string) connectors.CodeCall {
	return connectors.CodeCall{
		Operation: op, Args: json.RawMessage(args),
		Input: catalog.CallInput{
			TenantID: r.tenant, RunID: uuid.New(), Credentials: r.creds(),
			Connection: &catalog.ConnectionInfo{ID: r.conn.ID.String(), Kind: telegram.Kind, Name: "bot"},
		},
	}
}

func TestCodeAccessRegistersInADirectory(t *testing.T) {
	r := newRig(t)
	dir := connectors.NewDirectory(nil)
	require.NoError(t, dir.Register(r.c))
	access := r.c.CodeAccess()
	assert.Equal(t, []string{"sendMessage", "getMe"}, access.OperationNames())
	assert.Empty(t, access.Methods, "the token is in Telegram's URL: no HTTP methods")
	assert.False(t, access.Fetch)
	assert.NoError(t, r.c.Authorize(context.Background(), r.creds(), http.Header{}))
}

func TestCodeSendMessage(t *testing.T) {
	r := newRig(t)
	r.api.Chat(telegramtest.Chat{ID: 5001, Title: "ops"})
	out, err := r.c.CallCodeOperation(context.Background(), r.codeCall("sendMessage", `{"chat_id":5001,"text":"hello"}`))
	require.NoError(t, err)
	res := out.(map[string]any)
	assert.EqualValues(t, 5001, res["chat_id"])
	assert.NotZero(t, res["message_id"])
	require.Len(t, r.bot.Sent(), 1)
	assert.Equal(t, "hello", r.bot.Sent()[0].Text)
}

func TestCodeSendMessageInvalidArgs(t *testing.T) {
	r := newRig(t)
	for name, args := range map[string]string{
		"no chat":       `{"text":"x"}`,
		"empty text":    `{"chat_id":1,"text":" "}`,
		"parse mode":    `{"chat_id":1,"text":"x","parse_mode":"Markdown"}`,
		"unknown field": `{"chat_id":1,"text":"x","token":"y"}`,
		"not an object": `"hi"`,
	} {
		_, err := r.c.CallCodeOperation(context.Background(), r.codeCall("sendMessage", args))
		var ce *connectors.CodeOperationError
		require.ErrorAs(t, err, &ce, name)
		assert.Equal(t, "invalid_args", ce.Code, name)
	}
	assert.Empty(t, r.bot.Sent())
}

func TestCodeSendMessageRefusals(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	_, err := r.c.CallCodeOperation(ctx, r.codeCall("sendMessage", `{"chat_id":777,"text":"x"}`))
	var ce *connectors.CodeOperationError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, "provider_refused", ce.Code, "a chat the bot cannot reach")

	r.bot.Fail(telegramtest.Failure{Method: "sendMessage", Code: 429, Description: "Too Many Requests", RetryAfter: 3})
	r.api.Chat(telegramtest.Chat{ID: 5002})
	_, err = r.c.CallCodeOperation(ctx, r.codeCall("sendMessage", `{"chat_id":5002,"text":"x"}`))
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, "rate_limited", ce.Code)
	assert.True(t, ce.Retryable)

	r.bot.Fail(telegramtest.Failure{Method: "sendMessage", Code: 502, Description: "Bad Gateway"})
	_, err = r.c.CallCodeOperation(ctx, r.codeCall("sendMessage", `{"chat_id":5002,"text":"x"}`))
	require.Error(t, err)
	assert.NotErrorAs(t, err, &ce, "an unknown outcome is a plain error")
	assert.NotContains(t, err.Error(), tenantToken)
}

func TestCodeGetMe(t *testing.T) {
	r := newRig(t)
	out, err := r.c.CallCodeOperation(context.Background(), r.codeCall("getMe", `null`))
	require.NoError(t, err)
	me := out.(map[string]any)
	assert.Equal(t, "tenant_bot", me["username"])
	assert.EqualValues(t, 111111, me["id"])
	raw, err := json.Marshal(me)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "AAAAAAAA", "no token in the result")
}

func TestCodeOperationNeedsAToken(t *testing.T) {
	r := newRig(t)
	call := r.codeCall("getMe", ``)
	call.Input.Credentials = connectors.Credentials{}
	_, err := r.c.CallCodeOperation(context.Background(), call)
	var ce *connectors.CodeOperationError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, "provider_refused", ce.Code)
}
