package email

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	if w := newMaskedWriter(h.debugOut(), a.password, token); w != nil {
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
		// [UNAVAILABLE] (RFC 5530) is the server's backend being down for a moment, not a refusal of the
		// credentials: it is retried, and nothing is marked as needing a new sign-in.
		if ie.Code == imap.ResponseCodeUnavailable {
			return &catalog.CallError{Code: catalog.CallUnavailable, Message: "the IMAP server is temporarily unavailable", Retryable: true}
		}
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

// moveTo moves a message to another mailbox without ever losing one. With native MOVE (RFC 6851) it is one
// atomic command. Without it the message is copied first, and only when the server has answered the COPY
// is the original flagged \\Deleted, and then expunged by UID when the server offers UIDPLUS, so no other
// message that happens to be flagged is removed. (The client library's own fallback queues COPY, STORE and
// a plain EXPUNGE without waiting for the COPY, and would delete the source when the copy fails.) Without
// UIDPLUS the original stays in its mailbox flagged \\Deleted.
//
// Nothing but the server's answer to this call's own COPY is taken as proof that a message was copied: a
// message of the destination that has the same Message-ID is not (it may be another message, or the user's
// own), and several messages of the source with one Message-ID are not moved at all (which of them a
// destination copy stands for cannot be told). A call that is made again after the connection ended between
// the COPY and the last step (the node is repeated by the engine) copies again, so the destination may end
// up with a second copy, never with none; resumeMove finishes the case where the original is already
// flagged.
func (s *imapSession) moveTo(set imap.UIDSet, dest string) error {
	caps := s.c.Caps()
	if caps.Has(imap.CapMove) {
		_, err := s.c.Move(set, dest).Wait()
		return err
	}
	if nums, _ := set.Nums(); len(nums) > 1 {
		return &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "more than one message has this Message-ID: it is not moved"}
	}
	if _, err := s.c.Copy(set, dest).Wait(); err != nil {
		return err
	}
	// The original is flagged \\Deleted together with a keyword that names this move: it is the record that the
	// COPY of this very message was answered, which a repeat after an interrupted move needs (resumeMove). A
	// server that does not keep the keyword gets the flag alone, and the move then stays at the flag.
	flags := []imap.Flag{imap.FlagDeleted, imap.Flag(moveKeyword(dest))}
	if err := s.c.Store(set, &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: flags}, nil).Close(); err != nil {
		if err := s.c.Store(set, &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: flags[:1]}, nil).Close(); err != nil {
			return err
		}
	}
	if caps.Has(imap.CapUIDPlus) {
		return s.c.UIDExpunge(set).Close()
	}
	return nil
}

// moveKeyword is the keyword a move to dest puts on the original after its COPY was answered. It carries 128 bits
// of the destination's hash, so two folders are told apart for all practical purposes (32 bits would let
// different folders share one).
func moveKeyword(dest string) string {
	sum := sha256.Sum256([]byte(dest))
	return "$IotaMoved" + hex.EncodeToString(sum[:16])
}

// resumeMove finishes a move whose last step did not happen: the original was flagged \\Deleted by a move to
// this destination, and its UID EXPUNGE did not happen. The evidence that this workflow copied the message is
// on the original itself: the keyword moveTo puts on it together with \\Deleted once the COPY was answered
// (a message the user deleted, or that another tool flagged, does not have it, and is left alone). Only on a
// server with UIDPLUS (the flag-only behavior without it stays), only the messages that are exactly this one,
// only while the destination still holds a live copy, and only in the mailbox instance the messages were found
// in (UIDVALIDITY). It reports whether there was something to finish.
func (s *imapSession) resumeMove(source, dest, messageID string) (bool, error) {
	if !s.c.Caps().Has(imap.CapUIDPlus) || s.c.Caps().Has(imap.CapMove) {
		return false, nil
	}
	sel, err := s.selectMailbox(source, false)
	if err != nil {
		return false, err
	}
	validity := sel.UIDValidity
	found, err := s.c.UIDSearch(&imap.SearchCriteria{
		Header: []imap.SearchCriteriaHeaderField{{Key: "Message-ID", Value: messageID}},
		Flag:   []imap.Flag{imap.FlagDeleted, imap.Flag(moveKeyword(dest))},
	}, nil).Wait()
	if err != nil {
		return false, err
	}
	candidates := found.AllUIDs()
	if len(candidates) == 0 {
		return false, nil
	}
	bufs, err := s.c.Fetch(imap.UIDSetNum(candidates...), &imap.FetchOptions{UID: true, Envelope: true}).Collect()
	if err != nil {
		return false, err
	}
	var exact []imap.UID
	for _, b := range bufs {
		if b.Envelope != nil && bracket(b.Envelope.MessageID) == bracket(messageID) {
			exact = append(exact, b.UID)
		}
	}
	if len(exact) == 0 {
		return false, nil
	}
	copied, err := s.hasMessage(dest, messageID)
	if err != nil || !copied {
		return false, err
	}
	sel, err = s.selectMailbox(source, false)
	if err != nil {
		return false, err
	}
	if sel.UIDValidity != validity {
		return false, nil // the mailbox was made again meanwhile: a UID names another message now
	}
	return true, s.c.UIDExpunge(imap.UIDSetNum(exact...)).Close()
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
