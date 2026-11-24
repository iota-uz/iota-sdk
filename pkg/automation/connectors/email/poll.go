package email

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

const (
	// PollInterval is how often a mailbox is polled; the engine polls no more often than 30 s.
	PollInterval = 60 * time.Second
	// maxBatch is how many messages one poll reads; the rest is read by the next one.
	maxBatch = 50
	// maxRaw is the largest message read whole; a larger one is read by its headers only.
	maxRaw = 100 << 20
)

// cursor is where the polling of a mailbox stands: the highest UID read, in the mailbox's UIDVALIDITY.
type cursor struct {
	Mailbox     string `json:"mailbox"`
	UIDValidity uint32 `json:"uidvalidity"`
	LastUID     uint32 `json:"last_uid"`
}

// Interval is how often the connection is polled.
func (c *imapConnector) Interval(connectors.Connection) time.Duration { return PollInterval }

// Poll reads the messages that arrived after the cursor.
//
// The first poll of a connection, and the first after the mailbox's UIDVALIDITY changed (its UIDs mean
// other messages now), only sets the cursor to the last UID: what was in the mailbox before is never
// published. A network failure returns an error and the cursor stays; a refusal of the login returns
// unauthorized (or ErrNeedsReauth), which stops polling until the connection is authorized again.
func (c *imapConnector) Poll(ctx context.Context, conn connectors.Connection, creds connectors.Credentials, raw json.RawMessage) ([]engine.InboundEvent, json.RawMessage, error) {
	a, err := c.parseIMAP(creds)
	if err != nil {
		return nil, nil, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the connection's credentials are not usable: " + err.Error()}
	}
	token := ""
	if a.oauth() {
		if token, err = c.accessToken(ctx, conn.TenantID, conn.ID.String(), creds); err != nil {
			return nil, nil, err
		}
	}
	s, err := c.openIMAP(ctx, a, token)
	if err != nil {
		c.noteRefusal(ctx, conn, creds, a, err)
		return nil, nil, err
	}
	defer s.close()
	sel, err := s.selectMailbox(a.mailbox, true)
	if err != nil {
		return nil, nil, err
	}
	var cur cursor
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &cur)
	}
	if len(raw) == 0 || cur.Mailbox != a.mailbox || cur.UIDValidity != sel.UIDValidity {
		next, err := c.initialCursor(s, a.mailbox, sel)
		return nil, next, err
	}
	uids, err := c.newUIDs(s, cur.LastUID)
	if err != nil {
		return nil, nil, err
	}
	if len(uids) == 0 {
		return nil, raw, nil
	}
	if len(uids) > maxBatch {
		uids = uids[:maxBatch]
	}
	sizes, err := s.sizes(uids)
	if err != nil {
		return nil, nil, err
	}
	var (
		evs  []engine.InboundEvent
		last = cur.LastUID
	)
	for _, uid := range uids {
		msg, err := s.fetchRaw(uid, sizes[uid] > maxRaw)
		if err != nil {
			if len(evs) == 0 && last == cur.LastUID {
				return nil, nil, err
			}
			break // what was read is published and the cursor is where it got to; the rest comes next time
		}
		last = uint32(uid)
		if msg == nil {
			continue // the message was removed meanwhile
		}
		ev, ok := c.event(ctx, conn, a, sel.UIDValidity, uid, msg, sizes[uid] > maxRaw)
		if ok {
			evs = append(evs, ev)
		}
	}
	next, err := json.Marshal(cursor{Mailbox: a.mailbox, UIDValidity: sel.UIDValidity, LastUID: last})
	return evs, next, err
}

// noteRefusal marks the other connection of an OAuth sign-in when the server refused the token.
func (h *hub) noteRefusal(ctx context.Context, conn connectors.Connection, creds connectors.Credentials, a account, err error) {
	var ce *catalog.CallError
	if a.oauth() && errors.As(err, &ce) && ce.Code == catalog.CallUnauthorized {
		h.markLogin(ctx, conn.TenantID, conn.ID, creds)
	}
}

// initialCursor is the cursor at the mailbox's last UID: the highest one ever assigned, so every message
// that arrives later is after it.
func (c *imapConnector) initialCursor(s *imapSession, mailbox string, sel *imap.SelectData) (json.RawMessage, error) {
	last := uint32(0)
	if sel.UIDNext > 0 {
		last = uint32(sel.UIDNext) - 1
	} else if sel.NumMessages > 0 { // a server that does not say UIDNEXT: the highest UID in the mailbox
		res, err := s.c.UIDSearch(&imap.SearchCriteria{}, nil).Wait()
		if err != nil {
			return nil, err
		}
		for _, u := range res.AllUIDs() {
			last = max(last, uint32(u))
		}
	}
	return json.Marshal(cursor{Mailbox: mailbox, UIDValidity: sel.UIDValidity, LastUID: last})
}

