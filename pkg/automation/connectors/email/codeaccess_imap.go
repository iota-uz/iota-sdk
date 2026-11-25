package email

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/emersion/go-imap/v2"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// The code operations of an imap connection (env.connections.<name>.<operation>): both only read.
const (
	OpListMessages = "listMessages"
	OpGetMessage   = "getMessage"
)

const (
	// defaultListLimit and maxListLimit bound the messages one listMessages returns.
	defaultListLimit = 20
	maxListLimit     = 50
	// MaxCodeBody is the longest text, and the longest html, getMessage returns, in bytes.
	MaxCodeBody = 256 << 10
)

var (
	_ connectors.CodeAccessor = (*imapConnector)(nil)
	_ connectors.CodeOperator = (*imapConnector)(nil)
)

// CodeAccess offers the operations listMessages and getMessage: a read of the connection's mailbox, which
// never changes it (a message stays unseen, nothing is moved or deleted).
func (c *imapConnector) CodeAccess() connectors.CodeAccess {
	return connectors.CodeAccess{Operations: []connectors.CodeOperation{
		{
			Name:        OpListMessages,
			Description: "List the newest messages of a mailbox folder (headers only; since is the arrival date), without marking them read.",
			Params:      `{ mailbox?: string; unseen?: boolean; since?: string; limit?: number }`,
			Result:      `{ messages: { uid: number; message_id: string; from: { address: string; name?: string }; to: { address: string; name?: string }[]; cc: { address: string; name?: string }[]; subject: string; date: string; seen: boolean; size: number }[] }`,
		},
		{
			Name:        OpGetMessage,
			Description: "Read one message by its uid, with its text, html and the list of its attachments, without marking it read.",
			Params:      `{ uid: number; mailbox?: string }`,
			Result:      `{ uid: number; message_id: string; from: { address: string; name?: string }; to: { address: string; name?: string }[]; cc: { address: string; name?: string }[]; subject: string; date: string; text: string; html?: string; truncated: boolean; attachments: { filename: string; content_type: string; size: number }[] }`,
		},
	}}
}

// Authorize sets nothing: the operations read over IMAP with the connection's own credentials.
func (c *imapConnector) Authorize(context.Context, connectors.Credentials, http.Header) error {
	return nil
}

type listArgs struct {
	Mailbox string `json:"mailbox"`
	Unseen  bool   `json:"unseen"`
	Since   string `json:"since"`
	Limit   *int   `json:"limit"`
}

type getArgs struct {
	UID     *uint32 `json:"uid"`
	Mailbox string  `json:"mailbox"`
}

// decodeArgs reads the object of an operation; unknown fields are refused.
func decodeArgs(raw json.RawMessage, into any, fields string) error {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		raw = []byte("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return invalidArgs("the argument is not an object of " + fields + ": " + strings.TrimPrefix(err.Error(), "json: "))
	}
	if dec.More() {
		return invalidArgs("the argument is not an object of " + fields)
	}
	return nil
}

// folderArg is the mailbox the call names, INBOX by default.
func folderArg(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return "INBOX", nil
	case len(name) > 255 || !utf8.ValidString(name) || strings.ContainsAny(name, "\x00\r\n"):
		return "", invalidArgs("mailbox is not a folder name")
	}
	return name, nil
}

