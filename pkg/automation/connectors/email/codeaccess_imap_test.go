package email

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email/emailtest"
)

func (f *fx) imapCall(op, args string, creds connectors.Credentials) connectors.CodeCall {
	return connectors.CodeCall{Operation: op, Args: json.RawMessage(args), Input: catalog.CallInput{
		TenantID: tenant, RunID: uuid.New(), Credentials: creds,
		Connection: &catalog.ConnectionInfo{ID: uuid.NewString(), Kind: KindIMAP, Name: "inbox"},
	}}
}

func (f *fx) list(t *testing.T, args string) []map[string]any {
	t.Helper()
	out, err := f.imap.CallCodeOperation(context.Background(), f.imapCall(OpListMessages, args, f.imapCreds()))
	require.NoError(t, err)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	var res struct {
		Messages []map[string]any `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(raw, &res))
	return res.Messages
}

func (f *fx) get(t *testing.T, args string) map[string]any {
	t.Helper()
	out, err := f.imap.CallCodeOperation(context.Background(), f.imapCall(OpGetMessage, args, f.imapCreds()))
	require.NoError(t, err)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	var res map[string]any
	require.NoError(t, json.Unmarshal(raw, &res))
	return res
}

func TestCodeListMessages(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	for i := 1; i <= 3; i++ {
		raw := mailText(fmt.Sprintf("<m%d@x>", i), "Ann <ann@example.org>", fmt.Sprintf("Subject %d", i), "body")
		if i == 1 {
			f.srv.DeliverAt(mailbox, "INBOX", raw, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
			continue
		}
		f.srv.Deliver(mailbox, "INBOX", raw)
	}
	f.srv.Flag(mailbox, "INBOX", 2, `\Seen`)

	msgs := f.list(t, `{}`)
	require.Len(t, msgs, 3)
	assert.EqualValues(t, 3, msgs[0]["uid"], "newest first")
	assert.EqualValues(t, 1, msgs[2]["uid"])
	assert.Equal(t, "Subject 3", msgs[0]["subject"])
	assert.Equal(t, "<m3@x>", msgs[0]["message_id"])
	assert.Equal(t, map[string]any{"address": "ann@example.org", "name": "Ann"}, msgs[0]["from"])
	assert.Equal(t, "2026-01-01T10:00:00Z", msgs[0]["date"])
	assert.Equal(t, false, msgs[0]["seen"])
	assert.Equal(t, true, msgs[1]["seen"])
	assert.Positive(t, msgs[0]["size"])
	assert.NotContains(t, msgs[0], "text")

	assert.Len(t, f.list(t, `{"limit":2}`), 2)
	assert.EqualValues(t, 3, f.list(t, `{"limit":1}`)[0]["uid"])
	unseen := f.list(t, `{"unseen":true}`)
	require.Len(t, unseen, 2)
	assert.EqualValues(t, 3, unseen[0]["uid"])
	assert.EqualValues(t, 1, unseen[1]["uid"])
	assert.Len(t, f.list(t, `{"since":"2024-01-01"}`), 3)
	assert.Len(t, f.list(t, `{"since":"2025-06-01T00:00:00Z"}`), 2, "since is the arrival date")
	assert.Empty(t, f.list(t, `{"since":"2099-01-01"}`))

	f.srv.Mailbox(mailbox, "Archive")
	f.srv.Deliver(mailbox, "Archive", mailText("<a1@x>", "b@example.org", "Old", "x"))
	arch := f.list(t, `{"mailbox":"Archive"}`)
	require.Len(t, arch, 1)
	assert.Equal(t, "Old", arch[0]["subject"])
}

func TestCodeGetMessage(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	raw := strings.Join([]string{
		"From: Ann <ann@example.org>", "To: " + mailbox + ", Bob <bob@example.org>", "Cc: cc@example.org", "Subject: Report",
		"Date: Mon, 01 Jan 2026 10:00:00 +0000", "Message-ID: <r1@x>", "MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="B"`, "",
		"--B", `Content-Type: multipart/alternative; boundary="A"`, "",
		"--A", "Content-Type: text/plain; charset=utf-8", "", "plain body",
		"--A", "Content-Type: text/html; charset=utf-8", "", "<p>html <b>body</b></p>",
		"--A--",
		"--B", `Content-Type: application/pdf; name="r.pdf"`, `Content-Disposition: attachment; filename="r.pdf"`,
		"Content-Transfer-Encoding: base64", "", "SGVsbG8=",
		"--B--", ""}, "\r\n")
	uid := f.srv.Deliver(mailbox, "INBOX", []byte(raw))

	res := f.get(t, fmt.Sprintf(`{"uid":%d}`, uid))
	assert.EqualValues(t, uid, res["uid"])
	assert.Equal(t, "<r1@x>", res["message_id"])
	assert.Equal(t, "Report", res["subject"])
	assert.Equal(t, "plain body", res["text"])
	assert.Equal(t, "<p>html <b>body</b></p>", strings.TrimSpace(res["html"].(string)))
	assert.Equal(t, false, res["truncated"])
	assert.Len(t, res["to"], 2)
	assert.Len(t, res["cc"], 1)
	assert.Equal(t, []any{map[string]any{"filename": "r.pdf", "content_type": "application/pdf", "size": float64(5)}}, res["attachments"])
	assert.NotContains(t, res, "data")
}

