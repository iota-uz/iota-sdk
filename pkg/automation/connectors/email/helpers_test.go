package email

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/email/emailtest"
)

const (
	mailbox  = "robot@example.com"
	password = "app-password-1"
)

var tenant = uuid.MustParse("00000000-0000-0000-0000-0000000000a1")

// memUploads is an uploads storage in memory.
type memUploads struct {
	mu    sync.Mutex
	files map[string]UploadFile
	fail  bool
}

func (m *memUploads) Save(_ context.Context, _ uuid.UUID, f UploadFile) (Stored, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return Stored{}, fmt.Errorf("disk full")
	}
	if m.files == nil {
		m.files = map[string]UploadFile{}
	}
	id := fmt.Sprint(len(m.files) + 1)
	m.files[id] = f
	return Stored{ID: id, URL: "https://app.example/uploads/" + id}, nil
}

// usage records what a call spent.
type usage struct {
	mu      sync.Mutex
	entries []catalog.UsageEntry
}

func (u *usage) Record(_ context.Context, e catalog.UsageEntry) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.entries = append(u.entries, e)
	return nil
}

// fx is a fake server and the two connectors pointed at it.
type fx struct {
	srv     *emailtest.Server
	smtp    *smtpConnector
	imap    *imapConnector
	uploads *memUploads
	usage   *usage
	oauth   *fakeOAuth
	conns   *fakeConns
}

func newFx(t *testing.T, mode emailtest.Mode) *fx {
	t.Helper()
	srv := emailtest.New(t, mode, emailtest.Account{User: mailbox, Password: password})
	f := &fx{srv: srv, uploads: &memUploads{}, usage: &usage{}, oauth: &fakeOAuth{tokens: map[uuid.UUID]string{}}, conns: &fakeConns{}}
	providers := map[string]Provider{PresetGmail: {SMTPHost: "127.0.0.1", SMTPPort: atoi(port(srv.SMTPAddr)), IMAPHost: "127.0.0.1", IMAPPort: atoi(port(srv.IMAPAddr)), Security: srv.Security()}}
	cs := New(Config{Providers: providers, TLSConfig: srv.TLS, Uploads: f.uploads, OAuth: f.oauth, Connections: f.conns, Clock: func() time.Time { return time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC) }})
	f.smtp, f.imap = cs[0].(*smtpConnector), cs[1].(*imapConnector)
	return f
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func port(addr string) string { _, p := emailtest.HostPort(addr); return p }

// smtpCreds and imapCreds are the credentials of the account with a login and a password.
func (f *fx) smtpCreds() connectors.Credentials {
	return connectors.Credentials{"preset": "custom", "host": "127.0.0.1", "port": port(f.srv.SMTPAddr), "security": f.srv.Security(),
		"username": mailbox, "password": password, "from_address": mailbox, "from_name": "Robot"}
}

func (f *fx) imapCreds() connectors.Credentials {
	return connectors.Credentials{"preset": "custom", "host": "127.0.0.1", "port": port(f.srv.IMAPAddr), "security": f.srv.Security(),
		"username": mailbox, "password": password, "mailbox": "INBOX"}
}

// oauthSMTP are the credentials of an OAuth account: no preset host, so the fake's address is given by the
// test through the custom path with a token.
func (f *fx) oauthCreds(addr, token string) connectors.Credentials {
	return connectors.Credentials{"preset": "gmail", "username": mailbox, "access_token": token, "login_id": "login-1"}
}

func (f *fx) connection(kind catalog.ConnectorKind) connectors.Connection {
	return connectors.Connection{ID: uuid.New(), TenantID: tenant, Kind: kind, Name: string(kind), Identity: connectors.Identity{External: mailbox}}
}

// fakeOAuth is an OAuth2 that serves a fixed token per connection and fails when asked to.
type fakeOAuth struct {
	mu     sync.Mutex
	tokens map[uuid.UUID]string
	revoke bool
}

func (o *fakeOAuth) Begin(context.Context, uuid.UUID, catalog.ConnectorKind, string) (string, string, error) {
	return "", "", fmt.Errorf("not used")
}
func (o *fakeOAuth) Complete(context.Context, uuid.UUID, string, string) (connectors.Connection, error) {
	return connectors.Connection{}, fmt.Errorf("not used")
}
func (o *fakeOAuth) Exchange(context.Context, uuid.UUID, string, string) (connectors.OAuth2Grant, error) {
	return connectors.OAuth2Grant{}, fmt.Errorf("not used")
}
func (o *fakeOAuth) Token(_ context.Context, _ uuid.UUID, id uuid.UUID) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.revoke {
		return "", connectors.ErrNeedsReauth
	}
	return o.tokens[id], nil
}

// fakeConns is the store of connections the connector marks siblings in.
type fakeConns struct {
	mu      sync.Mutex
	conns   map[uuid.UUID]connectors.Connection
	creds   map[uuid.UUID]connectors.Credentials
	status  map[uuid.UUID]connectors.ConnectionStatus
	inbound map[uuid.UUID]json.RawMessage
}

func (s *fakeConns) add(c connectors.Connection, creds connectors.Credentials) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conns == nil {
		s.conns, s.creds, s.status = map[uuid.UUID]connectors.Connection{}, map[uuid.UUID]connectors.Credentials{}, map[uuid.UUID]connectors.ConnectionStatus{}
	}
	c.Status = connectors.StatusConnected
	s.conns[c.ID], s.creds[c.ID], s.status[c.ID] = c, creds, connectors.StatusConnected
}

func (s *fakeConns) List(_ context.Context, _ uuid.UUID, kind catalog.ConnectorKind) ([]connectors.Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []connectors.Connection
	for id, c := range s.conns {
		if c.Kind == kind {
			c.Status = s.status[id]
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *fakeConns) Get(_ context.Context, _, id uuid.UUID) (connectors.Connection, connectors.Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conns[id], s.creds[id], nil
}

func (s *fakeConns) SetStatus(_ context.Context, _, id uuid.UUID, st connectors.ConnectionStatus, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status[id] = st
	return nil
}

func (s *fakeConns) UpdateInbound(_ context.Context, _, id uuid.UUID, fn func(json.RawMessage) (json.RawMessage, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.inbound == nil {
		s.inbound = map[uuid.UUID]json.RawMessage{}
	}
	next, err := fn(s.inbound[id])
	if err != nil {
		return err
	}
	s.inbound[id] = next
	return nil
}

func (s *fakeConns) statusOf(id uuid.UUID) connectors.ConnectionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status[id]
}

// mailText is a plain message to the mailbox.
func mailText(id, from, subject, body string, extra ...string) []byte {
	h := []string{"From: " + from, "To: " + mailbox, "Subject: " + subject, "Date: Mon, 01 Jan 2026 10:00:00 +0000"}
	if id != "" {
		h = append(h, "Message-ID: "+id)
	}
	h = append(h, extra...)
	h = append(h, "MIME-Version: 1.0", "Content-Type: text/plain; charset=utf-8", "", body)
	return []byte(strings.Join(h, "\r\n") + "\r\n")
}

func must[T any](t *testing.T, v T, err error) T {
	t.Helper()
	require.NoError(t, err)
	return v
}