// CallCodeOperation reads the mailbox with the poller's own path: the same account, token, sign-in and
// handling of a refused login.
func (c *imapConnector) CallCodeOperation(ctx context.Context, call connectors.CodeCall) (any, error) {
	switch call.Operation {
	case OpListMessages:
		var args listArgs
		if err := decodeArgs(call.Args, &args, "mailbox, unseen, since and limit"); err != nil {
			return nil, err
		}
		mailbox, err := folderArg(args.Mailbox)
		if err != nil {
			return nil, err
		}
		limit := defaultListLimit
		if args.Limit != nil {
			if limit = *args.Limit; limit < 1 || limit > maxListLimit {
				return nil, invalidArgs("limit is a number from 1 to 50")
			}
		}
		var since time.Time
		if args.Since != "" {
			var ok bool
			if since, ok = parseSince(args.Since); !ok {
				return nil, invalidArgs("since is an ISO date (2026-01-31) or a time (2026-01-31T10:00:00Z)")
			}
		}
		return c.codeRead(ctx, call, mailbox, func(s *imapSession) (any, error) {
			return c.listMessages(s, args.Unseen, since, limit)
		})
	case OpGetMessage:
		var args getArgs
		if err := decodeArgs(call.Args, &args, "uid and mailbox"); err != nil {
			return nil, err
		}
		if args.UID == nil || *args.UID == 0 {
			return nil, invalidArgs("uid is a positive number")
		}
		mailbox, err := folderArg(args.Mailbox)
		if err != nil {
			return nil, err
		}
		uid := imap.UID(*args.UID)
		return c.codeRead(ctx, call, mailbox, func(s *imapSession) (any, error) {
			return c.getMessage(ctx, s, uid)
		})
	}
	return nil, &connectors.CodeOperationError{Code: "unknown_operation", Message: "the connection has no operation " + call.Operation}
}

func parseSince(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// codeRead opens the session as a poll does, selects the folder read-only and runs fn.
func (c *imapConnector) codeRead(ctx context.Context, call connectors.CodeCall, mailbox string, fn func(*imapSession) (any, error)) (any, error) {
	in := call.Input
	if !in.Deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, in.Deadline)
		defer cancel()
	}
	a, err := c.parseIMAP(in.Credentials)
	if err != nil {
		return nil, &connectors.CodeOperationError{Code: "provider_refused", Message: "the connection's credentials are not usable"}
	}
	token, err := c.callToken(ctx, in, a)
	if err != nil {
		return nil, imapCodeFailure(err)
	}
	s, err := c.openIMAP(ctx, a, token)
	if err != nil {
		var conn connectors.Connection
		conn.TenantID = in.TenantID
		if in.Connection != nil {
			conn.ID, _ = uuid.Parse(in.Connection.ID)
		}
		c.noteRefusal(ctx, conn, in.Credentials, a, token, err)
		return nil, imapCodeFailure(err)
	}
	defer s.close()
	if _, err := s.selectMailbox(mailbox, true); err != nil {
		var ce *catalog.CallError
		if errors.As(err, &ce) && ce.Code == catalog.CallInvalidRequest {
			return nil, &connectors.CodeOperationError{Code: "not_found", Message: "the mailbox " + mailbox + " is not available"}
		}
		return nil, imapCodeFailure(err)
	}
	out, err := fn(s)
	if err != nil {
		return nil, imapCodeFailure(err)
	}
	return out, nil
}

// imapCodeFailure turns a failure of the read into what code handles; the server's words are never in it.
func imapCodeFailure(err error) error {
	var coe *connectors.CodeOperationError
	if errors.As(err, &coe) {
		return coe
	}
	var ce *catalog.CallError
	if errors.As(err, &ce) {
		switch ce.Code {
		case catalog.CallInvalidRequest:
			return &connectors.CodeOperationError{Code: "invalid_args", Message: ce.Message}
		case catalog.CallRateLimited:
			return &connectors.CodeOperationError{Code: "rate_limited", Message: ce.Message, Retryable: true}
		case catalog.CallUnavailable:
			return &connectors.CodeOperationError{Code: "unavailable", Message: ce.Message, Retryable: true}
		}
		return &connectors.CodeOperationError{Code: "provider_refused", Message: ce.Message}
	}
	return &connectors.CodeOperationError{Code: "unavailable", Message: "the IMAP server could not be read", Retryable: true}
}

func notFound() error {
	return &connectors.CodeOperationError{Code: "not_found", Message: "the mailbox has no message with this uid"}
}