func TestCodeReadsLeaveMessagesUnseen(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	uid := f.srv.Deliver(mailbox, "INBOX", mailText("<u1@x>", "a@example.org", "Hi", "body"))
	f.list(t, `{}`)
	f.get(t, fmt.Sprintf(`{"uid":%d}`, uid))
	for _, m := range f.srv.Fetch(mailbox, "INBOX") {
		assert.NotContains(t, m.Flags, `\Seen`)
	}
	assert.Len(t, f.list(t, `{"unseen":true}`), 1)
}

func TestCodeGetMessageCapsBodies(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	big := strings.Repeat("é", MaxCodeBody) // 2 bytes each, past the cap in bytes
	raw := strings.Join([]string{"From: a@example.org", "To: " + mailbox, "Subject: Big", "Message-ID: <big@x>", "MIME-Version: 1.0",
		`Content-Type: multipart/alternative; boundary="A"`, "",
		"--A", "Content-Type: text/plain; charset=utf-8", "Content-Transfer-Encoding: 8bit", "", big,
		"--A--", ""}, "\r\n")
	uid := f.srv.Deliver(mailbox, "INBOX", []byte(raw))
	res := f.get(t, fmt.Sprintf(`{"uid":%d}`, uid))
	text := res["text"].(string)
	assert.LessOrEqual(t, len(text), MaxCodeBody)
	assert.NotContains(t, text, "�")
	assert.Equal(t, true, res["truncated"])
}

func TestCodeImapInvalidArgs(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	f.srv.Deliver(mailbox, "INBOX", mailText("<i1@x>", "a@example.org", "Hi", "body"))
	for name, c := range map[string][2]string{
		"limit zero":       {OpListMessages, `{"limit":0}`},
		"limit too big":    {OpListMessages, `{"limit":51}`},
		"limit fraction":   {OpListMessages, `{"limit":1.5}`},
		"bad since":        {OpListMessages, `{"since":"yesterday"}`},
		"unknown field":    {OpListMessages, `{"folder":"x"}`},
		"unseen a string":  {OpListMessages, `{"unseen":"yes"}`},
		"not an object":    {OpListMessages, `[]`},
		"bad mailbox":      {OpListMessages, `{"mailbox":"a\nb"}`},
		"no uid":           {OpGetMessage, `{}`},
		"uid zero":         {OpGetMessage, `{"uid":0}`},
		"uid negative":     {OpGetMessage, `{"uid":-1}`},
		"uid a string":     {OpGetMessage, `{"uid":"1"}`},
		"uid too large":    {OpGetMessage, `{"uid":4294967296}`},
		"get unknown":      {OpGetMessage, `{"uid":1,"seen":true}`},
		"get no arguments": {OpGetMessage, `null`},
	} {
		_, err := f.imap.CallCodeOperation(context.Background(), f.imapCall(c[0], c[1], f.imapCreds()))
		assert.Equal(t, "invalid_args", codeErr(t, err).Code, name)
	}
}

