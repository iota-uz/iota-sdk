// Package emailtest is a fake mail server for tests: an SMTP server that keeps what it was given and an
// IMAP server over an in-memory mailbox, with login and password, SASL XOAUTH2, TLS from the first byte or
// STARTTLS, and scripted failures (a refused recipient, a refused login, a connection that drops after the
// message was written). It is for the tests of the email connector and of the graphs that use it.
package emailtest

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// Mode is how a listener secures its connections.
type Mode int

const (
	// ModeTLS: TLS from the first byte (ports 465 and 993 of real servers).
	ModeTLS Mode = iota
	// ModeSTARTTLS: plain, then STARTTLS (587, 143).
	ModeSTARTTLS
)

// Account is a mailbox of the server.
type Account struct {
	User     string
	Password string
	// Tokens are the access tokens SASL XOAUTH2 accepts for the account.
	Tokens map[string]bool
}

// Sent is a message the SMTP server took.
type Sent struct {
	From  string
	Rcpts []string
	Data  []byte
	User  string // who logged in
}

// Server is a fake SMTP and IMAP server on loopback.
type Server struct {
	SMTPAddr, IMAPAddr string
	// TLS is the client configuration that trusts the server's certificate.
	TLS *tls.Config

	t    *testing.T
	mode Mode

	mu       sync.Mutex
	accounts map[string]*Account
	sent     []Sent
	rcptCode map[string]int // address (lower) → reply code
	dataCode int            // reply code of a message (0: accepted)
	dropData bool           // close the connection after the message was written, before the answer
	imapMem  *imapmemserver.Server
	users    map[string]*imapmemserver.User
	logins   []string // mechanisms the servers saw, "smtp:PLAIN", "imap:XOAUTH2"

	listeners []net.Listener
	smtpSrv   *smtp.Server
	imapSrv   *imapserver.Server
	imapCaps  imap.CapSet
	failFetch int
}

// New starts the servers; they stop with the test.
func New(t *testing.T, mode Mode, accounts ...Account) *Server {
	t.Helper()
	cert, pool := certificate(t)
	s := &Server{
		t: t, mode: mode, accounts: map[string]*Account{}, rcptCode: map[string]int{},
		imapMem: imapmemserver.New(), users: map[string]*imapmemserver.User{},
		TLS: &tls.Config{RootCAs: pool, ServerName: "localhost", MinVersion: tls.VersionTLS12},
	}
	serverTLS := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	for _, a := range accounts {
		s.AddAccount(a)
	}
	s.smtpSrv = smtp.NewServer(smtp.BackendFunc(func(c *smtp.Conn) (smtp.Session, error) { return &smtpSession{s: s, conn: c.Conn()}, nil }))
	s.smtpSrv.Domain = "localhost"
	s.smtpSrv.TLSConfig = serverTLS
	s.smtpSrv.MaxRecipients = 200
	s.smtpSrv.ReadTimeout, s.smtpSrv.WriteTimeout = 30*time.Second, 30*time.Second
	s.imapCaps = imap.CapSet{imap.CapIMAP4rev1: {}, imap.CapIMAP4rev2: {}, imap.CapMove: {}, imap.CapUIDPlus: {}}
	s.imapSrv = imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			// The connection reads imapCaps after its session is made; taking mu here orders that read
			// after a DisableMove or DisableUIDPlus made before the client dialed.
			s.mu.Lock()
			defer s.mu.Unlock()
			return &imapSession{Session: s.imapMem.NewSession(), s: s}, nil, nil
		},
		Caps:      s.imapCaps,
		TLSConfig: serverTLS,
	})
	s.SMTPAddr = s.listen(mode, serverTLS, func(l net.Listener) { _ = s.smtpSrv.Serve(l) })
	s.IMAPAddr = s.listen(mode, serverTLS, func(l net.Listener) { _ = s.imapSrv.Serve(l) })
	t.Cleanup(s.Close)
	return s
}

func (s *Server) listen(mode Mode, cfg *tls.Config, serve func(net.Listener)) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		s.t.Fatal(err)
	}
	if mode == ModeTLS {
		l = tls.NewListener(l, cfg)
	}
	s.listeners = append(s.listeners, l)
	go serve(l)
	return l.Addr().String()
}

// Close stops the servers.
func (s *Server) Close() {
	_ = s.smtpSrv.Close()
	_ = s.imapSrv.Close()
	for _, l := range s.listeners {
		_ = l.Close()
	}
}

// FailBodyFetch makes the n-th (1-based, counted from now) fetch of a message body fail once, as a server
// that has a trouble in the middle of a batch does.
func (s *Server) FailBodyFetch(n int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failFetch = n
}

