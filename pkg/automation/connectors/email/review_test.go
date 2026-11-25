package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email/emailtest"
)

// A temporary refusal of one recipient among several sends nothing: the whole call is unavailable, so a
// repeat does not send twice to the others.
func TestTemporaryRecipientRefusalAbortsBeforeData(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	f.srv.RejectRecipient("slow@example.org", 451)
	_, err := (&sendNode{f.smtp.hub}).Execute(context.Background(), f.sendInput(params(map[string]any{"to": []any{"boss@example.org", "slow@example.org"}}), f.smtpCreds()))
	ce := callError(t, err)
	assert.Equal(t, catalog.CallUnavailable, ce.Code)
	assert.True(t, ce.Retryable)
	assert.Empty(t, f.srv.Sent(), "no message went to the recipient the server accepted")
	assert.Empty(t, f.usage.entries)

	// 250 and 550: the message goes to the accepted one, the refused is listed.
	f2 := newFx(t, emailtest.ModeTLS)
	f2.srv.RejectRecipient("gone@example.org", 550)
	res, err := (&sendNode{f2.smtp.hub}).Execute(context.Background(), f2.sendInput(params(map[string]any{"to": []any{"boss@example.org", "gone@example.org"}}), f2.smtpCreds()))
	require.NoError(t, err)
	assert.Equal(t, []any{"gone@example.org"}, res.Output["rejected"])
}

// A failed read in the middle of a batch ends the poll with the error and no new cursor.
func TestFetchFailureInTheMiddleOfABatchKeepsTheCursor(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)
	for i := 1; i <= 3; i++ {
		f.srv.Deliver(mailbox, "INBOX", mailText(fmt.Sprintf("<b%d@x>", i), "a@example.org", "n", "x"))
	}
	f.srv.FailBodyFetch(2) // the second message of the batch
	evs, next, err := f.imap.Poll(context.Background(), conn, creds, cur)
	require.Error(t, err)
	assert.Empty(t, evs, "what was read is not published without the cursor that covers it")
	assert.Nil(t, next, "the engine keeps the cursor it had")

	evs, _ = f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 3, "the next poll reads the whole batch again")
	assert.Equal(t, "<b1@x>", evs[0].DedupKey)
}

// A message past the whole-message limit is read by its structure: a part within the limits is stored like
// any other (a large text body does not cost a small image its place), a part over a limit is reported with
// it, and the body of that one is never downloaded.
func TestOversizedMessageStoresWhatFitsAndReportsWhatDoesNot(t *testing.T) {
	old := maxRaw
	maxRaw = 1 << 20
	t.Cleanup(func() { maxRaw = old })
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)
	small := bytes.Repeat([]byte("s"), 2<<20)
	big := bytes.Repeat([]byte("b"), MaxAttachment+(1<<20))
	f.srv.Deliver(mailbox, "INBOX", multipart("<huge@x>", "Huge", "The covering text.", "<p>html</p>",
		att{typ: "application/pdf", name: "small.pdf", data: small}, att{typ: "application/zip", name: "big.zip", data: big}))
	evs, next := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 1)
	p := payloadOf(t, evs[0])
	assert.Equal(t, "Huge", p["subject"])
	assert.Equal(t, "The covering text.", p["text"])
	assert.Equal(t, true, p["hasHtml"])
	atts := p["attachments"].([]any)
	require.Len(t, atts, 2, "attachments are listed, not lost")
	first, second := atts[0].(map[string]any), atts[1].(map[string]any)
	assert.Equal(t, "small.pdf", first["filename"])
	assert.EqualValues(t, len(small), first["size"], "the decoded size")
	assert.Equal(t, "https://app.example/uploads/1", first["url"], "a part within both limits is stored")
	assert.Nil(t, first["error"])
	assert.Equal(t, "big.zip", second["filename"])
	assert.Equal(t, AttachmentTooLarge, second["error"])
	assert.Nil(t, second["url"])
	require.Len(t, f.uploads.files, 1, "the body of the part over the limit was not downloaded or stored")
	assert.Equal(t, small, f.uploads.files["1"].Data)

	// The parts together past 50 MB: the ones that fit are stored in order, the rest are message_too_big.
	chunk := bytes.Repeat([]byte("c"), 20<<20)
	f.srv.Deliver(mailbox, "INBOX", multipart("<huge2@x>", "Huge 2", "x", "",
		att{typ: "application/pdf", name: "a.pdf", data: chunk}, att{typ: "application/pdf", name: "b.pdf", data: chunk}, att{typ: "application/pdf", name: "c.pdf", data: chunk}))
	evs, _ = f.pollOnce(t, conn, creds, next)
	require.Len(t, evs, 1)
	atts = payloadOf(t, evs[0])["attachments"].([]any)
	require.Len(t, atts, 3)
	assert.NotEmpty(t, atts[0].(map[string]any)["url"])
	assert.NotEmpty(t, atts[1].(map[string]any)["url"])
	assert.Equal(t, AttachmentMessageTooBig, atts[2].(map[string]any)["error"])
}

