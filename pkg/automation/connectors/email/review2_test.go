package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email/emailtest"
)

// The review items of #1426: each MIME and IMAP case has a test of its own.

// A correct MIME wrapping (base64 in 76-character lines with CRLF) makes an attachment larger on the wire than
// it is: one that is within the limit once decoded is stored, in a message read whole and in one read by parts.
func TestCorrectlyWrappedAttachmentWithinTheDecodedLimitIsStored(t *testing.T) {
	for name, raw := range map[string]int64{"oversized message": 1 << 20, "whole message": defaultMaxRaw} {
		t.Run(name, func(t *testing.T) {
			old := maxRaw
			maxRaw = raw
			t.Cleanup(func() { maxRaw = old })
			f := newFx(t, emailtest.ModeTLS)
			conn, creds := f.connection(KindIMAP), f.imapCreds()
			_, cur := f.pollOnce(t, conn, creds, nil)
			data := bytes.Repeat([]byte{0xA5}, MaxAttachment) // exactly the limit: its base64 is ~34 MB on the wire
			f.srv.Deliver(mailbox, "INBOX", multipart("<wrap@x>", "Wrapped", "x", "", att{typ: "application/pdf", name: "limit.pdf", data: data}))
			evs, _ := f.pollOnce(t, conn, creds, cur)
			require.Len(t, evs, 1)
			a := payloadOf(t, evs[0])["attachments"].([]any)[0].(map[string]any)
			assert.Nil(t, a["error"], "stored, not too_large")
			assert.EqualValues(t, MaxAttachment, a["size"])
			require.Len(t, f.uploads.files, 1)
			for _, file := range f.uploads.files {
				assert.True(t, bytes.Equal(data, file.Data), "the decoded bytes are stored")
			}
		})
	}
}

