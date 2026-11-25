package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email/emailtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// pollOnce is one poll of the connection with the cursor the previous one returned.
func (f *fx) pollOnce(t *testing.T, conn connectors.Connection, creds connectors.Credentials, cur json.RawMessage) ([]engine.InboundEvent, json.RawMessage) {
	t.Helper()
	evs, next, err := f.imap.Poll(context.Background(), conn, creds, cur)
	require.NoError(t, err)
	return evs, next
}

func payloadOf(t *testing.T, ev engine.InboundEvent) map[string]any {
	t.Helper()
	var p map[string]any
	require.NoError(t, json.Unmarshal(ev.Payload, &p))
	return p
}

func TestFirstPollSetsTheCursorAndPublishesNothing(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	f.srv.Deliver(mailbox, "INBOX", mailText("<old1@x>", "a@example.org", "old", "before the connection"))
	f.srv.Deliver(mailbox, "INBOX", mailText("<old2@x>", "a@example.org", "old", "before the connection"))
	conn, creds := f.connection(KindIMAP), f.imapCreds()

	evs, cur := f.pollOnce(t, conn, creds, nil)
	assert.Empty(t, evs, "old mail never starts a graph")
	var c cursor
	require.NoError(t, json.Unmarshal(cur, &c))
	assert.Equal(t, "INBOX", c.Mailbox)
	assert.Equal(t, uint32(2), c.LastUID)
	assert.NotZero(t, c.UIDValidity)

	evs, cur2 := f.pollOnce(t, conn, creds, cur)
	assert.Empty(t, evs)
	assert.JSONEq(t, string(cur), string(cur2), "nothing new: the cursor stays")

	f.srv.Deliver(mailbox, "INBOX", mailText("<new1@x>", "Ann <ann@example.org>", "Question", "Is it ready?"))
	evs, cur3 := f.pollOnce(t, conn, creds, cur2)
	require.Len(t, evs, 1)
	ev := evs[0]
	assert.Equal(t, EventReceived, ev.Event)
	assert.Equal(t, "<new1@x>", ev.DedupKey)
	assert.Equal(t, engine.SubjectRef{Entity: graph.SubjectConversation, Key: conn.ID.String() + ":<new1@x>"}, ev.Subject, "a message with no references is the root of its thread")
	p := payloadOf(t, ev)
	assert.Equal(t, "Question", p["subject"])
	assert.Equal(t, "Is it ready?", p["text"])
	assert.Equal(t, map[string]any{"address": "ann@example.org", "name": "Ann"}, p["from"])
	assert.Equal(t, conn.ID.String(), p["connectionId"])
	assert.Equal(t, "INBOX", p["mailbox"])
	assert.Equal(t, false, p["isReply"])
	assert.Equal(t, false, p["isAuto"])
	assert.Equal(t, false, p["hasHtml"])

	evs, _ = f.pollOnce(t, conn, creds, cur3)
	assert.Empty(t, evs, "the cursor is past what was published")
}

func TestUIDValidityChangeStartsOver(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	f.srv.Deliver(mailbox, "INBOX", mailText("<a@x>", "a@example.org", "one", "x"))
	_, cur := f.pollOnce(t, conn, creds, nil)

	// The mailbox is rebuilt: its UIDs mean other messages now, and what it holds arrived before this poll.
	f.srv.ResetMailbox(mailbox, "INBOX")
	f.srv.Deliver(mailbox, "INBOX", mailText("<b@x>", "a@example.org", "two", "x"))
	f.srv.Deliver(mailbox, "INBOX", mailText("<c@x>", "a@example.org", "three", "x"))
	evs, cur2 := f.pollOnce(t, conn, creds, cur)
	assert.Empty(t, evs, "after UIDVALIDITY changed the old messages do not start graphs")
	var before, after cursor
	require.NoError(t, json.Unmarshal(cur, &before))
	require.NoError(t, json.Unmarshal(cur2, &after))
	assert.NotEqual(t, before.UIDValidity, after.UIDValidity)
	assert.Equal(t, uint32(2), after.LastUID)

	f.srv.Deliver(mailbox, "INBOX", mailText("<d@x>", "a@example.org", "four", "x"))
	evs, _ = f.pollOnce(t, conn, creds, cur2)
	require.Len(t, evs, 1)
	assert.Equal(t, "<d@x>", evs[0].DedupKey)
}

func TestAnotherMailboxStartsOver(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	f.srv.Mailbox(mailbox, "Support")
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	f.srv.Deliver(mailbox, "Support", mailText("<s1@x>", "a@example.org", "old", "x"))
	_, cur := f.pollOnce(t, conn, creds, nil)
	creds["mailbox"] = "Support"
	evs, cur2 := f.pollOnce(t, conn, creds, cur)
	assert.Empty(t, evs, "a connection pointed at another mailbox does not publish its old mail")
	var c cursor
	require.NoError(t, json.Unmarshal(cur2, &c))
	assert.Equal(t, "Support", c.Mailbox)
}

