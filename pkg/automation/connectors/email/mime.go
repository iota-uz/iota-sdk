package email

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/emersion/go-imap/v2"
	"io"
	"mime"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset" // decoders of the charsets mail comes in
	gomail "github.com/emersion/go-message/mail"
	"golang.org/x/net/html"
)

// MaxText is the longest text of an incoming message the event carries, in characters.
const MaxText = 100_000

// ---------------------------------------------------------------------------------------------
// outgoing

// outgoing is a message to send.
type outgoing struct {
	From      mail.Address
	To, Cc    []string
	ReplyTo   string
	Subject   string
	Text      string
	HTML      string
	MessageID string // with angle brackets
	InReplyTo string // with angle brackets, or empty
	Date      time.Time
}

// messageIDFor is the Message-ID of the message a call sends: <{hash of the idempotency key}@{domain of the
// sender}>, the same on every attempt of the call, generated before anything is sent, so the id is known
// even when the answer is lost.
func messageIDFor(idempotencyKey, fromAddress string) string {
	sum := sha256.Sum256([]byte(idempotencyKey))
	domain := "localhost"
	if i := strings.LastIndexByte(fromAddress, '@'); i >= 0 && i < len(fromAddress)-1 {
		domain = fromAddress[i+1:]
	}
	return "<" + hex.EncodeToString(sum[:16]) + "@" + domain + ">"
}

// bracket is a Message-ID in angle brackets; empty stays empty.
func bracket(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if !strings.HasPrefix(id, "<") {
		id = "<" + id
	}
	if !strings.HasSuffix(id, ">") {
		id += ">"
	}
	return id
}

func unbracket(id string) string {
	return strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(id), "<"), ">")
}

var lineBreaks = strings.NewReplacer("\r", " ", "\n", " ")