// unknownEncoding is a message whose attachments are in a transfer encoding the reader does not know
// (x-uuencode), once with a charset it does not know either, between readable ones — one of them base64 with an
// unknown charset, which is still decoded.
func unknownEncoding() []byte {
	var b bytes.Buffer
	b.WriteString("From: Ann <ann@example.org>\r\nTo: " + mailbox + "\r\nSubject: Uu\r\nMessage-ID: <uu@x>\r\nMIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=\"MIX\"\r\n\r\n--MIX\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nbody\r\n")
	b.WriteString("--MIX\r\nContent-Type: application/pdf; name=\"uu.pdf\"\r\nContent-Disposition: attachment; filename=\"uu.pdf\"\r\nContent-Transfer-Encoding: x-uuencode\r\n\r\n")
	b.WriteString("begin 644 uu.pdf\r\n#86)C\r\n`\r\nend\r\n")
	b.WriteString("--MIX\r\nContent-Type: text/csv; name=\"ok.csv\"\r\nContent-Disposition: attachment; filename=\"ok.csv\"\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(base64.StdEncoding.EncodeToString([]byte("a,b\r\n")) + "\r\n")
	b.WriteString("--MIX\r\nContent-Type: text/csv; charset=x-unknown; name=\"uu.csv\"\r\nContent-Disposition: attachment; filename=\"uu.csv\"\r\nContent-Transfer-Encoding: x-uuencode\r\n\r\n")
	b.WriteString("begin 644 uu.csv\r\n#86)C\r\n`\r\nend\r\n")
	b.WriteString("--MIX\r\nContent-Type: text/csv; charset=x-unknown; name=\"odd.csv\"\r\nContent-Disposition: attachment; filename=\"odd.csv\"\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(base64.StdEncoding.EncodeToString([]byte("c,d\r\n")) + "\r\n--MIX--\r\n")
	return b.Bytes()
}

// An attachment in a transfer encoding the reader does not know is not published as stored: its bytes are
// still encoded, so it is unreadable; the message and its other attachments are as usual.
func TestAttachmentInAnUnsupportedTransferEncodingIsUnreadable(t *testing.T) {
	t.Run("parsed whole", func(t *testing.T) {
		m, err := parseMessage(unknownEncoding())
		require.NoError(t, err)
		require.Len(t, m.attachment, 4)
		assert.Equal(t, AttachmentUnreadable, m.attachment[0].reason)
		assert.Nil(t, m.attachment[0].data, "the encoded bytes are not kept as the file")
		assert.Empty(t, m.attachment[1].reason)
		assert.Equal(t, "a,b\r\n", string(m.attachment[1].data))
		assert.Equal(t, AttachmentUnreadable, m.attachment[2].reason, "an unknown charset does not hide the unknown encoding")
		assert.Nil(t, m.attachment[2].data)
		assert.Empty(t, m.attachment[3].reason, "a supported encoding with an unknown charset is decoded")
		assert.Equal(t, "c,d\r\n", string(m.attachment[3].data))
		assert.Equal(t, "body", m.text)
	})
	for name, raw := range map[string]int64{"oversized message": 256, "whole message": defaultMaxRaw} {
		t.Run("polled, "+name, func(t *testing.T) {
			old := maxRaw
			maxRaw = raw
			t.Cleanup(func() { maxRaw = old })
			f := newFx(t, emailtest.ModeTLS)
			conn, creds := f.connection(KindIMAP), f.imapCreds()
			_, cur := f.pollOnce(t, conn, creds, nil)
			f.srv.Deliver(mailbox, "INBOX", unknownEncoding())
			evs, _ := f.pollOnce(t, conn, creds, cur)
			require.Len(t, evs, 1, "the message is published")
			atts := payloadOf(t, evs[0])["attachments"].([]any)
			require.Len(t, atts, 4)
			for _, i := range []int{0, 2} {
				uu := atts[i].(map[string]any)
				assert.Equal(t, AttachmentUnreadable, uu["error"], uu["filename"])
				assert.Empty(t, uu["url"], "no stored file stands for it")
			}
			assert.Nil(t, atts[1].(map[string]any)["error"])
			assert.Nil(t, atts[3].(map[string]any)["error"])
			assert.Len(t, f.uploads.files, 2, "only the readable attachments are stored")
		})
	}
}

// A mailbox whose IMAP login is not an email address has no address identity, yet a reply to a message the
// graph sends will come to it: email.send seeds its cursor before sending, so the first reply is not jumped
// over by the first poll.
func TestFirstReplyIsKeptWhenTheIMAPLoginIsNotAnAddress(t *testing.T) {
	ctx := context.Background()
	const login = "robot" // a login of the server, not an address
	f := newFx(t, emailtest.ModeTLS)
	f.srv.AddAccount(emailtest.Account{User: login, Password: password})
	f.srv.Deliver(login, "INBOX", mailText("<old@x>", "a@example.org", "old", "before the send"))

	imapCreds := f.imapCreds()
	imapCreds["username"] = login
	id, err := f.imap.Test(ctx, imapCreds)
	require.NoError(t, err)
	require.Empty(t, id.External, "the login is not an address: the connection has no address identity")
	imapConn := f.connection(KindIMAP)
	imapConn.Identity = id
	f.conns.add(imapConn, imapCreds)
	other := f.connection(KindIMAP)
	other.Identity = connectors.Identity{External: "someone@else.example", Kind: connectors.IdentityEmail}
	f.conns.add(other, f.imapCreds())

	res, err := (&sendNode{f.smtp.hub}).Execute(ctx, f.sendInput(params(nil), f.smtpCreds()))
	require.NoError(t, err)
	require.Equal(t, "sent", res.Outcome)
	sentID := res.Output["message_id"].(string)

	require.True(t, hasCursor(f, imapConn), "the mailbox without an address was baselined before the send")
	assert.False(t, hasCursor(f, other), "a mailbox of another address is left to the poller")

	// The reply arrives before the first poll of the mailbox.
	f.srv.Deliver(login, "INBOX", mailText("<reply@x>", "boss@example.org", "Re: Invoice 42", "Paid.", "In-Reply-To: "+sentID, "References: "+sentID))
	evs, _ := f.pollOnce(t, imapConn, imapCreds, seededCursor(t, f, imapConn))
	require.Len(t, evs, 1, "the first reply is published, the mail from before the send is not")
	assert.Equal(t, "<reply@x>", payloadOf(t, evs[0])["messageId"])
}

func hasCursor(f *fx, c connectors.Connection) bool {
	f.conns.mu.Lock()
	defer f.conns.mu.Unlock()
	return len(f.conns.inbound[c.ID]) > 0
}

func seededCursor(t *testing.T, f *fx, c connectors.Connection) json.RawMessage {
	t.Helper()
	f.conns.mu.Lock()
	defer f.conns.mu.Unlock()
	var all map[string]struct {
		Cursor json.RawMessage `json:"cursor"`
	}
	require.NoError(t, json.Unmarshal(f.conns.inbound[c.ID], &all))
	require.NotEmpty(t, all["poll"].Cursor)
	return all["poll"].Cursor
}

// A mailbox that does not answer has a time of its own before a send: the send is not held for long, nothing is
// seeded for that mailbox, and the message goes out.
func TestASilentMailboxDoesNotHoldTheSend(t *testing.T) {
	old := baselineTimeout
	baselineTimeout = 300 * time.Millisecond
	t.Cleanup(func() { baselineTimeout = old })
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { _ = c.Close() }) // accepted and never answered
		}
	}()
	f := newFx(t, emailtest.ModeTLS)
	silent := f.connection(KindIMAP)
	silent.Identity = connectors.Identity{} // the address is unknown: it is baselined before the send
	creds := f.imapCreds()
	creds["port"] = port(ln.Addr().String())
	creds["username"] = "silent"
	f.conns.add(silent, creds)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	res, err := (&sendNode{f.smtp.hub}).Execute(ctx, f.sendInput(params(nil), f.smtpCreds()))
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
	assert.Less(t, time.Since(start), 10*time.Second, "the silent mailbox took its own time, not the send's")
	assert.False(t, hasCursor(f, silent))
	assert.Len(t, f.srv.Sent(), 1)
}

// The mailbox of the sender's address is where the reply comes: its baseline takes the send's time, not the short
// one of a mailbox whose address is unknown, so a slow server still gets its cursor before the message goes.
func TestTheSendersMailboxIsSeededEvenWhenItIsSlow(t *testing.T) {
	old := baselineTimeout
	baselineTimeout = time.Nanosecond // a mailbox of unknown address would never make it
	t.Cleanup(func() { baselineTimeout = old })
	f := newFx(t, emailtest.ModeTLS)
	own := f.connection(KindIMAP)
	own.Identity = connectors.Identity{External: mailbox, Kind: connectors.IdentityEmail}
	f.conns.add(own, f.imapCreds())
	res, err := (&sendNode{f.smtp.hub}).Execute(context.Background(), f.sendInput(params(nil), f.smtpCreds()))
	require.NoError(t, err)
	assert.Equal(t, "sent", res.Outcome)
	assert.True(t, hasCursor(f, own), "the sender's mailbox is seeded with the send's time")
}
