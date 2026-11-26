package email

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"

	"github.com/iota-uz/iota-sdk/pkg/automation/backtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

var _ backtest.HistoryImporter = (*imapConnector)(nil)

// historyFilter narrows an import of a mailbox's history: another folder of the account, and messages
// whose From or Subject header contains the given text.
type historyFilter struct {
	Mailbox string `json:"mailbox"`
	From    string `json:"from"`
	Subject string `json:"subject"`
}

// historyEnvelopeBatch is how many messages one FETCH ENVELOPE reads.
const historyEnvelopeBatch = 200

// ImportHistory reads the messages of the mailbox sent within the window, oldest first, as the events
// email.received.v1 the poller would have published when they arrived (with their attachments stored as
// a poll stores them). The mailbox is opened read-only: no message is marked read or moved.
func (c *imapConnector) ImportHistory(ctx context.Context, ref backtest.ConnectionRef, w backtest.TimeWindow, raw json.RawMessage, emit func(backtest.Event) error) error {
	var f historyFilter
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &f); err != nil {
			return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the filter of a mailbox import is {mailbox, from, subject}"}
		}
	}
	conn, creds := ref.Connection, ref.Credentials
	a, err := c.parseIMAP(creds)
	if err != nil {
		return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the connection's credentials are not usable: " + err.Error()}
	}
	if m := strings.TrimSpace(f.Mailbox); m != "" {
		a.mailbox = m
	}
	token := ""
	if a.oauth() {
		if token, err = c.accessToken(ctx, conn.TenantID, conn.ID.String(), creds); err != nil {
			return err
		}
	}
	s, err := c.openIMAP(ctx, a, token)
	if err != nil {
		c.noteRefusal(ctx, conn, creds, a, token, err)
		return err
	}
	defer s.close()
	sel, err := s.selectMailbox(a.mailbox, true)
	if err != nil {
		return err
	}
	// The server compares dates only. A message is a candidate when it arrived since the day before the
	// window (arrival follows sending) or its Date header is since then (an APPEND may set the arrival
	// apart from the header); each message's own date narrows the candidates below.
	// The dates compared have no offset: from UTC, two calendar days back covers every zone (−12 to +14).
	since := w.From.UTC().AddDate(0, 0, -2)
	criteria := &imap.SearchCriteria{Or: [][2]imap.SearchCriteria{{{Since: since}, {SentSince: since}}}}
	if v := strings.TrimSpace(f.From); v != "" {
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{Key: "From", Value: v})
	}
	if v := strings.TrimSpace(f.Subject); v != "" {
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{Key: "Subject", Value: v})
	}
	res, err := s.c.UIDSearch(criteria, nil).Wait()
	if err != nil {
		return err
	}
	uids := res.AllUIDs()
	sort.Slice(uids, func(i, j int) bool { return uids[i] < uids[j] })
	// The envelopes of all candidates first: the messages in the window are emitted in the order of their
	// date (UID breaks ties), not of their arrival in the mailbox, so a full dataset keeps the earliest.
	type candidate struct {
		uid  imap.UID
		at   time.Time
		size int64
	}
	var picked []candidate
	for start := 0; start < len(uids); start += historyEnvelopeBatch {
		batch := uids[start:min(start+historyEnvelopeBatch, len(uids))]
		bufs, err := s.c.Fetch(imap.UIDSetNum(batch...), &imap.FetchOptions{UID: true, Envelope: true, RFC822Size: true, InternalDate: true}).Collect()
		if err != nil {
			return err
		}
		for _, b := range bufs {
			at := b.InternalDate
			if b.Envelope != nil && !b.Envelope.Date.IsZero() {
				at = b.Envelope.Date
			}
			if !at.IsZero() && (at.Before(w.From) || !at.Before(w.To)) {
				continue
			}
			picked = append(picked, candidate{uid: b.UID, at: at, size: b.RFC822Size})
		}
	}
	sort.SliceStable(picked, func(i, j int) bool {
		if !picked[i].at.Equal(picked[j].at) {
			return picked[i].at.Before(picked[j].at)
		}
		return picked[i].uid < picked[j].uid
	})
	for _, mt := range picked {
		if err := ctx.Err(); err != nil {
			return err
		}
		uid := mt.uid
		var (
			m       received
			missing bool
		)
		if mt.size > maxRaw {
			m, missing, err = c.oversized(ctx, s, uid)
		} else {
			var body []byte
			if body, err = s.fetchRaw(uid, false); err == nil && body == nil {
				missing = true
			} else if err == nil {
				if m, err = parseMessage(body); err != nil {
					err, missing = nil, true
				}
			}
		}
		if err != nil {
			return err
		}
		if missing {
			continue
		}
		if m.date.IsZero() {
			m.date = mt.at
		}
		if !m.date.IsZero() && (m.date.Before(w.From) || !m.date.Before(w.To)) {
			continue
		}
		ev := c.event(ctx, conn, a, sel.UIDValidity, uid, m)
		if err := emit(backtest.Event{Kind: ev.Event, Key: ev.DedupKey, Title: m.subject, Subject: ev.Subject, Payload: ev.Payload,
			OccurredAt: ev.OccurredAt, BySelf: ev.BySelf}); err != nil {
			return err
		}
	}
	return nil
}
