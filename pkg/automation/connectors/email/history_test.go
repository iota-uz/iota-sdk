package email

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/backtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email/emailtest"
)

// The history of a mailbox is the messages sent within the window, as the events the poller publishes;
// the mailbox is read-only: nothing is marked read.
func TestImportHistoryReadsTheWindow(t *testing.T) {
	f := newFx(t, emailtest.ModeTLS)
	// Bob's message arrives first but was sent later: the import emits by date.
	f.srv.Deliver(mailbox, "INBOX", datedMail("<b@x>", "Bob <bob@example.org>", "Broken", "it broke", "Tue, 06 Jan 2026 10:00:00 +0000"))
	f.srv.Deliver(mailbox, "INBOX", datedMail("<a@x>", "Ann <ann@example.org>", "Order", "I want to buy", "Mon, 05 Jan 2026 10:00:00 +0000"))
	f.srv.Deliver(mailbox, "INBOX", datedMail("<old@x>", "Old <old@example.org>", "Old", "too old", "Mon, 01 Dec 2025 10:00:00 +0000"))
	conn, creds := f.connection(KindIMAP), f.imapCreds()

	var got []backtest.Event
	w := backtest.TimeWindow{From: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}
	err := f.imap.ImportHistory(context.Background(), backtest.ConnectionRef{Connection: conn, Credentials: creds}, w, nil, func(ev backtest.Event) error {
		got = append(got, ev)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, EventReceived, got[0].Kind)
	assert.Equal(t, "<a@x>", got[0].Key)
	assert.Equal(t, "Order", got[0].Title)
	assert.Equal(t, time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC), got[0].OccurredAt.UTC())
	var p map[string]any
	require.NoError(t, json.Unmarshal(got[1].Payload, &p))
	assert.Equal(t, "it broke", p["text"])

	// A filter narrows the import; emit's error stops it.
	got = nil
	err = f.imap.ImportHistory(context.Background(), backtest.ConnectionRef{Connection: conn, Credentials: creds}, w, json.RawMessage(`{"from": "bob"}`), func(ev backtest.Event) error {
		got = append(got, ev)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "<b@x>", got[0].Key)
	// An error of emit stops the import: it is returned and no message comes after it.
	stop := errors.New("dataset full")
	calls := 0
	err = f.imap.ImportHistory(context.Background(), backtest.ConnectionRef{Connection: conn, Credentials: creds}, w, nil, func(backtest.Event) error {
		calls++
		return stop
	})
	require.ErrorIs(t, err, stop)
	assert.Equal(t, 1, calls)

	// A window that starts at 00:30 +14:00 still finds a message sent an hour into it at 23:30 −12:00 the
	// calendar day before. The server received it on January 8 (its INTERNALDATE, the clock of the server
	// behind the sender's), so neither date the server compares is on the window's first day.
	f.srv.DeliverAt(mailbox, "INBOX", datedMail("<z@x>", "Zed <z@example.org>", "Zones", "far east", "Thu, 08 Jan 2026 23:30:00 -1200"),
		time.Date(2026, 1, 8, 23, 0, 0, 0, time.UTC))
	east := time.FixedZone("+14", 14*3600)
	got = nil
	err = f.imap.ImportHistory(context.Background(), backtest.ConnectionRef{Connection: conn, Credentials: creds},
		backtest.TimeWindow{From: time.Date(2026, 1, 10, 0, 30, 0, 0, east), To: time.Date(2026, 1, 10, 2, 0, 0, 0, east)}, nil, func(ev backtest.Event) error {
			got = append(got, ev)
			return nil
		})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "<z@x>", got[0].Key)
	for _, m := range f.srv.Fetch(mailbox, "INBOX") {
		assert.NotContains(t, m.Flags, `\Seen`, "the import marks nothing read")
	}
}

func datedMail(id, from, subject, body, date string) []byte {
	h := []string{"From: " + from, "To: " + mailbox, "Subject: " + subject, "Date: " + date, "Message-ID: " + id,
		"MIME-Version: 1.0", "Content-Type: text/plain; charset=utf-8", "", body}
	return []byte(strings.Join(h, "\r\n") + "\r\n")
}