// listed is one message of listMessages.
type listed struct {
	UID       uint32   `json:"uid"`
	MessageID string   `json:"message_id"`
	From      person   `json:"from"`
	To        []person `json:"to"`
	Cc        []person `json:"cc"`
	Subject   string   `json:"subject"`
	Date      string   `json:"date"`
	Seen      bool     `json:"seen"`
	Size      int64    `json:"size"`
}

func (c *imapConnector) listMessages(s *imapSession, unseen bool, since time.Time, limit int) (any, error) {
	crit := &imap.SearchCriteria{Since: since}
	if unseen {
		crit.NotFlag = []imap.Flag{imap.FlagSeen}
	}
	found, err := s.c.UIDSearch(crit, nil).Wait()
	if err != nil {
		return nil, err
	}
	uids := found.AllUIDs()
	sort.Slice(uids, func(i, j int) bool { return uids[i] > uids[j] })
	if len(uids) > limit {
		uids = uids[:limit]
	}
	out := make([]listed, 0, len(uids))
	if len(uids) > 0 {
		head := &imap.FetchItemBodySection{Peek: true, Specifier: imap.PartSpecifierHeader}
		bufs, err := s.c.Fetch(imap.UIDSetNum(uids...), &imap.FetchOptions{
			UID: true, Flags: true, RFC822Size: true, InternalDate: true, BodySection: []*imap.FetchItemBodySection{head},
		}).Collect()
		if err != nil {
			return nil, err
		}
		for _, b := range bufs {
			var m received
			if len(b.BodySection) > 0 {
				m, _ = headerOnly(b.BodySection[0].Bytes)
			}
			at := m.date
			if at.IsZero() {
				at = b.InternalDate
			}
			l := listed{UID: uint32(b.UID), MessageID: m.messageID, From: m.from, To: nonNil(m.to), Cc: nonNil(m.cc),
				Subject: m.subject, Date: dateOf(at), Size: b.RFC822Size}
			for _, f := range b.Flags {
				l.Seen = l.Seen || f == imap.FlagSeen
			}
			out = append(out, l)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].UID > out[j].UID })
	}
	return map[string]any{"messages": out}, nil
}

func dateOf(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

type attachmentInfo struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

// capBytes cuts s to n bytes on a character boundary.
func capBytes(s string, n int) (string, bool) {
	if len(s) <= n {
		return s, false
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n], true
}

func (c *imapConnector) getMessage(ctx context.Context, s *imapSession, uid imap.UID) (any, error) {
	sizes, err := s.sizes([]imap.UID{uid})
	if err != nil {
		return nil, err
	}
	size, ok := sizes[uid]
	if !ok {
		return nil, notFound()
	}
	var m received
	if size > maxRaw {
		var missing bool
		if m, missing, err = c.oversized(ctx, s, uid); err != nil {
			return nil, err
		} else if missing {
			return nil, notFound()
		}
	} else {
		raw, err := s.fetchRaw(uid, false)
		if err != nil {
			return nil, err
		}
		if raw == nil {
			return nil, notFound()
		}
		if m, err = parseMessage(raw); err != nil {
			return nil, notFound()
		}
	}
	text, cut := capBytes(m.text, MaxCodeBody)
	// parseMessage keeps MaxText characters: a text that long was most likely cut there.
	truncated := cut || utf8.RuneCountInString(text) >= MaxText
	res := map[string]any{
		"uid": uint32(uid), "message_id": m.messageID, "from": m.from, "to": nonNil(m.to), "cc": nonNil(m.cc),
		"subject": m.subject, "date": dateOf(m.date), "text": text,
	}
	if m.html != "" {
		html, cutHTML := capBytes(m.html, MaxCodeBody)
		res["html"] = html
		truncated = truncated || cutHTML
	}
	atts := make([]attachmentInfo, 0, len(m.attachment))
	for _, p := range m.attachment {
		atts = append(atts, attachmentInfo{Filename: p.filename, ContentType: p.contentType, Size: p.size})
	}
	res["attachments"] = atts
	res["truncated"] = truncated
	return res, nil
}
