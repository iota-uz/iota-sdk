package email

import (
	"context"
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

// sendInput is one call of a new run: its idempotency key (and so the Message-ID the message is sent with)
// is derived from the run, so two runs against one mailbox never send the same message. A test that repeats
// a call reuses the CallInput.
func (f *fx) sendInput(params map[string]any, creds connectors.Credentials) catalog.CallInput {
	run := uuid.New()
	return catalog.CallInput{
		TenantID: tenant, RunID: run, NodeID: "send", Attempt: 1, IdempotencyKey: "run:send:" + run.String() + ":1",
		Params: params, Credentials: creds, Usage: f.usage,
		Connection: &catalog.ConnectionInfo{ID: uuid.NewString(), Kind: KindSMTP, Name: "smtp"},
	}
}

func params(m map[string]any) map[string]any {
	base := map[string]any{"to": []any{"boss@example.org"}, "subject": "Invoice 42", "text": "Hello,\nthe invoice."}
	for k, v := range m {
		base[k] = v
	}
	return base
}

func callError(t *testing.T, err error) *catalog.CallError {
	t.Helper()
	var ce *catalog.CallError
	require.ErrorAs(t, err, &ce)
	return ce
}

func TestSendDeliversWithItsMessageIDBothModes(t *testing.T) {
	for name, mode := range map[string]emailtest.Mode{"tls": emailtest.ModeTLS, "starttls": emailtest.ModeSTARTTLS} {
		t.Run(name, func(t *testing.T) {
			f := newFx(t, mode)
			n := &sendNode{f.smtp.hub}
			in := f.sendInput(params(map[string]any{"cc": []any{"Ann <ann@example.org>"}, "bcc": []any{"audit@example.org"}, "reply_to": "help@example.com"}), f.smtpCreds())
			res, err := n.Execute(context.Background(), in)
			require.NoError(t, err)
			assert.Equal(t, "sent", res.Outcome)
			id := res.Output["message_id"].(string)
			assert.Regexp(t, `^<[0-9a-f]{32}@example\.com>$`, id, "the id is made from the idempotency key and the sender's domain")
			assert.ElementsMatch(t, []any{"boss@example.org", "ann@example.org", "audit@example.org"}, res.Output["accepted"])
			assert.Empty(t, res.Output["rejected"])

			sent := f.srv.Sent()
			require.Len(t, sent, 1)
			assert.Equal(t, mailbox, sent[0].From)
			assert.ElementsMatch(t, []string{"boss@example.org", "ann@example.org", "audit@example.org"}, sent[0].Rcpts)
			raw := string(sent[0].Data)
			assert.Contains(t, raw, "Message-Id: "+id, "the header carries the id the node reported")
			assert.Contains(t, raw, "Subject: Invoice 42")
			assert.Contains(t, raw, "Reply-To: <help@example.com>")
			assert.NotContains(t, strings.ToLower(raw), "bcc", "a blind copy is in the envelope only")

			// the same call (a repeat of its attempt) has the same id; another call has another.
			res2, err := n.Execute(context.Background(), in)
			require.NoError(t, err)
			assert.Equal(t, id, res2.Output["message_id"])
			in.IdempotencyKey = "run:send:tok:2"
			res3, err := n.Execute(context.Background(), in)
			require.NoError(t, err)
			assert.NotEqual(t, id, res3.Output["message_id"])

			require.Len(t, f.usage.entries, 3)
			assert.Equal(t, catalog.UsageEntry{TenantID: tenant, Kind: KindSMTP, ConnectionID: in.Connection.ID, Unit: "message", Quantity: 1, RunID: &in.RunID, At: f.usage.entries[0].At}, f.usage.entries[0])
		})
	}
}

func TestSendHTMLAndThreading(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	n := &sendNode{f.smtp.hub}
	_, err := n.Execute(context.Background(), f.sendInput(params(map[string]any{"html": "<p>Hello <b>you</b></p>", "in_reply_to": "abc@mail.example"}), f.smtpCreds()))
	require.NoError(t, err)
	raw := string(f.srv.Sent()[0].Data)
	assert.Contains(t, raw, "multipart/alternative")
	assert.Contains(t, raw, "text/html")
	assert.Contains(t, raw, "In-Reply-To: <abc@mail.example>")
	assert.Contains(t, raw, "References: <abc@mail.example>")
}

func TestSendRefusals(t *testing.T) {
	ctx := context.Background()
	t.Run("a wrong password is unauthorized and sends nothing", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		creds := f.smtpCreds()
		creds["password"] = "nope"
		_, err := (&sendNode{f.smtp.hub}).Execute(ctx, f.sendInput(params(nil), creds))
		ce := callError(t, err)
		assert.Equal(t, catalog.CallUnauthorized, ce.Code)
		assert.NotContains(t, ce.Message, "nope")
		assert.Contains(t, ce.Message, "535")
		assert.Empty(t, f.srv.Sent())
	})
	t.Run("every recipient refused for good is rejected", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.RejectRecipient("boss@example.org", 550)
		_, err := (&sendNode{f.smtp.hub}).Execute(ctx, f.sendInput(params(nil), f.smtpCreds()))
		assert.Equal(t, catalog.CallRejected, callError(t, err).Code)
		assert.Empty(t, f.srv.Sent())
	})
	t.Run("some recipients refused: the rest is sent, the refused are listed", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.RejectRecipient("gone@example.org", 550)
		res, err := (&sendNode{f.smtp.hub}).Execute(ctx, f.sendInput(params(map[string]any{"to": []any{"boss@example.org", "gone@example.org"}}), f.smtpCreds()))
		require.NoError(t, err)
		assert.Equal(t, "sent", res.Outcome)
		assert.Equal(t, []any{"boss@example.org"}, res.Output["accepted"])
		assert.Equal(t, []any{"gone@example.org"}, res.Output["rejected"])
	})
	t.Run("4xx is unavailable and retryable, and nothing is sent", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.RejectRecipient("boss@example.org", 451)
		_, err := (&sendNode{f.smtp.hub}).Execute(ctx, f.sendInput(params(nil), f.smtpCreds()))
		ce := callError(t, err)
		assert.Equal(t, catalog.CallUnavailable, ce.Code)
		assert.True(t, ce.Retryable)

		f = newFx(t, emailtest.ModeTLS)
		f.srv.FailData(451)
		_, err = (&sendNode{f.smtp.hub}).Execute(ctx, f.sendInput(params(nil), f.smtpCreds()))
		ce = callError(t, err)
		assert.Equal(t, catalog.CallUnavailable, ce.Code)
		assert.True(t, ce.Retryable)
		assert.Empty(t, f.srv.Sent())
	})
	t.Run("a refused message is rejected", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.FailData(554)
		_, err := (&sendNode{f.smtp.hub}).Execute(ctx, f.sendInput(params(nil), f.smtpCreds()))
		assert.Equal(t, catalog.CallRejected, callError(t, err).Code)
	})
	t.Run("a drop after the message was written is outcome_unknown, and it is not sent again", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.DropAfterData(true)
		n := &sendNode{f.smtp.hub}
		_, err := n.Execute(ctx, f.sendInput(params(nil), f.smtpCreds()))
		assert.Equal(t, catalog.CallOutcomeUnknown, callError(t, err).Code)
		assert.False(t, n.Idempotent(), "the engine never repeats the call by itself")
		assert.Len(t, f.srv.Sent(), 1)
		assert.Empty(t, f.usage.entries, "a message that may not have gone is not counted")
	})
	t.Run("a server that cannot be reached is unavailable", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		creds := f.smtpCreds()
		creds["port"] = "1"
		_, err := (&sendNode{f.smtp.hub}).Execute(ctx, f.sendInput(params(nil), creds))
		ce := callError(t, err)
		assert.Equal(t, catalog.CallUnavailable, ce.Code)
	})
	t.Run("addresses are checked before anything is sent", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		for _, p := range []map[string]any{
			{"to": []any{"not an address"}}, {"to": []any{}}, {"reply_to": "x"}, {"in_reply_to": "has space@x"},
		} {
			_, err := (&sendNode{f.smtp.hub}).Execute(ctx, f.sendInput(params(p), f.smtpCreds()))
			assert.Equal(t, catalog.CallInvalidRequest, callError(t, err).Code, "%v", p)
		}
		assert.Empty(t, f.srv.Sent())
	})
	t.Run("no line break gets into a header", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		_, err := (&sendNode{f.smtp.hub}).Execute(ctx, f.sendInput(params(map[string]any{"subject": "hi\r\nBcc: evil@example.org"}), f.smtpCreds()))
		require.NoError(t, err)
		raw := string(f.srv.Sent()[0].Data)
		assert.NotContains(t, raw, "\r\nBcc:")
	})
}