func (f *fx) labelOnce(t *testing.T, p map[string]any) (catalog.CallResult, error) {
	t.Helper()
	return (&labelNode{f.imap.hub}).Execute(context.Background(), catalog.CallInput{TenantID: tenant, Params: p, Credentials: f.imapCreds(), Connection: &catalog.ConnectionInfo{ID: uuid.NewString()}})
}

// Without MOVE the message is copied first, the copy is checked, and only then is the original deleted — by
// UID, so no other message that is flagged is expunged. A copy that fails deletes nothing.
func TestMoveWithoutMoveNeverLosesMail(t *testing.T) {
	t.Run("a missing destination deletes nothing", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.DisableMove()
		f.srv.Deliver(mailbox, "INBOX", mailText("<m1@x>", "a@example.org", "one", "x"))
		_, err := f.labelOnce(t, map[string]any{"message_id": "<m1@x>", "move_to": "Nowhere"})
		require.Error(t, err)
		inbox := f.srv.Fetch(mailbox, "INBOX")
		require.Len(t, inbox, 1, "the message is still there")
		assert.NotContains(t, lower(inbox[0].Flags), `\deleted`, "and not even marked")
	})
	t.Run("with UIDPLUS only the moved message is expunged", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.DisableMove()
		f.srv.Mailbox(mailbox, "Archive")
		stale := f.srv.Deliver(mailbox, "INBOX", mailText("<m1@x>", "a@example.org", "one", "x"))
		f.srv.Deliver(mailbox, "INBOX", mailText("<m2@x>", "a@example.org", "two", "x"))
		_, err := f.labelOnce(t, map[string]any{"message_id": "<m2@x>", "keywords": []any{"keep"}})
		require.NoError(t, err)
		// m1 is flagged deleted by someone else, awaiting its own expunge: an EXPUNGE that is not scoped to
		// the moved message's UID would remove it.
		f.srv.Flag(mailbox, "INBOX", stale, `\Deleted`)
		res, err := f.labelOnce(t, map[string]any{"message_id": "<m2@x>", "move_to": "Archive"})
		require.NoError(t, err)
		assert.Equal(t, "done", res.Outcome)
		inbox := f.srv.Fetch(mailbox, "INBOX")
		require.Len(t, inbox, 1)
		assert.Contains(t, string(inbox[0].Raw), "<m1@x>", "the other message is untouched")
		assert.Len(t, f.srv.Fetch(mailbox, "Archive"), 1)
	})
	t.Run("without UIDPLUS the original stays, flagged", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.DisableMove()
		f.srv.DisableUIDPlus()
		f.srv.Mailbox(mailbox, "Archive")
		f.srv.Deliver(mailbox, "INBOX", mailText("<m1@x>", "a@example.org", "one", "x"))
		_, err := f.labelOnce(t, map[string]any{"message_id": "<m1@x>", "move_to": "Archive"})
		require.NoError(t, err)
		assert.Len(t, f.srv.Fetch(mailbox, "Archive"), 1, "copied")
		inbox := f.srv.Fetch(mailbox, "INBOX")
		require.Len(t, inbox, 1, "never expunged by a command that is not scoped to it")
		assert.Contains(t, lower(inbox[0].Flags), `\deleted`)
	})
}

