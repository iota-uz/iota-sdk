package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

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

// unknownEncoding is a message whose attachment is in a transfer encoding the reader does not know (x-uuencode).
func unknownEncoding() []byte {
	var b bytes.Buffer
	b.WriteString("From: Ann <ann@example.org>\r\nTo: " + mailbox + "\r\nSubject: Uu\r\nMessage-ID: <uu@x>\r\nMIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=\"MIX\"\r\n\r\n--MIX\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nbody\r\n")
	b.WriteString("--MIX\r\nContent-Type: application/pdf; name=\"uu.pdf\"\r\nContent-Disposition: attachment; filename=\"uu.pdf\"\r\nContent-Transfer-Encoding: x-uuencode\r\n\r\n")
	b.WriteString("begin 644 uu.pdf\r\n#86)C\r\n`\r\nend\r\n")
	b.WriteString("--MIX\r\nContent-Type: text/csv; name=\"ok.csv\"\r\nContent-Disposition: attachment; filename=\"ok.csv\"\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(base64.StdEncoding.EncodeToString([]byte("a,b\r\n")) + "\r\n--MIX--\r\n")
	return b.Bytes()
}

// An attachment in a transfer encoding the reader does not know is not published as stored: its bytes are
// still encoded, so it is unreadable; the message and its other attachments are as usual.
func TestAttachmentInAnUnsupportedTransferEncodingIsUnreadable(t *testing.T) {
	t.Run("parsed whole", func(t *testing.T) {
		m, err := parseMessage(unknownEncoding())
		require.NoError(t, err)
		require.Len(t, m.attachment, 2)
		assert.Equal(t, AttachmentUnreadable, m.attachment[0].reason)
		assert.Nil(t, m.attachment[0].data, "the encoded bytes are not kept as the file")
		assert.Empty(t, m.attachment[1].reason)
		assert.Equal(t, "a,b\r\n", string(m.attachment[1].data))
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
			require.Len(t, atts, 2)
			uu := atts[0].(map[string]any)
			assert.Equal(t, AttachmentUnreadable, uu["error"])
			assert.Empty(t, uu["url"], "no stored file stands for it")
			assert.Nil(t, atts[1].(map[string]any)["error"])
			assert.Len(t, f.uploads.files, 1, "only the readable attachment is stored")
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
