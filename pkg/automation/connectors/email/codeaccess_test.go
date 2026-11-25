package email

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email/emailtest"
)

func (f *fx) codeCall(args string, creds connectors.Credentials) connectors.CodeCall {
	return connectors.CodeCall{Operation: OpSend, Args: json.RawMessage(args), Input: catalog.CallInput{
		TenantID: tenant, RunID: uuid.New(), Credentials: creds, Usage: f.usage,
		Connection: &catalog.ConnectionInfo{ID: uuid.NewString(), Kind: KindSMTP, Name: "smtp"},
	}}
}

func codeErr(t *testing.T, err error) *connectors.CodeOperationError {
	t.Helper()
	var ce *connectors.CodeOperationError
	require.ErrorAs(t, err, &ce)
	return ce
}

func TestCodeAccessRegistersInADirectory(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	require.NoError(t, connectors.NewDirectory(nil).Register(f.smtp))
	assert.Equal(t, []string{"send"}, f.smtp.CodeAccess().OperationNames())
	require.NoError(t, connectors.NewDirectory(nil).Register(f.imap))
	assert.Equal(t, []string{"listMessages", "getMessage"}, f.imap.CodeAccess().OperationNames())
	_, isOperator := any(f.imap).(connectors.CodeOperator)
	assert.True(t, isOperator)
}

func TestCodeSendDeliversThroughTheConnection(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	out, err := f.smtp.CallCodeOperation(context.Background(), f.codeCall(
		`{"to":"boss@example.org","cc":["Ann <ann@example.org>"],"subject":"Invoice 42","text":"hello","html":"<b>hello</b>","reply_to":"help@example.com"}`, f.smtpCreds()))
	require.NoError(t, err)
	res := out.(map[string]any)
	assert.Regexp(t, `^<[0-9a-f]{32}@example\.com>$`, res["message_id"])
	assert.ElementsMatch(t, []any{"boss@example.org", "ann@example.org"}, res["accepted"])
	sent := f.srv.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, mailbox, sent[0].From, "the sender is the connection's")
	raw := string(sent[0].Data)
	assert.Contains(t, raw, "Subject: Invoice 42")
	assert.Contains(t, raw, "text/html")
	assert.Len(t, f.usage.entries, 1)
	assert.NotContains(t, raw+res["message_id"].(string), password)

	// A second call is another message.
	out2, err := f.smtp.CallCodeOperation(context.Background(), f.codeCall(`{"to":["boss@example.org"],"subject":"x","text":"y"}`, f.smtpCreds()))
	require.NoError(t, err)
	assert.NotEqual(t, res["message_id"], out2.(map[string]any)["message_id"])
}

func TestCodeSendInvalidArgs(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	for name, args := range map[string]string{
		"no to":                    `{"subject":"s","text":"t"}`,
		"no subject":               `{"to":"a@example.org","text":"t"}`,
		"no body":                  `{"to":"a@example.org","subject":"s"}`,
		"bad address":              `{"to":"not-an-address","subject":"s","text":"t"}`,
		"bad reply_to":             `{"to":"a@example.org","subject":"s","text":"t","reply_to":"nope"}`,
		"unknown field":            `{"to":"a@example.org","subject":"s","text":"t","from":"x@y.z"}`,
		"to a number":              `{"to":5,"subject":"s","text":"t"}`,
		"not an object":            `[]`,
		"long subject":             `{"to":"a@example.org","subject":"` + strings.Repeat("s", 999) + `","text":"t"}`,
		"long subject in Cyrillic": `{"to":"a@example.org","subject":"` + strings.Repeat("ж", 999) + `","text":"t"}`,
	} {
		_, err := f.smtp.CallCodeOperation(context.Background(), f.codeCall(args, f.smtpCreds()))
		assert.Equal(t, "invalid_args", codeErr(t, err).Code, name)
	}
	assert.Empty(t, f.srv.Sent())
}

func TestCodeSendRefusals(t *testing.T) {
	ctx := context.Background()
	t.Run("wrong password", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		creds := f.smtpCreds()
		creds["password"] = "nope-secret"
		_, err := f.smtp.CallCodeOperation(ctx, f.codeCall(`{"to":"a@example.org","subject":"s","text":"t"}`, creds))
		ce := codeErr(t, err)
		assert.Equal(t, "provider_refused", ce.Code)
		assert.NotContains(t, ce.Error(), "nope-secret")
	})
	t.Run("temporary refusal is retryable", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.RejectRecipient("boss@example.org", 451)
		_, err := f.smtp.CallCodeOperation(ctx, f.codeCall(`{"to":"boss@example.org","subject":"s","text":"t"}`, f.smtpCreds()))
		ce := codeErr(t, err)
		assert.Equal(t, "unavailable", ce.Code)
		assert.True(t, ce.Retryable)
	})
	t.Run("every recipient refused", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.RejectRecipient("boss@example.org", 550)
		_, err := f.smtp.CallCodeOperation(ctx, f.codeCall(`{"to":"boss@example.org","subject":"s","text":"t"}`, f.smtpCreds()))
		assert.Equal(t, "provider_refused", codeErr(t, err).Code)
	})
	t.Run("unknown operation", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		call := f.codeCall(`{}`, f.smtpCreds())
		call.Operation = "nope"
		_, err := f.smtp.CallCodeOperation(ctx, call)
		assert.Equal(t, "unknown_operation", codeErr(t, err).Code)
	})
}

// The limits count characters: a subject of 998 Cyrillic letters (1996 bytes) is sent.
func TestCodeSendCountsCharacters(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	_, err := f.smtp.CallCodeOperation(context.Background(), f.codeCall(
		`{"to":"boss@example.org","subject":"`+strings.Repeat("ж", 998)+`","text":"`+strings.Repeat("ю", 2000)+`"}`, f.smtpCreds()))
	require.NoError(t, err)
}