// DisableMove and DisableUIDPlus make the IMAP server stop advertising the extension, as an older server
// does: a client that uses it anyway is wrong.
func (s *Server) DisableMove() {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.imapCaps, imap.CapMove)
	delete(s.imapCaps, imap.CapIMAP4rev2) // which has MOVE and UIDPLUS in it
}
func (s *Server) DisableUIDPlus() {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.imapCaps, imap.CapUIDPlus)
	delete(s.imapCaps, imap.CapIMAP4rev2)
}

// Security is the "security" credential that matches the servers' mode.
func (s *Server) Security() string {
	if s.mode == ModeTLS {
		return "tls"
	}
	return "starttls"
}

// HostPort splits an address of the server.
func HostPort(addr string) (string, string) {
	h, p, _ := net.SplitHostPort(addr)
	return h, p
}

// AddAccount adds a mailbox with an INBOX.
func (s *Server) AddAccount(a Account) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := a
	if cp.Tokens == nil {
		cp.Tokens = map[string]bool{}
	}
	s.accounts[a.User] = &cp
	u := imapmemserver.NewUser(a.User, a.Password)
	if err := u.Create("INBOX", nil); err != nil {
		s.t.Fatal(err)
	}
	s.users[a.User] = u
	s.imapMem.AddUser(u)
}

// AcceptToken makes SASL XOAUTH2 accept a token for an account; RevokeTokens refuses them all.
func (s *Server) AcceptToken(user, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[user].Tokens[token] = true
}

func (s *Server) RevokeTokens(user string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accounts[user].Tokens = map[string]bool{}
}

// RejectRecipient makes the SMTP server answer code (550, 451) to an address.
func (s *Server) RejectRecipient(addr string, code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rcptCode[strings.ToLower(addr)] = code
}

// FailData makes the SMTP server answer code to the message (451: try later, 554: refused).
func (s *Server) FailData(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dataCode = code
}

// DropAfterData makes the SMTP server close the connection once the message was written, before it answers.
func (s *Server) DropAfterData(drop bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropData = drop
}

// Sent is the messages the SMTP server took.
func (s *Server) Sent() []Sent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Sent(nil), s.sent...)
}

// Mechanisms are the login mechanisms the servers saw ("smtp:XOAUTH2", "imap:PLAIN").
func (s *Server) Mechanisms() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.logins...)
}

func (s *Server) seen(proto, mech string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logins = append(s.logins, proto+":"+strings.ToUpper(mech))
}

// Deliver puts a message in a mailbox of an account and returns its UID.
func (s *Server) Deliver(user, mailbox string, raw []byte) uint32 {
	s.t.Helper()
	u := s.users[user]
	if u == nil {
		s.t.Fatalf("emailtest: no account %q", user)
	}
	d, err := u.Append(mailbox, &literal{Reader: bytes.NewReader(raw), n: int64(len(raw))}, &imap.AppendOptions{})
	if err != nil {
		s.t.Fatal(err)
	}
	return uint32(d.UID)
}

// Mailbox creates a mailbox.
func (s *Server) Mailbox(user, name string) {
	s.t.Helper()
	if err := s.users[user].Create(name, nil); err != nil {
		s.t.Fatal(err)
	}
}

// ResetMailbox deletes a mailbox and creates it again: its UIDVALIDITY changes and its messages are gone.
func (s *Server) ResetMailbox(user, name string) {
	s.t.Helper()
	if err := s.users[user].Delete(name); err != nil {
		s.t.Fatal(err)
	}
	if err := s.users[user].Create(name, nil); err != nil {
		s.t.Fatal(err)
	}
}

type literal struct {
	*bytes.Reader
	n int64
}

func (l *literal) Size() int64 { return l.n }

// Flag adds flags to a message of a mailbox.
func (s *Server) Flag(user, mailbox string, uid uint32, flags ...string) {
	s.t.Helper()
	s.withClient(user, mailbox, func(c *imapclient.Client) {
		fl := make([]imap.Flag, len(flags))
		for i, f := range flags {
			fl[i] = imap.Flag(f)
		}
		if err := c.Store(imap.UIDSetNum(imap.UID(uid)), &imap.StoreFlags{Op: imap.StoreFlagsAdd, Silent: true, Flags: fl}, nil).Close(); err != nil {
			s.t.Fatal(err)
		}
	}, false)
}

// Messages are the messages of a mailbox as raw bytes with their flags, in UID order.
type Message struct {
	UID   uint32
	Flags []string
	Raw   []byte
}

