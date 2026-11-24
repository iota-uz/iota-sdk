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