// A message of the destination that has the same Message-ID is not proof of a copy (it may be another
// message): the fallback copies, and only then is the original dealt with. A repeat after an interrupted
// move may leave a second copy, never none. Several messages of the source with one id are not moved.
func TestMoveFallbackNeverTakesADestinationMessageForItsCopy(t *testing.T) {
	t.Run("a message with the same id in the destination is not the copy", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.DisableMove()
		f.srv.Mailbox(mailbox, "Archive")
		f.srv.Deliver(mailbox, "INBOX", mailText("<m1@x>", "a@example.org", "one", "x"))
		f.srv.Deliver(mailbox, "Archive", mailText("<m1@x>", "a@example.org", "another message with the same id", "x"))
		res, err := f.labelOnce(t, map[string]any{"message_id": "<m1@x>", "move_to": "Archive"})
		require.NoError(t, err)
		assert.Equal(t, "done", res.Outcome)
		subjects := ""
		for _, m := range f.srv.Fetch(mailbox, "Archive") {
			subjects += string(m.Raw)
		}
		assert.Contains(t, subjects, "Subject: one", "the message itself was copied")
		assert.Contains(t, subjects, "another message with the same id", "and the other is still there")
		assert.Empty(t, f.srv.Fetch(mailbox, "INBOX"))
	})
	t.Run("a repeat copies again: a second copy, never none", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.DisableMove()
		f.srv.DisableUIDPlus()
		f.srv.Mailbox(mailbox, "Archive")
		raw := mailText("<m1@x>", "a@example.org", "one", "x")
		f.srv.Deliver(mailbox, "INBOX", raw)
		f.srv.Deliver(mailbox, "Archive", raw) // the first attempt copied, then the connection ended
		res, err := f.labelOnce(t, map[string]any{"message_id": "<m1@x>", "move_to": "Archive"})
		require.NoError(t, err)
		assert.Equal(t, "done", res.Outcome)
		assert.Len(t, f.srv.Fetch(mailbox, "Archive"), 2)
		inbox := f.srv.Fetch(mailbox, "INBOX")
		require.Len(t, inbox, 1)
		assert.Contains(t, lower(inbox[0].Flags), `\deleted`)
		// The original is flagged now: the message is no longer the mailbox's.
		res, err = f.labelOnce(t, map[string]any{"message_id": "<m1@x>", "move_to": "Archive"})
		require.NoError(t, err)
		assert.Equal(t, "not_found", res.Outcome)
		assert.Len(t, f.srv.Fetch(mailbox, "Archive"), 2)
	})
	t.Run("several messages with one id are not moved", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.DisableMove()
		f.srv.Mailbox(mailbox, "Archive")
		f.srv.Deliver(mailbox, "INBOX", mailText("<m1@x>", "a@example.org", "one", "x"))
		f.srv.Deliver(mailbox, "INBOX", mailText("<m1@x>", "a@example.org", "one again", "x"))
		_, err := f.labelOnce(t, map[string]any{"message_id": "<m1@x>", "move_to": "Archive"})
		require.Error(t, err)
		assert.Equal(t, catalog.CallInvalidRequest, callError(t, err).Code)
		assert.Empty(t, f.srv.Fetch(mailbox, "Archive"), "nothing was copied")
		inbox := f.srv.Fetch(mailbox, "INBOX")
		require.Len(t, inbox, 2, "and nothing was deleted")
		for _, m := range inbox {
			assert.NotContains(t, lower(m.Flags), `\deleted`)
		}
	})
}