// Fetch reads a mailbox through a real IMAP connection to the server's own memory (so flags and moves are
// seen as a client would see them).
func (s *Server) Fetch(user, mailbox string) []Message {
	s.t.Helper()
	var out []Message
	s.withClient(user, mailbox, func(c *imapclient.Client) {
		section := &imap.FetchItemBodySection{Peek: true}
		var all imap.UIDSet
		all.AddRange(1, 0)
		bufs, err := c.Fetch(all, &imap.FetchOptions{UID: true, Flags: true, BodySection: []*imap.FetchItemBodySection{section}}).Collect()
		if err != nil {
			s.t.Fatal(err)
		}
		for _, b := range bufs {
			m := Message{UID: uint32(b.UID)}
			for _, f := range b.Flags {
				m.Flags = append(m.Flags, string(f))
			}
			if len(b.BodySection) > 0 {
				m.Raw = b.BodySection[0].Bytes
			}
			out = append(out, m)
		}
	}, true)
	return out
}

// withClient runs fn on a logged-in IMAP client of the account with a mailbox selected.
func (s *Server) withClient(user, mailbox string, fn func(*imapclient.Client), readOnly bool) {
	s.t.Helper()
	s.mu.Lock()
	pass := s.accounts[user].Password
	s.mu.Unlock()
	var c *imapclient.Client
	var err error
	opts := &imapclient.Options{TLSConfig: s.TLS}
	if s.mode == ModeTLS {
		c, err = imapclient.DialTLS(s.IMAPAddr, opts)
	} else {
		c, err = imapclient.DialStartTLS(s.IMAPAddr, opts)
	}
	if err != nil {
		s.t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if err := c.Login(user, pass).Wait(); err != nil {
		s.t.Fatal(err)
	}
	if _, err := c.Select(mailbox, &imap.SelectOptions{ReadOnly: readOnly}).Wait(); err != nil {
		s.t.Fatal(err)
	}
	fn(c)
}

// ---------------------------------------------------------------------------------------------
// SMTP

type smtpSession struct {
	conn  net.Conn
	s     *Server
	user  string
	from  string
	rcpts []string
}

var _ smtp.AuthSession = (*smtpSession)(nil)

func (x *smtpSession) AuthMechanisms() []string { return []string{"PLAIN", "LOGIN", "XOAUTH2"} }

func (x *smtpSession) Auth(mech string) (sasl.Server, error) {
	x.s.seen("smtp", mech)
	switch strings.ToUpper(mech) {
	case "PLAIN":
		return sasl.NewPlainServer(func(identity, user, pass string) error {
			if !x.s.passwordOK(user, pass) {
				return smtp.ErrAuthFailed
			}
			x.user = user
			return nil
		}), nil
	case "LOGIN":
		return &loginServer{check: func(user, pass string) error {
			if !x.s.passwordOK(user, pass) {
				return smtp.ErrAuthFailed
			}
			x.user = user
			return nil
		}}, nil
	case "XOAUTH2":
		return &xoauthServer{s: x.s, proto: "smtp", on: func(u string) { x.user = u }}, nil
	}
	return nil, smtp.ErrAuthUnknownMechanism
}

func (x *smtpSession) Reset()        { x.from, x.rcpts = "", nil }
func (x *smtpSession) Logout() error { return nil }

func (x *smtpSession) Mail(from string, _ *smtp.MailOptions) error {
	if x.user == "" {
		return smtp.ErrAuthRequired
	}
	x.from = from
	return nil
}

func (x *smtpSession) Rcpt(to string, _ *smtp.RcptOptions) error {
	x.s.mu.Lock()
	code := x.s.rcptCode[strings.ToLower(to)]
	x.s.mu.Unlock()
	if code != 0 {
		return &smtp.SMTPError{Code: code, EnhancedCode: smtp.EnhancedCode{code / 100, 1, 1}, Message: "recipient refused"}
	}
	x.rcpts = append(x.rcpts, to)
	return nil
}

func (x *smtpSession) Data(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	x.s.mu.Lock()
	code, drop := x.s.dataCode, x.s.dropData
	x.s.mu.Unlock()
	if drop {
		// Taken, but never acknowledged: the client cannot tell.
		x.s.mu.Lock()
		x.s.sent = append(x.s.sent, Sent{From: x.from, Rcpts: x.rcpts, Data: data, User: x.user})
		x.s.mu.Unlock()
		_ = x.conn.Close()
		return errDrop
	}
	if code != 0 {
		return &smtp.SMTPError{Code: code, EnhancedCode: smtp.EnhancedCode{code / 100, 0, 0}, Message: "message refused"}
	}
	x.s.mu.Lock()
	x.s.sent = append(x.s.sent, Sent{From: x.from, Rcpts: x.rcpts, Data: data, User: x.user})
	x.s.mu.Unlock()
	return nil
}

var errDrop = errors.New("emailtest: drop the connection")

func (s *Server) passwordOK(user, pass string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.accounts[user]
	return a != nil && a.Password != "" && a.Password == pass
}

// loginServer is the AUTH LOGIN exchange.
type loginServer struct {
	check func(user, pass string) error
	user  string
	step  int
}

func (l *loginServer) Next(resp []byte) ([]byte, bool, error) {
	switch l.step {
	case 0:
		l.step = 1
		return []byte("Username:"), false, nil
	case 1:
		l.user, l.step = string(resp), 2
		return []byte("Password:"), false, nil
	}
	return nil, true, l.check(l.user, string(resp))
}

// xoauthServer is the server side of SASL XOAUTH2.
type xoauthServer struct {
	s      *Server
	proto  string
	on     func(user string)
	failed bool
}

func (x *xoauthServer) Next(resp []byte) ([]byte, bool, error) {
	if x.failed {
		// The client answered the error challenge with an empty response: the exchange ends.
		return nil, true, smtp.ErrAuthFailed
	}
	if resp == nil {
		return []byte{}, false, nil
	}
	var user, token string
	for _, f := range strings.Split(string(resp), "\x01") {
		switch {
		case strings.HasPrefix(f, "user="):
			user = strings.TrimPrefix(f, "user=")
		case strings.HasPrefix(f, "auth=Bearer "):
			token = strings.TrimPrefix(f, "auth=Bearer ")
		}
	}
	x.s.mu.Lock()
	a := x.s.accounts[user]
	ok := a != nil && a.Tokens[token]
	x.s.mu.Unlock()
	if !ok {
		x.failed = true
		b, _ := json.Marshal(map[string]string{"status": "401", "schemes": "Bearer", "scope": "https://mail.google.com/"})
		return b, false, nil
	}
	if x.on != nil {
		x.on(user)
	}
	return nil, true, nil
}

// ---------------------------------------------------------------------------------------------
// IMAP

type imapSession struct {
	imapserver.Session
	s *Server
}

var (
	_ imapserver.SessionSASL      = (*imapSession)(nil)
	_ imapserver.SessionIMAP4rev2 = (*imapSession)(nil)
)

// Namespace and Move are the optional parts of the in-memory session, which embedding its interface hides.
func (x *imapSession) Namespace() (*imap.NamespaceData, error) {
	return x.Session.(imapserver.SessionNamespace).Namespace()
}

func (x *imapSession) Move(w *imapserver.MoveWriter, numSet imap.NumSet, dest string) error {
	return x.Session.(imapserver.SessionMove).Move(w, numSet, dest)
}

func (x *imapSession) Login(user, pass string) error {
	x.s.seen("imap", "LOGIN")
	return x.Session.Login(user, pass)
}

// Fetch fails the body fetch FailBodyFetch names.
func (x *imapSession) Fetch(w *imapserver.FetchWriter, numSet imap.NumSet, options *imap.FetchOptions) error {
	if len(options.BodySection) > 0 {
		x.s.mu.Lock()
		x.s.failFetch--
		fail := x.s.failFetch == 0
		x.s.mu.Unlock()
		if fail {
			return &imap.Error{Type: imap.StatusResponseTypeNo, Text: "temporary failure"}
		}
	}
	return x.Session.Fetch(w, numSet, options)
}

func (x *imapSession) AuthenticateMechanisms() []string { return []string{"PLAIN", "XOAUTH2"} }

func (x *imapSession) Authenticate(mech string) (sasl.Server, error) {
	x.s.seen("imap", mech)
	switch strings.ToUpper(mech) {
	case "PLAIN":
		return sasl.NewPlainServer(func(_, user, pass string) error { return x.Session.Login(user, pass) }), nil
	case "XOAUTH2":
		return &imapXOAuth{xoauthServer: xoauthServer{s: x.s, proto: "imap"}, sess: x}, nil
	}
	return nil, &imap.Error{Type: imap.StatusResponseTypeNo, Text: "SASL mechanism not supported"}
}

type imapXOAuth struct {
	xoauthServer
	sess *imapSession
}

func (x *imapXOAuth) Next(resp []byte) ([]byte, bool, error) {
	if x.failed {
		return nil, true, &imap.Error{Type: imap.StatusResponseTypeNo, Code: imap.ResponseCodeAuthenticationFailed, Text: "Invalid credentials"}
	}
	var user string
	x.xoauthServer.on = func(u string) { user = u }
	ch, done, err := x.xoauthServer.Next(resp)
	if err != nil || !done {
		return ch, done, err
	}
	x.s.mu.Lock()
	pass := x.s.accounts[user].Password
	x.s.mu.Unlock()
	return nil, true, x.sess.Session.Login(user, pass)
}

// ---------------------------------------------------------------------------------------------
// TLS

func certificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		BasicConstraintsValid: true, IsCA: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, pool
}