func TestReplyBelongsToTheThreadOfItsFirstReference(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)
	sentID := "<abc123@example.com>"
	f.srv.Deliver(mailbox, "INBOX", mailText("<r1@gmail>", "Ann <ann@example.org>", "Re: Invoice 42", "Paid.", "In-Reply-To: <abc123@example.com>", "References: <abc123@example.com>"))
	f.srv.Deliver(mailbox, "INBOX", mailText("<r2@gmail>", "bob@example.org", "Re: Re: Invoice 42", "Me too.", "In-Reply-To: <r1@gmail>", "References: <abc123@example.com> <r1@gmail>"))
	f.srv.Deliver(mailbox, "INBOX", mailText("<r3@gmail>", "bob@example.org", "No references", "x", "In-Reply-To: <abc123@example.com>"))
	f.srv.Deliver(mailbox, "INBOX", mailText("<other@gmail>", "bob@example.org", "Re: Something else", "x", "In-Reply-To: <zzz@example.com>", "References: <zzz@example.com>"))
	evs, _ := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 4)
	for _, ev := range evs[:3] {
		assert.Equal(t, ConversationKey(conn.ID.String(), sentID), ev.Subject.Key, "the first of References, else In-Reply-To")
	}
	assert.NotEqual(t, ConversationKey(conn.ID.String(), sentID), evs[3].Subject.Key, "a reply to another message does not wake the wait on this one")
	p := payloadOf(t, evs[1])
	assert.Equal(t, []any{"<abc123@example.com>", "<r1@gmail>"}, p["references"])
	assert.Equal(t, "<r1@gmail>", p["inReplyTo"])
	assert.Equal(t, sentID, p["threadId"])
	assert.Equal(t, true, p["isReply"])
}

func TestDedupKeyFallsBackToPosition(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)
	f.srv.Deliver(mailbox, "INBOX", mailText("", "a@example.org", "no id", "x"))
	evs, _ := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 1)
	assert.Regexp(t, `^[0-9]+:1$`, evs[0].DedupKey, "uidvalidity:uid when the message has no Message-ID")
	assert.Regexp(t, `^`+conn.ID.String()+`:[0-9]+:1$`, evs[0].Subject.Key)
	assert.Empty(t, payloadOf(t, evs[0])["messageId"])
}

func TestTheSameMessageTwiceHasOneDedupKey(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)
	raw := mailText("<dup@x>", "a@example.org", "twice", "x")
	f.srv.Deliver(mailbox, "INBOX", raw)
	f.srv.Deliver(mailbox, "INBOX", raw)
	evs, _ := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 2)
	assert.Equal(t, evs[0].DedupKey, evs[1].DedupKey, "the engine drops the second")
}

func TestPollFailures(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)

	t.Run("a refused login is unauthorized: the engine stops polling", func(t *testing.T) {
		bad := f.imapCreds()
		bad["password"] = "wrong-password"
		_, next, err := f.imap.Poll(context.Background(), conn, bad, cur)
		require.Error(t, err)
		assert.Equal(t, catalog.CallUnauthorized, callError(t, err).Code)
		assert.NotContains(t, err.Error(), "wrong-password")
		assert.Nil(t, next)
	})
	t.Run("a network failure keeps the cursor", func(t *testing.T) {
		down := f.imapCreds()
		down["port"] = "1"
		_, next, err := f.imap.Poll(context.Background(), conn, down, cur)
		require.Error(t, err)
		var ce *catalog.CallError
		require.NotErrorIs(t, err, connectors.ErrNeedsReauth)
		assert.False(t, asCallUnauthorized(err, &ce), "not a login problem")
		assert.Nil(t, next, "no new cursor: the engine keeps the old one")
	})
	t.Run("a revoked sign-in is ErrNeedsReauth", func(t *testing.T) {
		f.oauth.revoke = true
		defer func() { f.oauth.revoke = false }()
		oc := f.oauthCreds("", "x")
		_, _, err := f.imap.Poll(context.Background(), conn, oc, cur)
		assert.ErrorIs(t, err, connectors.ErrNeedsReauth)
	})
}

