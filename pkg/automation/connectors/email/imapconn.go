package email

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// imapSession is an open, secured and authenticated IMAP session.
type imapSession struct {
	c    *imapclient.Client
	stop func()
}

func (s *imapSession) close() {
	if s == nil {
		return
	}
	_ = s.c.Logout().Wait()
	s.stop()
	_ = s.c.Close()
}

// openIMAP connects (implicit TLS on 993, otherwise STARTTLS is required), and logs in. token is the
// access token of an OAuth account.
func (h *hub) openIMAP(ctx context.Context, a account, token string) (*imapSession, error) {
	implicit := a.implicitTLS(true)
	conn, err := h.dialConn(ctx, a, implicit)
	if err != nil {
		return nil, err
	}
	stop := watch(ctx, conn)
	opts := &imapclient.Options{TLSConfig: h.tlsConfig(a.host)}
	if w := newMaskedWriter(h.cfg.Debug, a.password, token); w != nil {
		opts.DebugWriter = w
	}
	var c *imapclient.Client
	if implicit {
		c = imapclient.New(conn, opts)
	} else if c, err = imapclient.NewStartTLS(conn, opts); err != nil {
		stop()
		_ = conn.Close()
		return nil, fmt.Errorf("email: the server %s does not secure the connection with STARTTLS: %w", a.host, errors.New("no STARTTLS"))
	}
	s := &imapSession{c: c, stop: stop}
	if err := c.WaitGreeting(); err != nil {
		s.close()
		return nil, networkOrRefused(err, ctx)
	}
	var auth error
	switch {
	case a.oauth():
		auth = c.Authenticate(newXOAuth2(a.username, token))
	default:
		auth = c.Login(a.username, a.password).Wait()
	}
	if auth != nil {
		s.close()
		return nil, networkOrRefused(auth, ctx)
	}
	return s, nil
}

// networkOrRefused tells a refusal of the credentials (the server answered NO or BAD to the login) from a
// network failure; the server's text is dropped.
func networkOrRefused(err error, ctx context.Context) error {
	var ie *imap.Error
	if errors.As(err, &ie) {
		if ie.Type == imap.StatusResponseTypeNo || ie.Type == imap.StatusResponseTypeBad {
			return refused("imap", 401)
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return errors.New("email: a network error talking to the IMAP server")
	}
	return fmt.Errorf("email: the IMAP server failed: %s", imapText(err))
}

// imapText is an IMAP error without the server's words in it.
func imapText(err error) string {
	var ie *imap.Error
	if errors.As(err, &ie) {
		return strings.ToLower(string(ie.Type))
	}
	return "an error"
}

func (s *imapSession) selectMailbox(name string, readOnly bool) (*imap.SelectData, error) {
	d, err := s.c.Select(name, &imap.SelectOptions{ReadOnly: readOnly}).Wait()
	if err != nil {
		var ie *imap.Error
		if errors.As(err, &ie) && ie.Type == imap.StatusResponseTypeNo {
			return nil, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the mailbox " + name + " is not available"}
		}
		return nil, err
	}
	return d, nil
}

var _ sasl.Client = (*xoauth2)(nil)

// moveTo moves messages to another mailbox without ever losing or doubling one. With native MOVE (RFC 6851)
// it is one atomic command. Without it the messages are copied first and the copy is checked: only then are
// the originals flagged \\Deleted, and expunged by UID when the server offers UIDPLUS, so no other message
// that happens to be flagged is removed. (The client library's own fallback queues COPY, STORE and a plain
// EXPUNGE without waiting for the COPY, and would delete the source when the copy fails.) Without UIDPLUS
// the originals stay in their mailbox flagged \\Deleted.
//
// The steps are repeatable: a call that is made again after the connection ended in the middle of them
// (the node is idempotent) finds the message already in the destination, by its Message-ID, and does not
// copy it a second time.
func (s *imapSession) moveTo(set imap.UIDSet, source, dest, messageID string) error {
	caps := s.c.Caps()
	if caps.Has(imap.CapMove) {
		_, err := s.c.Move(set, dest).Wait()
		return err
	}
	// Is it there already (a copy of an earlier attempt)? A destination that is not there is refused here,
	// before anything is changed.
	validity, err := s.selectValidity(source)
	if err != nil {
		return err
	}
	copied, err := s.hasMessage(dest, messageID)
	if err != nil {
		return err
	}
	sel, err := s.selectMailbox(source, false)
	if err != nil {
		return err
	}
	if sel.UIDValidity != validity {
		return errors.New("email: the mailbox changed while a message was moved")
	}
	if !copied {
		if _, err := s.c.Copy(set, dest).Wait(); err != nil {
			return err
		}
	}
	if err := s.c.Store(set, &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: []imap.Flag{imap.FlagDeleted}}, nil).Close(); err != nil {
		return err
	}
	if caps.Has(imap.CapUIDPlus) {
		return s.c.UIDExpunge(set).Close()
	}
	return nil
}

// selectValidity is the UIDVALIDITY of the mailbox (opened read-write, as the caller has it).
func (s *imapSession) selectValidity(name string) (uint32, error) {
	sel, err := s.selectMailbox(name, false)
	if err != nil {
		return 0, err
	}
	return sel.UIDValidity, nil
}

// hasMessage reports whether a mailbox holds a live copy of the message: a message that is not flagged
// \\Deleted and whose Message-ID is exactly the one asked for (read-only; it fails for a mailbox the server
// does not have).
func (s *imapSession) hasMessage(mailbox, messageID string) (bool, error) {
	if _, err := s.selectMailbox(mailbox, true); err != nil {
		return false, err
	}
	uids, err := s.findExact(messageID)
	return len(uids) > 0, err
}

// findExact is the live messages of the selected mailbox whose Message-ID is exactly messageID. SEARCH
// HEADER matches a substring of the field, and a message flagged \\Deleted is on its way out: the search
// narrows, the envelope decides, and a deleted message is never counted.
func (s *imapSession) findExact(messageID string) ([]imap.UID, error) {
	found, err := s.c.UIDSearch(&imap.SearchCriteria{
		Header:  []imap.SearchCriteriaHeaderField{{Key: "Message-ID", Value: messageID}},
		NotFlag: []imap.Flag{imap.FlagDeleted},
	}, nil).Wait()
	if err != nil {
		return nil, err
	}
	candidates := found.AllUIDs()
	if len(candidates) == 0 {
		return nil, nil
	}
	bufs, err := s.c.Fetch(imap.UIDSetNum(candidates...), &imap.FetchOptions{UID: true, Flags: true, Envelope: true}).Collect()
	if err != nil {
		return nil, err
	}
	var out []imap.UID
	for _, b := range bufs {
		if b.Envelope == nil || bracket(b.Envelope.MessageID) != bracket(messageID) {
			continue
		}
		deleted := false
		for _, f := range b.Flags {
			deleted = deleted || f == imap.FlagDeleted
		}
		if !deleted {
			out = append(out, b.UID)
		}
	}
	return out, nil
}