// buildMessage is the RFC 5322 form of m: text, or text and HTML as alternatives, in UTF-8.
func buildMessage(m outgoing) ([]byte, error) {
	var h gomail.Header
	h.SetDate(m.Date)
	h.SetAddressList("From", []*gomail.Address{{Name: m.From.Name, Address: m.From.Address}})
	h.SetAddressList("To", addresses(m.To))
	if len(m.Cc) > 0 {
		h.SetAddressList("Cc", addresses(m.Cc))
	}
	if m.ReplyTo != "" {
		h.SetAddressList("Reply-To", addresses([]string{m.ReplyTo}))
	}
	h.SetSubject(lineBreaks.Replace(m.Subject))
	h.SetMessageID(unbracket(m.MessageID))
	if m.InReplyTo != "" {
		h.SetMsgIDList("In-Reply-To", []string{unbracket(m.InReplyTo)})
		h.SetMsgIDList("References", []string{unbracket(m.InReplyTo)})
	}
	h.Set("MIME-Version", "1.0")
	var buf bytes.Buffer
	if m.HTML == "" {
		w, err := gomail.CreateSingleInlineWriter(&buf, h)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(w, m.Text); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	w, err := gomail.CreateInlineWriter(&buf, h)
	if err != nil {
		return nil, err
	}
	for _, p := range []struct{ typ, body string }{{"text/plain", m.Text}, {"text/html", m.HTML}} {
		var ph gomail.InlineHeader
		ph.SetContentType(p.typ, map[string]string{"charset": "utf-8"})
		pw, err := w.CreatePart(ph)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(pw, p.body); err != nil {
			return nil, err
		}
		if err := pw.Close(); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func addresses(list []string) []*gomail.Address {
	out := make([]*gomail.Address, 0, len(list))
	for _, a := range list {
		out = append(out, &gomail.Address{Address: a})
	}
	return out
}

// ---------------------------------------------------------------------------------------------
// incoming

// person is an address of an incoming message.
type person struct {
	Address string `json:"address"`
	Name    string `json:"name,omitempty"`
}

// part is an attachment as the message carries it: its name, type and the bytes (nil when they were not
// read because the part was past the limit; size is then the real size).
type part struct {
	filename    string
	contentType string
	size        int64
	data        []byte
	// reason is why the part was not read (an attachments[].error code), when it was not.
	reason string
}

// received is an incoming message as the connector reads it.
type received struct {
	messageID  string // with angle brackets; empty when the message has none
	inReplyTo  string
	references []string
	from       person
	to, cc     []person
	subject    string
	text       string
	hasHTML    bool
	attachment []part
	auto       bool
	date       time.Time
}

var errNotMail = errors.New("email: the message cannot be read")

// parseMessage reads a message. HTML is turned into text and dropped; attachments are read into memory up
// to MaxAttachment each (their size is counted past it) and never opened. A message whose body cannot be
// read still gives its headers.
func parseMessage(raw []byte) (received, error) {
	r, err := gomail.CreateReader(bytes.NewReader(raw))
	if err != nil && r == nil {
		return received{}, errNotMail
	}
	defer func() { _ = r.Close() }()
	m := headersOf(r.Header)
	var plain, htmlText string
	var sawPlain, sawHTML bool
	for {
		p, err := r.NextPart()
		if err != nil && p == nil {
			break // io.EOF, or a part that cannot be read: what was read stands
		}
		var hdr message.Header
		switch h := p.Header.(type) {
		case *gomail.InlineHeader:
			hdr = h.Header
		case *gomail.AttachmentHeader:
			hdr = h.Header
		}
		typ, _, _ := hdr.ContentType()
		typ = strings.ToLower(typ)
		disp, params, _ := hdr.ContentDisposition()
		name := params["filename"]
		if name == "" {
			_, ctParams, _ := hdr.ContentType()
			name = ctParams["name"]
		}
		textual := typ == "text/plain" || typ == "text/html"
		switch {
		case textual && disp != "attachment" && name == "":
			b, _ := io.ReadAll(io.LimitReader(p.Body, 4<<20))
			switch {
			case typ == "text/plain" && !sawPlain:
				plain, sawPlain = string(b), true
			case typ == "text/html":
				m.hasHTML = true
				if !sawHTML {
					htmlText, sawHTML = htmlToText(string(b)), true
				}
			}
		default:
			if typ == "" {
				typ = "application/octet-stream"
			}
			a := part{filename: name, contentType: typ}
			data, _ := io.ReadAll(io.LimitReader(p.Body, MaxAttachment+1))
			a.size = int64(len(data))
			if a.size > MaxAttachment {
				n, _ := io.Copy(io.Discard, p.Body)
				a.size += n
			} else {
				a.data = data
			}
			m.attachment = append(m.attachment, a)
		}
	}
	switch {
	case sawPlain && strings.TrimSpace(plain) != "":
		m.text = plain
	case sawHTML:
		m.text = htmlText
	default:
		m.text = plain
	}
	m.text = clip(normalizeText(m.text), MaxText)
	return m, nil
}

// headerOnly is the message of a header block alone (a message too large to read whole).
func headerOnly(raw []byte) (received, error) {
	e, err := message.Read(bytes.NewReader(raw))
	if err != nil && e == nil {
		return received{}, errNotMail
	}
	return headersOf(gomail.Header{Header: e.Header}), nil
}

func headersOf(h gomail.Header) received {
	var m received
	if id, err := h.MessageID(); err == nil && id != "" {
		m.messageID = bracket(id)
	}
	if ids, err := h.MsgIDList("In-Reply-To"); err == nil && len(ids) > 0 {
		m.inReplyTo = bracket(ids[len(ids)-1])
	}
	if ids, err := h.MsgIDList("References"); err == nil {
		for _, id := range ids {
			m.references = append(m.references, bracket(id))
		}
	}
	m.from = firstPerson(h, "From")
	m.to = persons(h, "To")
	m.cc = persons(h, "Cc")
	m.subject, _ = h.Subject()
	if d, err := h.Date(); err == nil {
		m.date = d
	}
	m.auto = isAuto(h)
	if m.references == nil {
		m.references = []string{}
	}
	return m
}

func persons(h gomail.Header, key string) []person {
	list, _ := h.AddressList(key)
	out := make([]person, 0, len(list))
	for _, a := range list {
		if a.Address != "" {
			out = append(out, person{Address: a.Address, Name: a.Name})
		}
	}
	return out
}

func firstPerson(h gomail.Header, key string) person {
	if l := persons(h, key); len(l) > 0 {
		return l[0]
	}
	return person{}
}

// isAuto: a message a person did not write — Auto-Submitted (but "no"), a mailing list (List-Id), bulk
// mail (Precedence: bulk, list, junk).
func isAuto(h gomail.Header) bool {
	if v := strings.ToLower(strings.TrimSpace(h.Get("Auto-Submitted"))); v != "" && v != "no" {
		return true
	}
	if strings.TrimSpace(h.Get("List-Id")) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(h.Get("Precedence"))) {
	case "bulk", "list", "junk":
		return true
	}
	return false
}

var replySubject = regexp.MustCompile(`(?i)^\s*(re|ответ|aw)\s*(\[\d+\])?\s*:`)

// isReply: a message in a thread — it answers another (In-Reply-To), or its subject says so (Re:, Ответ:).
func isReply(m received) bool { return m.inReplyTo != "" || replySubject.MatchString(m.subject) }

// threadRoot is the Message-ID that names the conversation of a message: the first of References, else
// In-Reply-To, else its own.
func threadRoot(m received) string {
	switch {
	case len(m.references) > 0:
		return m.references[0]
	case m.inReplyTo != "":
		return m.inReplyTo
	}
	return m.messageID
}

func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}

// clip cuts s to n characters, on a character boundary.
func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// htmlToText is the text of an HTML document: block elements start a new line, script and style are
// dropped, entities are decoded. Nothing is fetched or executed.
func htmlToText(src string) string {
	var b strings.Builder
	z := html.NewTokenizer(strings.NewReader(src))
	skip := 0
	for {
		switch tt := z.Next(); tt {
		case html.ErrorToken:
			return collapseBlank(b.String())
		case html.TextToken:
			if skip == 0 {
				b.WriteString(string(z.Text()))
			}
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			tag := string(name)
			switch tag {
			case "script", "style", "head", "title":
				if tt == html.StartTagToken {
					skip++
				} else if tt == html.EndTagToken && skip > 0 {
					skip--
				}
			case "br", "p", "div", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "table", "ul", "ol", "hr":
				if skip == 0 {
					b.WriteString("\n")
				}
			}
		}
	}
}

var blankRuns = regexp.MustCompile(`\n[ \t]*(\n[ \t]*)+`)

func collapseBlank(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.Join(strings.Fields(l), " ")
	}
	return strings.TrimSpace(blankRuns.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

// oversized reads a message past the whole-message limit by its structure: the header block, the text of
// its first text part (the first megabyte) and every other part. A part within the limits (25 MB decoded,
// 50 MB of attachments in the message) is fetched by itself, as it would be from a smaller message, so a
// large text body does not cost a small image its place; a part over a limit is reported with it
// (too_large, message_too_big) and its body is never downloaded.
func (c *imapConnector) oversized(_ context.Context, s *imapSession, uid imap.UID) (received, bool, error) {
	head, err := s.fetchRaw(uid, true)
	if err != nil {
		return received{}, false, err
	}
	if head == nil {
		return received{}, true, nil
	}
	m, err := headerOnly(head)
	if err != nil {
		return received{}, true, nil //nolint:nilerr // a message without a readable header is skipped, as a removed one
	}
	bufs, err := s.c.Fetch(imap.UIDSetNum(uid), &imap.FetchOptions{UID: true, BodyStructure: &imap.FetchItemBodyStructure{Extended: true}}).Collect()
	if err != nil {
		return received{}, false, err
	}
	var structure imap.BodyStructure
	for _, b := range bufs {
		if b.UID == uid {
			structure = b.BodyStructure
		}
	}
	if structure == nil {
		return m, false, nil
	}
	var textPath []int
	var textPart *imap.BodyStructureSinglePart
	type located struct {
		path []int
		sp   *imap.BodyStructureSinglePart
	}
	var where []located
	structure.Walk(func(path []int, bs imap.BodyStructure) bool {
		sp, ok := bs.(*imap.BodyStructureSinglePart)
		if !ok {
			return true
		}
		name := sp.Filename()
		kind := sp.MediaType()
		if (kind == "text/plain" || kind == "text/html") && name == "" && (sp.Disposition() == nil || sp.Disposition().Value != "attachment") {
			if textPart == nil || (kind == "text/plain" && textPart.MediaType() != "text/plain") {
				textPath, textPart = append([]int(nil), path...), sp
			}
			if kind == "text/html" {
				m.hasHTML = true
			}
			return true
		}
		size := int64(sp.Size)
		if strings.EqualFold(sp.Encoding, "base64") {
			size = size / 4 * 3
		}
		m.attachment = append(m.attachment, part{filename: name, contentType: kind, size: size})
		where = append(where, located{path: append([]int(nil), path...), sp: sp})
		return true
	})
	var total int64
	for i := range m.attachment {
		a := &m.attachment[i]
		switch {
		case a.size > MaxAttachment:
			a.reason = AttachmentTooLarge
		case total+a.size > MaxAttachmentsPerMsg:
			a.reason = AttachmentMessageTooBig
		default:
			data, err := s.fetchDecoded(uid, where[i].path, where[i].sp, 0)
			if err != nil {
				return received{}, false, err
			}
			if int64(len(data)) > MaxAttachment {
				a.reason, a.size = AttachmentTooLarge, int64(len(data))
				continue
			}
			a.data, a.size = data, int64(len(data))
			total += a.size
		}
	}
	if textPart != nil {
		if body, err := s.fetchDecoded(uid, textPath, textPart, 1<<20); err == nil {
			text := string(body)
			if textPart.MediaType() == "text/html" {
				text = htmlToText(text)
			}
			m.text = clip(normalizeText(text), MaxText)
		} else if !errors.Is(err, errNotMail) {
			return received{}, false, err
		}
	}
	return m, false, nil
}

// fetchDecoded reads a part of a message by itself (the first limit bytes when limit > 0) and decodes its
// transfer encoding and charset.
func (s *imapSession) fetchDecoded(uid imap.UID, path []int, sp *imap.BodyStructureSinglePart, limit int64) ([]byte, error) {
	section := &imap.FetchItemBodySection{Peek: true, Part: path}
	if limit > 0 {
		section.Partial = &imap.SectionPartial{Offset: 0, Size: limit}
	}
	bufs, err := s.c.Fetch(imap.UIDSetNum(uid), &imap.FetchOptions{UID: true, BodySection: []*imap.FetchItemBodySection{section}}).Collect()
	if err != nil {
		return nil, err
	}
	var raw []byte
	for _, b := range bufs {
		if b.UID == uid && len(b.BodySection) > 0 {
			raw = b.BodySection[0].Bytes
		}
	}
	var h message.Header
	h.Set("Content-Type", mime.FormatMediaType(sp.MediaType(), sp.Params))
	if sp.Encoding != "" {
		h.Set("Content-Transfer-Encoding", sp.Encoding)
	}
	e, err := message.New(h, bytes.NewReader(raw))
	if err != nil && e == nil {
		return nil, errNotMail
	}
	return io.ReadAll(io.LimitReader(e.Body, MaxAttachment+1))
}