// An entry in the destination that only looks like the message — a header that contains its Message-ID
// among others, or an exact copy flagged \\Deleted — is no proof of a copy: the message is copied, and only
// then is the original expunged.
func TestMoveFallbackDoesNotTrustASubstringOrADeletedCopy(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	f.srv.DisableMove() // UIDPLUS stays: the original is expunged by UID
	f.srv.Mailbox(mailbox, "Archive")
	f.srv.Deliver(mailbox, "INBOX", mailText("<m1@x>", "a@example.org", "one", "x"))
	// A decoy whose Message-ID field contains the id as a substring, and a dead exact copy.
	f.srv.Deliver(mailbox, "Archive", mailText("<other@x> <m1@x>", "a@example.org", "decoy", "x"))
	dead := f.srv.Deliver(mailbox, "Archive", mailText("<m1@x>", "a@example.org", "dead copy", "x"))
	f.srv.Flag(mailbox, "Archive", dead, `\Deleted`)

	res, err := f.labelOnce(t, map[string]any{"message_id": "<m1@x>", "move_to": "Archive"})
	require.NoError(t, err)
	assert.Equal(t, "done", res.Outcome)
	live := 0
	for _, m := range f.srv.Fetch(mailbox, "Archive") {
		if bytes.Contains(m.Raw, []byte("Subject: one")) && !contains(lower(m.Flags), `\deleted`) {
			live++
		}
	}
	assert.Equal(t, 1, live, "a live copy of the message was made in the destination before the original went")
	assert.Empty(t, f.srv.Fetch(mailbox, "INBOX"))
}

// RFC 2045 sets the longest line of a base64 body, not the shortest: an attachment of exactly the limit
// whose lines are two characters long is as large on the wire as it is, and is stored — in a message that is
// read in parts (oversized) and in one that is read whole.
func TestBase64WithShortLinesWithinTheLimitIsStored(t *testing.T) {
	for name, raw := range map[string]int64{"oversized message": 1 << 20, "whole message": defaultMaxRaw} {
		t.Run(name, func(t *testing.T) {
			old := maxRaw
			maxRaw = raw
			t.Cleanup(func() { maxRaw = old })
			f := newFx(t, emailtest.ModeTLS)
			conn, creds := f.connection(KindIMAP), f.imapCreds()
			_, cur := f.pollOnce(t, conn, creds, nil)
			data := bytes.Repeat([]byte("x"), MaxAttachment)
			f.srv.Deliver(mailbox, "INBOX", multipart("<short@x>", "Short lines", "Covering text.", "", att{typ: "application/pdf", name: "exact.pdf", data: data, width: 2}))
			evs, _ := f.pollOnce(t, conn, creds, cur)
			require.Len(t, evs, 1)
			atts := payloadOf(t, evs[0])["attachments"].([]any)
			require.Len(t, atts, 1)
			a := atts[0].(map[string]any)
			assert.Nil(t, a["error"], "stored, not too_large")
			assert.EqualValues(t, MaxAttachment, a["size"])
			require.Len(t, f.uploads.files, 1)
		})
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// A part that cannot be decoded, or a text cut at the limit of the partial read, is a result about the
// content: the message is still published, and the mailbox does not stall on it.
func TestOversizedMessageWithAnUndecodablePartIsPublished(t *testing.T) {
	old := maxRaw
	maxRaw = 1 << 20
	t.Cleanup(func() { maxRaw = old })
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)

	longText := bytes.Repeat([]byte("word "), 400_000) // 2 MB: past the 1 MB read of a text part, so cut
	var b bytes.Buffer
	b.WriteString("From: Ann <ann@example.org>\r\nTo: " + mailbox + "\r\nSubject: Broken\r\nMessage-ID: <broken@x>\r\nMIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=\"MIX\"\r\n\r\n--MIX\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	enc := base64.StdEncoding.EncodeToString(longText)
	for len(enc) > 76 {
		b.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc + "\r\n--MIX\r\nContent-Type: application/pdf; name=\"bad.pdf\"\r\nContent-Disposition: attachment; filename=\"bad.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\n!!! this is not base64 !!!\r\n--MIX--\r\n")
	f.srv.Deliver(mailbox, "INBOX", b.Bytes())
	f.srv.Deliver(mailbox, "INBOX", mailText("<after@x>", "a@example.org", "after", "x"))
	evs, _ := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 2, "the broken message does not hold back the one behind it")
	p := payloadOf(t, evs[0])
	assert.NotEmpty(t, p["text"], "the part of the text that was read")
	atts := p["attachments"].([]any)
	require.Len(t, atts, 1)
	assert.Equal(t, AttachmentUnreadable, atts[0].(map[string]any)["error"])
}

// BODYSTRUCTURE gives the size of the encoded octets, line breaks included: an attachment of 24.5 MiB, wrapped
// as base64 is, is within the limit of 25 MiB and is stored.
func TestWrappedBase64IsMeasuredByItsDecodedBytes(t *testing.T) {
	old := maxRaw
	maxRaw = 1 << 20
	t.Cleanup(func() { maxRaw = old })
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)
	data := bytes.Repeat([]byte("w"), MaxAttachment-(512<<10))
	f.srv.Deliver(mailbox, "INBOX", multipart("<wrapped@x>", "Wrapped", "x", "", att{typ: "application/pdf", name: "big.pdf", data: data}))
	evs, _ := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 1)
	a := payloadOf(t, evs[0])["attachments"].([]any)[0].(map[string]any)
	assert.Nil(t, a["error"], "the encoded size is past 25 MiB, the decoded one is not")
	assert.EqualValues(t, len(data), a["size"])
	assert.NotEmpty(t, a["url"])
}