func asCallUnauthorized(err error, ce **catalog.CallError) bool {
	for e := err; e != nil; {
		if c, ok := e.(*catalog.CallError); ok {
			*ce = c
			return c.Code == catalog.CallUnauthorized
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		e = u.Unwrap()
	}
	return false
}

func TestXOAUTH2Poll(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	f.srv.AcceptToken(mailbox, "ya29.imap")
	conn := f.connection(KindIMAP)
	creds := f.oauthCreds("", "stale")
	f.conns.add(conn, creds)
	f.oauth.tokens[conn.ID] = "ya29.imap"
	_, cur := f.pollOnce(t, conn, creds, nil)
	f.srv.Deliver(mailbox, "INBOX", mailText("<x1@x>", "a@example.org", "hi", "x"))
	evs, _ := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 1)
	assert.Contains(t, f.srv.Mechanisms(), "imap:XOAUTH2")

	t.Run("a token the server refuses stops polling and marks the smtp connection of the sign-in", func(t *testing.T) {
		smtpConn := f.connection(KindSMTP)
		f.conns.add(smtpConn, f.oauthCreds("", "ya29.imap")) // the companion holds the token that is refused
		f.srv.RevokeTokens(mailbox)
		_, _, err := f.imap.Poll(context.Background(), conn, creds, cur)
		assert.Equal(t, catalog.CallUnauthorized, callError(t, err).Code)
		assert.Equal(t, connectors.StatusNeedsReauth, f.conns.statusOf(smtpConn.ID))
	})
}

func TestIMAPTest(t *testing.T) {
	for _, mode := range []emailtest.Mode{emailtest.ModeTLS, emailtest.ModeSTARTTLS} {
		f := newFx(t, mode)
		id, err := f.imap.Test(context.Background(), f.imapCreds())
		require.NoError(t, err)
		assert.Equal(t, mailbox, id.Display)
		assert.Equal(t, connectors.IdentityEmail, id.Kind)
	}
	f := newFx(t, emailtest.ModeTLS)
	creds := f.imapCreds()
	creds["mailbox"] = "Nope"
	_, err := f.imap.Test(context.Background(), creds)
	require.Error(t, err)
	creds = f.imapCreds()
	creds["password"] = "wrong-password"
	_, err = f.imap.Test(context.Background(), creds)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 401")
	assert.NotContains(t, err.Error(), "wrong-password")
}

func TestAttachmentsAreStoredAndHTMLIsNotPassedOn(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)

	png := []byte("\x89PNG\r\n\x1a\nfake-image-bytes")
	f.srv.Deliver(mailbox, "INBOX", multipart("<pic@x>", "Photo", "See the picture.", "<p>See the <b>picture</b>.</p>",
		att{typ: "image/png", name: "cat.png", data: png}, att{typ: "application/octet-stream", name: "huge.bin", data: bytes.Repeat([]byte("a"), MaxAttachment+10)}))
	evs, _ := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 1)
	p := payloadOf(t, evs[0])
	assert.Equal(t, "See the picture.", p["text"])
	assert.Equal(t, true, p["hasHtml"])
	assert.NotContains(t, string(evs[0].Payload), "<b>", "the HTML of a message is not in the event")
	atts := p["attachments"].([]any)
	require.Len(t, atts, 2)
	first := atts[0].(map[string]any)
	assert.Equal(t, "cat.png", first["filename"])
	assert.Equal(t, "image/png", first["contentType"])
	assert.EqualValues(t, len(png), first["size"])
	assert.Equal(t, "https://app.example/uploads/1", first["url"])
	assert.Equal(t, "1", first["upload_id"])
	assert.Equal(t, png, f.uploads.files["1"].Data, "the file in the storage is the attachment")
	assert.Equal(t, "image/png", f.uploads.files["1"].Mime)
	big := atts[1].(map[string]any)
	assert.Equal(t, AttachmentTooLarge, big["error"])
	assert.Nil(t, big["url"])
	assert.EqualValues(t, MaxAttachment+10, big["size"])
	assert.Len(t, f.uploads.files, 1, "what is over the limit is not stored")
}

func TestAttachmentLimitPerMessageAndStorageFailures(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)
	chunk := bytes.Repeat([]byte("b"), 20<<20)
	f.srv.Deliver(mailbox, "INBOX", multipart("<three@x>", "Big", "x", "",
		att{typ: "application/pdf", name: "a.pdf", data: chunk}, att{typ: "application/pdf", name: "b.pdf", data: chunk}, att{typ: "application/pdf", name: "c.pdf", data: chunk}))
	evs, cur := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 1)
	atts := payloadOf(t, evs[0])["attachments"].([]any)
	require.Len(t, atts, 3)
	assert.NotEmpty(t, atts[0].(map[string]any)["url"])
	assert.NotEmpty(t, atts[1].(map[string]any)["url"])
	assert.Equal(t, AttachmentMessageTooBig, atts[2].(map[string]any)["error"], "50 MB per message")

	f.uploads.fail = true
	f.srv.Deliver(mailbox, "INBOX", multipart("<fail@x>", "Fail", "x", "", att{typ: "text/csv", name: "t.csv", data: []byte("a,b")}))
	evs, cur = f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 1, "the event is never lost for an attachment")
	assert.Equal(t, AttachmentStorage, payloadOf(t, evs[0])["attachments"].([]any)[0].(map[string]any)["error"])

	f.imap.cfg.Uploads = nil
	f.srv.Deliver(mailbox, "INBOX", multipart("<nostore@x>", "No", "x", "", att{typ: "text/csv", name: "t.csv", data: []byte("a,b")}))
	evs, _ = f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 1)
	assert.Equal(t, AttachmentNoStorage, payloadOf(t, evs[0])["attachments"].([]any)[0].(map[string]any)["error"])
}

