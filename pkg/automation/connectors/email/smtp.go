package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// connTimeout bounds one exchange with a server when the context has no deadline of its own.
const connTimeout = 60 * time.Second

// watch closes conn when ctx ends, so a blocked read returns; the returned func stops watching.
func watch(ctx context.Context, conn net.Conn) func() {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(connTimeout)
	}
	_ = conn.SetDeadline(deadline)
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()
	return func() { close(done) }
}

// dialConn opens a TCP connection, TLS from the first byte when implicit.
func (h *hub) dialConn(ctx context.Context, a account, implicit bool) (net.Conn, error) {
	d := h.dialer()
	if implicit {
		td := &tls.Dialer{NetDialer: d, Config: h.tlsConfig(a.host)}
		return td.DialContext(ctx, "tcp", a.addr())
	}
	return d.DialContext(ctx, "tcp", a.addr())
}

// smtpClient is an open, secured and authenticated SMTP session.
type smtpClient struct {
	*smtp.Client
	conn net.Conn
	stop func()
}

func (s *smtpClient) Close() error {
	s.stop()
	return s.Client.Close()
}

// dialSMTP connects, secures (implicit TLS on 465, otherwise STARTTLS is required: a login never crosses
// the network in the clear) and authenticates. token is the access token of an OAuth account.
func (h *hub) dialSMTP(ctx context.Context, a account, token string) (*smtpClient, error) {
	implicit := a.implicitTLS(false)
	conn, err := h.dialConn(ctx, a, implicit)
	if err != nil {
		return nil, err
	}
	stop := watch(ctx, conn)
	var cl *smtp.Client
	if implicit {
		cl = smtp.NewClient(conn)
	} else if cl, err = smtp.NewClientStartTLS(conn, h.tlsConfig(a.host)); err != nil {
		stop()
		_ = conn.Close()
		return nil, fmt.Errorf("email: the server %s does not secure the connection with STARTTLS: %w", a.host, noText(err))
	}
	if w := newMaskedWriter(h.cfg.Debug, a.password, token); w != nil {
		cl.DebugWriter = w
	}
	s := &smtpClient{Client: cl, conn: conn, stop: stop}
	if err := s.authenticate(a, token); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

func (s *smtpClient) authenticate(a account, token string) error {
	var auth sasl.Client
	switch {
	case a.oauth():
		if !s.SupportsAuth("XOAUTH2") {
			return errors.New("email: the server does not offer XOAUTH2 (status 400)")
		}
		auth = newXOAuth2(a.username, token)
	case a.username == "" && a.password == "":
		return nil // a relay that takes mail without a login (the platform's own)
	case s.SupportsAuth("PLAIN"):
		auth = sasl.NewPlainClient("", a.username, a.password)
	case s.SupportsAuth("LOGIN"):
		auth = sasl.NewLoginClient(a.username, a.password)
	default:
		return errors.New("email: the server offers no AUTH PLAIN or LOGIN (status 400)")
	}
	if err := s.Auth(auth); err != nil {
		var se *smtp.SMTPError
		if errors.As(err, &se) {
			if se.Code >= 500 { // 535 bad credentials, 534/530 policy
				return refused("smtp", se.Code)
			}
			return &catalog.CallError{Code: catalog.CallUnavailable, Message: fmt.Sprintf("the server answered %d to the login", se.Code), Retryable: true}
		}
		return err
	}
	return nil
}

// noText drops the server's text from an error: it may echo what was sent.
func noText(err error) error {
	var se *smtp.SMTPError
	if errors.As(err, &se) {
		return fmt.Errorf("the server answered %d", se.Code)
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return errors.New("a network error")
	}
	return errors.New(err.Error())
}

// sendResult is what the server did with a message.
type sendResult struct {
	accepted, rejected []string
}

// stage is how far a delivery got; a failure at or after stageData may have delivered the message.
type stage int

const (
	stageConnect stage = iota
	stageEnvelope
	stageData
)

// deliver sends raw from `from` to rcpts and reports, per recipient, what the server said.
//
// A 4xx anywhere before the message is accepted — also for one recipient among several — is a refusal
// that delivered nothing (unavailable, retryable). A 5xx for every recipient is rejected; for some of
// them, the others are sent and the refused ones are listed. A connection that ends after the message was written and before the server's
// answer to it leaves the outcome unknown: the message may have been taken.
func (h *hub) deliver(ctx context.Context, a account, token, from string, rcpts []string, raw []byte) (sendResult, error) {
	cl, err := h.dialSMTP(ctx, a, token)
	if err != nil {
		return sendResult{}, mapTransport(err, stageConnect)
	}
	defer func() { _ = cl.Close() }()
	if err := cl.Mail(from, nil); err != nil {
		return sendResult{}, mapTransport(err, stageEnvelope)
	}
	var res sendResult
	var temporary *smtp.SMTPError
	for _, to := range rcpts {
		err := cl.Rcpt(to, nil)
		var se *smtp.SMTPError
		switch {
		case err == nil:
			res.accepted = append(res.accepted, to)
		case errors.As(err, &se) && se.Code >= 500:
			res.rejected = append(res.rejected, to)
		case errors.As(err, &se):
			temporary = se
			res.rejected = append(res.rejected, to)
		default:
			return sendResult{}, mapTransport(err, stageEnvelope)
		}
	}
	if temporary != nil {
		// A recipient the server may take later: nothing is sent, so the call can be made again whole; a
		// message that went to the others would be sent twice to them.
		_ = cl.Reset()
		return sendResult{}, &catalog.CallError{Code: catalog.CallUnavailable, Message: fmt.Sprintf("the server answered %d to a recipient", temporary.Code), Retryable: true}
	}
	if len(res.accepted) == 0 {
		return res, &catalog.CallError{Code: catalog.CallRejected, Message: "the server refused every recipient"}
	}
	w, err := cl.Data()
	if err != nil {
		return sendResult{}, mapTransport(err, stageEnvelope)
	}
	// A failure while the message is written leaves the server without its end marker: nothing was
	// delivered. Only the end of the message, and the wait for the server's answer to it, are in doubt.
	if _, err := w.Write(raw); err != nil {
		return sendResult{}, mapTransport(err, stageEnvelope)
	}
	if _, err := w.CloseWithResponse(); err != nil {
		return sendResult{}, mapTransport(err, stageData)
	}
	_ = cl.Quit()
	return res, nil
}

// mapTransport is the CallError of a failure at a stage of the delivery.
func mapTransport(err error, at stage) error {
	var ce *catalog.CallError
	if errors.As(err, &ce) {
		return err
	}
	var se *smtp.SMTPError
	if errors.As(err, &se) {
		switch {
		case se.Code >= 500:
			return &catalog.CallError{Code: catalog.CallRejected, Message: fmt.Sprintf("the server answered %d", se.Code)}
		default:
			return &catalog.CallError{Code: catalog.CallUnavailable, Message: fmt.Sprintf("the server answered %d", se.Code), Retryable: true}
		}
	}
	if at == stageData {
		return &catalog.CallError{Code: catalog.CallOutcomeUnknown, Message: "the connection ended while the message was sent: it may have been delivered"}
	}
	return &catalog.CallError{Code: catalog.CallUnavailable, Message: "the server could not be reached", Retryable: true}
}
