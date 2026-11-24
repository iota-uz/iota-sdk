package email

import (
	"bytes"
	"context"
	"fmt"
	"testing"

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

// A message past the whole-message limit is read by its structure: each part is reported with its size and
// the limit it is over, and no part body is downloaded.
func TestOversizedMessageReportsItsAttachments(t *testing.T) {
	old := maxRaw
	maxRaw = 1 << 20
	t.Cleanup(func() { maxRaw = old })
	f := newFx(t, emailtest.ModeTLS)
	conn, creds := f.connection(KindIMAP), f.imapCreds()
	_, cur := f.pollOnce(t, conn, creds, nil)
	small := bytes.Repeat([]byte("s"), 2<<20)
	big := bytes.Repeat([]byte("b"), MaxAttachment+(1<<20))
	f.srv.Deliver(mailbox, "INBOX", multipart("<huge@x>", "Huge", "The covering text.", "<p>html</p>",
		att{"application/pdf", "small.pdf", small}, att{"application/zip", "big.zip", big}))
	evs, _ := f.pollOnce(t, conn, creds, cur)
	require.Len(t, evs, 1)
	p := payloadOf(t, evs[0])
	assert.Equal(t, "Huge", p["subject"])
	assert.Equal(t, "The covering text.", p["text"])
	assert.Equal(t, true, p["hasHtml"])
	atts := p["attachments"].([]any)
	require.Len(t, atts, 2, "attachments are listed, not lost")
	first, second := atts[0].(map[string]any), atts[1].(map[string]any)
	assert.Equal(t, "small.pdf", first["filename"])
	assert.Equal(t, "application/pdf", first["contentType"])
	assert.InDelta(t, len(small), first["size"], float64(len(small))/10)
	assert.Equal(t, AttachmentMessageTooBig, first["error"], "a message this large is past the attachment budget of one message")
	assert.Equal(t, "big.zip", second["filename"])
	assert.Equal(t, AttachmentTooLarge, second["error"])
	assert.Nil(t, second["url"])
	assert.Empty(t, f.uploads.files, "no body was downloaded and nothing was stored")
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
		f.srv.Deliver(mailbox, "INBOX", mailText("<m1@x>", "a@example.org", "one", "x"))
		f.srv.Deliver(mailbox, "INBOX", mailText("<m2@x>", "a@example.org", "two", "x"))
		_, err := f.labelOnce(t, map[string]any{"message_id": "<m2@x>", "keywords": []any{"keep"}})
		require.NoError(t, err)
		// m1 is flagged deleted by someone else, awaiting its own expunge.
		_, err = f.labelOnce(t, map[string]any{"message_id": "<m1@x>", "keywords": []any{"stale"}})
		require.NoError(t, err)
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