// newUIDs are the UIDs above last, ascending. A range "n:*" always names the last message even when its
// UID is below n, so the result is filtered.
func (c *imapConnector) newUIDs(s *imapSession, last uint32) ([]imap.UID, error) {
	var set imap.UIDSet
	set.AddRange(imap.UID(last+1), 0)
	res, err := s.c.UIDSearch(&imap.SearchCriteria{UID: []imap.UIDSet{set}}, nil).Wait()
	if err != nil {
		return nil, err
	}
	var out []imap.UID
	for _, u := range res.AllUIDs() {
		if uint32(u) > last {
			out = append(out, u)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// sizes are the sizes of the messages, to leave the huge ones unread.
func (s *imapSession) sizes(uids []imap.UID) (map[imap.UID]int64, error) {
	set := imap.UIDSetNum(uids...)
	bufs, err := s.c.Fetch(set, &imap.FetchOptions{UID: true, RFC822Size: true}).Collect()
	if err != nil {
		return nil, err
	}
	out := make(map[imap.UID]int64, len(bufs))
	for _, b := range bufs {
		out[b.UID] = b.RFC822Size
	}
	return out, nil
}

// fetchRaw reads a message without marking it read (BODY.PEEK); headersOnly reads its header block alone.
// A message that is gone is nil.
func (s *imapSession) fetchRaw(uid imap.UID, headersOnly bool) ([]byte, error) {
	section := &imap.FetchItemBodySection{Peek: true}
	if headersOnly {
		section.Specifier = imap.PartSpecifierHeader
	}
	bufs, err := s.c.Fetch(imap.UIDSetNum(uid), &imap.FetchOptions{UID: true, BodySection: []*imap.FetchItemBodySection{section}}).Collect()
	if err != nil {
		return nil, err
	}
	for _, b := range bufs {
		if b.UID == uid && len(b.BodySection) > 0 {
			return b.BodySection[0].Bytes, nil
		}
	}
	return nil, nil
}

// event is the inbound event of a message; false when it cannot be read at all.
func (c *imapConnector) event(ctx context.Context, conn connectors.Connection, a account, uidValidity uint32, uid imap.UID, raw []byte, headersOnly bool) (engine.InboundEvent, bool) {
	var (
		m   received
		err error
	)
	if headersOnly {
		m, err = headerOnly(raw)
	} else {
		m, err = parseMessage(raw)
	}
	if err != nil {
		c.log().Warn("email: a message cannot be read", "connection_id", conn.ID.String(), "uid", uint32(uid))
		return engine.InboundEvent{}, false
	}
	position := strconv.FormatUint(uint64(uidValidity), 10) + ":" + strconv.FormatUint(uint64(uid), 10)
	at := m.date
	if at.IsZero() {
		at = c.now()
	}
	connID := conn.ID.String()
	p := payload{
		ConnectionID: connID, Mailbox: a.mailbox, MessageID: m.messageID, InReplyTo: m.inReplyTo,
		ThreadID: threadRoot(m), References: m.references, From: m.from, To: nonNil(m.to), Cc: nonNil(m.cc),
		Subject: m.subject, Text: m.text, HasHTML: m.hasHTML, IsReply: isReply(m), IsAuto: m.auto,
		Attachments: c.store(ctx, conn, m.attachment), ReceivedAt: at.UTC(),
	}
	if p.ThreadID == "" {
		p.ThreadID = position
	}
	body, err := json.Marshal(p)
	if err != nil {
		return engine.InboundEvent{}, false
	}
	dedup := m.messageID
	if dedup == "" {
		dedup = position
	}
	return engine.InboundEvent{
		Event: EventReceived, DedupKey: dedup, Subject: subjectOf(connID, m, position), Payload: body, OccurredAt: at,
		BySelf: sameMailbox(m.from.Address, a.username, conn.Identity.External),
	}, true
}

func sameMailbox(from string, own ...string) bool {
	for _, o := range own {
		if o != "" && strings.EqualFold(strings.TrimSpace(from), strings.TrimSpace(o)) {
			return true
		}
	}
	return false
}

func nonNil(p []person) []person {
	if p == nil {
		return []person{}
	}
	return p
}

// payload is the payload of email.received.v1. The HTML of a message is never in it: only its text.
type payload struct {
	ConnectionID string       `json:"connectionId"`
	Mailbox      string       `json:"mailbox"`
	MessageID    string       `json:"messageId"`
	InReplyTo    string       `json:"inReplyTo,omitempty"`
	ThreadID     string       `json:"threadId"`
	References   []string     `json:"references"`
	From         person       `json:"from"`
	To           []person     `json:"to"`
	Cc           []person     `json:"cc"`
	Subject      string       `json:"subject"`
	Text         string       `json:"text"`
	HasHTML      bool         `json:"hasHtml"`
	IsReply      bool         `json:"isReply"`
	IsAuto       bool         `json:"isAuto"`
	Attachments  []attachment `json:"attachments"`
	ReceivedAt   time.Time    `json:"receivedAt"`
}

// attachment is an attachment as the event carries it: no content, and where it was stored.
type attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	UploadID    string `json:"upload_id,omitempty"`
	URL         string `json:"url,omitempty"`
	Error       string `json:"error,omitempty"`
}

// store saves the attachments of a message in the uploads storage: each up to 25 MB, and 50 MB per
// message. One that cannot be stored is still listed, with the reason: the event is never lost for it.
func (c *imapConnector) store(ctx context.Context, conn connectors.Connection, parts []part) []attachment {
	out := make([]attachment, 0, len(parts))
	var total int64
	for i, p := range parts {
		name := p.filename
		if name == "" {
			name = "attachment-" + strconv.Itoa(i+1)
		}
		a := attachment{Filename: name, ContentType: p.contentType, Size: p.size}
		switch {
		case p.size > MaxAttachment || p.data == nil:
			a.Error = AttachmentTooLarge
		case total+p.size > MaxAttachmentsPerMsg:
			a.Error = AttachmentMessageTooBig
		case c.cfg.Uploads == nil:
			a.Error = AttachmentNoStorage
		default:
			saved, err := c.cfg.Uploads.Save(ctx, conn.TenantID, UploadFile{Name: name, Mime: p.contentType, Data: p.data})
			if err != nil {
				a.Error = AttachmentStorage
			} else {
				a.UploadID, a.URL = saved.ID, saved.URL
				total += p.size
			}
		}
		out = append(out, a)
	}
	return out
}