// A part that decodes to some bytes and then fails is not stored as if those bytes were the file: the part is
// unreadable, and the message and its other attachments are as usual.
func TestAttachmentThatFailsToDecodeAfterSomeBytesIsUnreadable(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("From: Ann <ann@example.org>\r\nTo: " + mailbox + "\r\nSubject: Half\r\nMessage-ID: <half@x>\r\nMIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=\"MIX\"\r\n\r\n--MIX\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nbody\r\n")
	b.WriteString("--MIX\r\nContent-Type: application/pdf; name=\"half.pdf\"\r\nContent-Disposition: attachment; filename=\"half.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString("QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo=\r\n!!! not base64 !!!\r\n")
	b.WriteString("--MIX\r\nContent-Type: text/csv; name=\"ok.csv\"\r\nContent-Disposition: attachment; filename=\"ok.csv\"\r\n\r\na,b\r\n--MIX--\r\n")
	m, err := parseMessage(b.Bytes())
	require.NoError(t, err)
	require.Len(t, m.attachment, 2)
	assert.Equal(t, AttachmentUnreadable, m.attachment[0].reason)
	assert.Nil(t, m.attachment[0].data, "the bytes that decoded are not kept")
	assert.Empty(t, m.attachment[1].reason)
	assert.NotNil(t, m.attachment[1].data)
}

// A text-only message declares UTF-8: a transfer encoding alone leaves a reader to guess US-ASCII.
func TestTextOnlyMessageDeclaresUTF8(t *testing.T) {
	raw, err := buildMessage(outgoing{From: mail.Address{Address: "a@example.org"}, To: []string{"b@example.org"}, Subject: "Привет", Text: "Привет, мир", MessageID: "<u@x>", Date: time.Now()})
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(string(raw)), "charset=utf-8")
	m, err := parseMessage(raw)
	require.NoError(t, err)
	assert.Equal(t, "Привет, мир", strings.TrimSpace(m.text))
}