func TestSMTPTestDoesNotSend(t *testing.T) {
	for _, mode := range []emailtest.Mode{emailtest.ModeTLS, emailtest.ModeSTARTTLS} {
		f := newFx(t, mode)
		id, err := f.smtp.Test(context.Background(), f.smtpCreds())
		require.NoError(t, err)
		assert.Equal(t, connectors.Identity{Display: mailbox, External: mailbox, Kind: connectors.IdentityEmail}, id)
		assert.Empty(t, f.srv.Sent())
		assert.Contains(t, f.srv.Mechanisms(), "smtp:PLAIN")
	}
	f := newFx(t, emailtest.ModeTLS)
	creds := f.smtpCreds()
	creds["password"] = "wrong-password"
	_, err := f.smtp.Test(context.Background(), creds)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "wrong-password")
	assert.Contains(t, err.Error(), "status 535")
}

func TestTLSIsRequired(t *testing.T) {
	// A server that greets in the clear and does not offer STARTTLS never gets a login: the connector
	// refuses to talk in the clear. The fake answers EHLO without STARTTLS (not silence the client would
	// time out on), so it is the refusal that is tested, and no AUTH command reaches the server.
	f := newFx(t, emailtest.ModePlain)
	creds := f.smtpCreds()
	creds["security"] = "starttls"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := f.smtp.Test(ctx, creds)
	require.Error(t, err)
	require.NoError(t, ctx.Err(), "refused, not timed out")
	assert.Contains(t, strings.ToLower(err.Error()), "starttls")
	assert.Empty(t, f.srv.Mechanisms(), "no AUTH was attempted")
}