func TestCodeImapRefusals(t *testing.T) {
	ctx := context.Background()
	t.Run("unknown uid", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		_, err := f.imap.CallCodeOperation(ctx, f.imapCall(OpGetMessage, `{"uid":99}`, f.imapCreds()))
		assert.Equal(t, "not_found", codeErr(t, err).Code)
	})
	t.Run("unknown mailbox", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		_, err := f.imap.CallCodeOperation(ctx, f.imapCall(OpListMessages, `{"mailbox":"Nope"}`, f.imapCreds()))
		assert.Equal(t, "not_found", codeErr(t, err).Code)
	})
	t.Run("wrong password", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		creds := f.imapCreds()
		creds["password"] = "nope-secret"
		_, err := f.imap.CallCodeOperation(ctx, f.imapCall(OpListMessages, `{}`, creds))
		ce := codeErr(t, err)
		assert.Equal(t, "provider_refused", ce.Code)
		assert.False(t, ce.Retryable)
		assert.NotContains(t, ce.Error(), "nope-secret")
		assert.NotContains(t, ce.Error(), password)
	})
	t.Run("server down is retryable", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		creds := f.imapCreds()
		f.srv.Close()
		_, err := f.imap.CallCodeOperation(ctx, f.imapCall(OpListMessages, `{}`, creds))
		ce := codeErr(t, err)
		assert.Equal(t, "unavailable", ce.Code)
		assert.True(t, ce.Retryable)
	})
	t.Run("expired deadline", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		call := f.imapCall(OpListMessages, `{}`, f.imapCreds())
		call.Input.Deadline = time.Now().Add(-time.Second)
		_, err := f.imap.CallCodeOperation(ctx, call)
		ce := codeErr(t, err)
		assert.Equal(t, "unavailable", ce.Code)
		assert.True(t, ce.Retryable)
	})
	t.Run("unknown operation", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		_, err := f.imap.CallCodeOperation(ctx, f.imapCall("delete", `{}`, f.imapCreds()))
		assert.Equal(t, "unknown_operation", codeErr(t, err).Code)
	})
}

func TestCodeImapRevokedSignInNeedsReauth(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn := f.connection(KindIMAP)
	creds := f.oauthCreds("", "tok-1")
	f.conns.add(conn, creds)
	f.oauth.revoke = true
	call := f.imapCall(OpListMessages, `{}`, creds)
	call.Input.Connection.ID = conn.ID.String()
	_, err := f.imap.CallCodeOperation(context.Background(), call)
	assert.Equal(t, "provider_refused", codeErr(t, err).Code)
	assert.Equal(t, connectors.StatusNeedsReauth, f.conns.statusOf(conn.ID))
}

func TestCodeImapOAuthRefusedTokenNeedsReauth(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	f.srv.AcceptToken(mailbox, "good-token")
	conn := f.connection(KindIMAP)
	creds := f.oauthCreds("", "stale-token")
	f.conns.add(conn, creds)
	f.oauth.tokens[conn.ID] = "stale-token"
	call := f.imapCall(OpListMessages, `{}`, creds)
	call.Input.Connection.ID = conn.ID.String()
	_, err := f.imap.CallCodeOperation(context.Background(), call)
	ce := codeErr(t, err)
	assert.Equal(t, "provider_refused", ce.Code)
	assert.NotContains(t, ce.Error(), "stale-token")
	assert.Equal(t, connectors.StatusNeedsReauth, f.conns.statusOf(conn.ID))
}