// IMAP's [UNAVAILABLE] answer to a login is the server being briefly down, not a refusal of the credentials.
func TestImapUnavailableLoginIsRetryable(t *testing.T) {
	err := networkOrRefused(&imap.Error{Type: imap.StatusResponseTypeNo, Code: imap.ResponseCodeUnavailable, Text: "backend down"}, context.Background())
	ce := callError(t, err)
	assert.Equal(t, catalog.CallUnavailable, ce.Code)
	assert.True(t, ce.Retryable)
	err = networkOrRefused(&imap.Error{Type: imap.StatusResponseTypeNo, Code: imap.ResponseCodeAuthenticationFailed, Text: "bad"}, context.Background())
	assert.Equal(t, catalog.CallUnauthorized, callError(t, err).Code, "a refused login stays unauthorized")
}

// A move that ended before its UID EXPUNGE is finished on the repeat only when the original carries the record
// of this workflow's COPY; a message that someone flagged \Deleted, with a same-id message already in the
// destination, is not this workflow's to expunge.
func TestResumeMoveNeedsTheRecordOfItsOwnCopy(t *testing.T) {
	setup := func(t *testing.T) (*fx, uint32) {
		t.Helper()
		f := newFx(t, emailtest.ModeTLS)
		f.srv.DisableMove()
		f.srv.Mailbox(mailbox, "Archive")
		raw := mailText("<m1@x>", "a@example.org", "one", "x")
		uid := f.srv.Deliver(mailbox, "INBOX", raw)
		f.srv.Deliver(mailbox, "Archive", raw)
		return f, uid
	}
	t.Run("flagged by this workflow's move: finished", func(t *testing.T) {
		f, uid := setup(t)
		f.srv.Flag(mailbox, "INBOX", uid, `\Deleted`, moveKeyword("Archive"))
		res, err := f.labelOnce(t, map[string]any{"message_id": "<m1@x>", "move_to": "Archive"})
		require.NoError(t, err)
		assert.Equal(t, "done", res.Outcome)
		assert.Empty(t, f.srv.Fetch(mailbox, "INBOX"), "expunged by UID")
		assert.Len(t, f.srv.Fetch(mailbox, "Archive"), 1, "no second copy")
	})
	t.Run("flagged by someone else: left alone", func(t *testing.T) {
		f, uid := setup(t)
		f.srv.Flag(mailbox, "INBOX", uid, `\Deleted`)
		res, err := f.labelOnce(t, map[string]any{"message_id": "<m1@x>", "move_to": "Archive"})
		require.NoError(t, err)
		assert.Equal(t, "not_found", res.Outcome)
		assert.Len(t, f.srv.Fetch(mailbox, "INBOX"), 1, "the message the user flagged is still there")
	})
	t.Run("flagged by a move to another folder: left alone", func(t *testing.T) {
		f, uid := setup(t)
		f.srv.Flag(mailbox, "INBOX", uid, `\Deleted`, moveKeyword("Elsewhere"))
		res, err := f.labelOnce(t, map[string]any{"message_id": "<m1@x>", "move_to": "Archive"})
		require.NoError(t, err)
		assert.Equal(t, "not_found", res.Outcome)
		assert.Len(t, f.srv.Fetch(mailbox, "INBOX"), 1)
	})
	t.Run("a move that completes leaves the record on the original when it stays flagged", func(t *testing.T) {
		f := newFx(t, emailtest.ModeTLS)
		f.srv.DisableMove()
		f.srv.DisableUIDPlus()
		f.srv.Mailbox(mailbox, "Archive")
		f.srv.Deliver(mailbox, "INBOX", mailText("<m1@x>", "a@example.org", "one", "x"))
		_, err := f.labelOnce(t, map[string]any{"message_id": "<m1@x>", "move_to": "Archive"})
		require.NoError(t, err)
		inbox := f.srv.Fetch(mailbox, "INBOX")
		require.Len(t, inbox, 1)
		assert.Contains(t, lower(inbox[0].Flags), lower([]string{moveKeyword("Archive")})[0])
	})
}