func TestXOAUTH2SendAndRevocation(t *testing.T) {
	ctx := context.Background()
	f := newFx(t, emailtest.ModeTLS)
	f.srv.AcceptToken(mailbox, "ya29.token")
	smtpConn, imapConn := f.connection(KindSMTP), f.connection(KindIMAP)
	creds := f.oauthCreds("", "stale")
	f.conns.add(smtpConn, creds)
	imapCreds := f.oauthCreds("", "stale")
	f.conns.add(imapConn, imapCreds)
	f.oauth.tokens[smtpConn.ID] = "ya29.token"

	in := f.sendInput(params(nil), creds)
	in.Connection.ID = smtpConn.ID.String()
	res, err := (&sendNode{f.smtp.hub}).Execute(ctx, in)
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
	assert.Contains(t, f.srv.Mechanisms(), "smtp:XOAUTH2", "a sign-in authenticates with XOAUTH2, with the token OAuth2.Token gave")
	assert.Equal(t, mailbox, f.srv.Sent()[0].User)
	assert.Equal(t, mailbox, f.srv.Sent()[0].From, "the sender is the mailbox of the sign-in")

	t.Run("a token the server refuses is unauthorized and marks the other connection of the sign-in", func(t *testing.T) {
		f.srv.RevokeTokens(mailbox)
		_, err := (&sendNode{f.smtp.hub}).Execute(ctx, in)
		assert.Equal(t, catalog.CallUnauthorized, callError(t, err).Code)
		assert.Equal(t, connectors.StatusNeedsReauth, f.conns.statusOf(imapConn.ID), "the imap connection shares the login")
	})
	t.Run("a refresh the provider refuses is unauthorized and marks the other connection", func(t *testing.T) {
		f.conns.status[imapConn.ID] = connectors.StatusConnected
		f.oauth.revoke = true
		_, err := (&sendNode{f.smtp.hub}).Execute(ctx, in)
		assert.Equal(t, catalog.CallUnauthorized, callError(t, err).Code)
		assert.Equal(t, connectors.StatusNeedsReauth, f.conns.statusOf(imapConn.ID))
		assert.Equal(t, connectors.StatusNeedsReauth, f.conns.statusOf(smtpConn.ID), "the mailbox's cursor was seeded first, which met the refusal on the receiving side: the whole sign-in is marked")
	})
}

func TestSMTPTestWithToken(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	f.srv.AcceptToken(mailbox, "tok-1")
	creds := f.oauthCreds("", "tok-1")
	id, err := f.smtp.Test(context.Background(), creds)
	require.NoError(t, err)
	assert.Equal(t, mailbox, id.Display)
	creds["access_token"] = "tok-2"
	_, err = f.smtp.Test(context.Background(), creds)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "tok-2")
}