func TestHTMLOnlyMessageGivesItsText(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)
	raw := "From: a@example.org\r\nTo: " + mailbox + "\r\nSubject: Html\r\nMessage-ID: <h@x>\r\nContent-Type: text/html; charset=utf-8\r\n\r\n" +
		"<html><head><style>p{color:red}</style></head><body><p>Hello&nbsp;<b>world</b></p><script>alert(1)</script><p>Second &amp; last</p></body></html>\r\n"
	f.srv.Deliver(mailbox, "INBOX", []byte(raw))
	evs, _ := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 1)
	p := payloadOf(t, evs[0])
	assert.Equal(t, "Hello world\n\nSecond & last", p["text"])
	assert.Equal(t, true, p["hasHtml"])
}

func TestTextIsLimited(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)
	f.srv.Deliver(mailbox, "INBOX", mailText("<long@x>", "a@example.org", "long", string(bytes.Repeat([]byte("я"), MaxText+500))))
	evs, _ := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 1)
	assert.Len(t, []rune(payloadOf(t, evs[0])["text"].(string)), MaxText)
}

func TestBatchIsBoundedAndTheRestComesNext(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)
	for i := 0; i < maxBatch+5; i++ {
		f.srv.Deliver(mailbox, "INBOX", mailText(fmt.Sprintf("<m%d@x>", i), "a@example.org", "n", "x"))
	}
	evs, cur := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, maxBatch)
	evs, _ = f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 5)
	assert.Equal(t, fmt.Sprintf("<m%d@x>", maxBatch), evs[0].DedupKey)
}

func TestOwnMessageIsBySelf(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)
	f.srv.Deliver(mailbox, "INBOX", mailText("<me@x>", "Robot <ROBOT@example.com>", "note to self", "x"))
	f.srv.Deliver(mailbox, "INBOX", mailText("<you@x>", "a@example.org", "hi", "x"))
	evs, _ := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 2)
	assert.True(t, evs[0].BySelf)
	assert.False(t, evs[1].BySelf)
}

func TestIntervalIsAMinute(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	assert.Equal(t, PollInterval, f.imap.Interval(connectors.Connection{}))
	assert.GreaterOrEqual(t, PollInterval, engine.MinPollInterval)
}

type att struct {
	typ, name string
	data      []byte
	width     int // the characters of a base64 line (76 when 0)
}

// multipart is a message with a text, an optional HTML alternative and attachments.
func multipart(id, subject, text, html string, atts ...att) []byte {
	var b bytes.Buffer
	b.WriteString("From: Ann <ann@example.org>\r\nTo: " + mailbox + "\r\nSubject: " + subject + "\r\nMessage-ID: " + id + "\r\nMIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=\"MIX\"\r\n\r\n--MIX\r\n")
	if html == "" {
		b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n" + text + "\r\n")
	} else {
		b.WriteString("Content-Type: multipart/alternative; boundary=\"ALT\"\r\n\r\n")
		b.WriteString("--ALT\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + text + "\r\n")
		b.WriteString("--ALT\r\nContent-Type: text/html; charset=utf-8\r\n\r\n" + html + "\r\n--ALT--\r\n")
	}
	for _, a := range atts {
		enc := base64.StdEncoding.EncodeToString(a.data)
		b.WriteString("--MIX\r\nContent-Type: " + a.typ + "; name=\"" + a.name + "\"\r\nContent-Disposition: attachment; filename=\"" + a.name + "\"\r\nContent-Transfer-Encoding: base64\r\n\r\n")
		width := a.width
		if width == 0 {
			width = 76
		}
		for len(enc) > width {
			b.WriteString(enc[:width] + "\r\n")
			enc = enc[width:]
		}
		b.WriteString(enc + "\r\n")
	}
	b.WriteString("--MIX--\r\n")
	return b.Bytes()
}
