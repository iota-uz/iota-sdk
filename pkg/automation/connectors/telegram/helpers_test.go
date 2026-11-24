package telegram_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/telegram/telegramtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

const (
	tenantToken   = "111111:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	platformToken = "222222:BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	origin        = "https://erp.example.com"
)

// inbound is an engine.Inbound that records what is published and knows the open correlations.
type inbound struct {
	mu     sync.Mutex
	open   map[string]uuid.UUID
	events []engine.InboundEvent
}

func (i *inbound) Publish(_ context.Context, _ pgx.Tx, ev engine.InboundEvent) (bool, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.events = append(i.events, ev)
	return true, nil
}

func (i *inbound) Correlation(_ context.Context, token string) (uuid.UUID, bool, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	t, ok := i.open[token]
	return t, ok, nil
}

// uploads is an in-memory uploads store.
type uploads struct {
	mu    sync.Mutex
	files map[string]telegram.UploadFile
	fail  bool
}

func (u *uploads) Save(_ context.Context, _ uuid.UUID, f telegram.UploadFile) (telegram.Stored, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.fail {
		return telegram.Stored{}, errors.New("disk full")
	}
	if u.files == nil {
		u.files = map[string]telegram.UploadFile{}
	}
	id := uuid.NewString()
	u.files[id] = f
	return telegram.Stored{ID: id, URL: "/uploads/" + id}, nil
}

func (u *uploads) Open(_ context.Context, _ uuid.UUID, id string) (telegram.UploadFile, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	f, ok := u.files[id]
	if !ok {
		return telegram.UploadFile{}, errors.New("not found")
	}
	return f, nil
}

// state records the writes of the connector to its connection.
type state struct {
	mu      sync.Mutex
	status  connectors.ConnectionStatus
	reason  string
	inbound json.RawMessage
}

func (s *state) SetStatus(_ context.Context, _, _ uuid.UUID, st connectors.ConnectionStatus, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status, s.reason = st, reason
	return nil
}

func (s *state) UpdateInbound(_ context.Context, _, _ uuid.UUID, fn func(json.RawMessage) (json.RawMessage, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next, err := fn(s.inbound)
	if err != nil {
		return err
	}
	s.inbound = next
	return nil
}

func (s *state) InboundState(context.Context, uuid.UUID, uuid.UUID) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inbound, nil
}

// rig is a connector against the fake Bot API, with a tenant connection and the platform bot.
type rig struct {
	t        *testing.T
	api      *telegramtest.Server
	c        *telegram.Connector
	in       *inbound
	up       *uploads
	st       *state
	tenant   uuid.UUID
	conn     connectors.Connection
	bot      *telegramtest.Bot // the tenant's own
	platform *telegramtest.Bot
}

func newRig(t *testing.T) *rig {
	t.Helper()
	api := telegramtest.New()
	t.Cleanup(api.Close)
	r := &rig{t: t, api: api, in: &inbound{open: map[string]uuid.UUID{}}, up: &uploads{}, st: &state{}, tenant: uuid.New()}
	r.c = telegram.New(telegram.Config{
		PlatformToken: platformToken, PublicOrigin: origin, Inbound: r.in, APIURL: api.URL(),
		Uploads: r.up, Connections: r.st,
	})
	r.conn = connectors.Connection{ID: uuid.New(), TenantID: r.tenant, Kind: telegram.Kind, Name: "bot", Status: connectors.StatusConnected}
	r.bot = api.Bot(tenantToken, "tenant_bot")
	r.platform = api.Bot(platformToken, "platform_bot")
	return r
}

func (r *rig) creds() connectors.Credentials { return connectors.Credentials{"token": tenantToken} }

func (r *rig) platformConn() connectors.Connection {
	return connectors.Connection{Kind: telegram.Kind, Name: "platform", Status: connectors.StatusConnected}
}

func (r *rig) platformCreds() connectors.Credentials {
	return connectors.Credentials{"token": platformToken}
}

// post sends an update through Verify and Handle as the engine's handler does.
func (r *rig) post(conn connectors.Connection, creds connectors.Credentials, secret string, update map[string]any) (int, []engine.InboundEvent) {
	r.t.Helper()
	body, err := json.Marshal(update)
	require.NoError(r.t, err)
	req := httptest.NewRequest(http.MethodPost, "/automations/hooks/x", strings.NewReader(string(body)))
	if secret != "" {
		req.Header.Set(telegram.SecretHeader, secret)
	}
	if err := r.c.Verify(req, body, conn, creds); err != nil {
		return http.StatusNotFound, nil
	}
	w := httptest.NewRecorder()
	evs, err := r.c.Handle(context.Background(), w, body, conn, creds)
	if errors.Is(err, engine.ErrWebhookRetry) {
		return http.StatusServiceUnavailable, nil
	}
	if err != nil {
		return http.StatusBadRequest, nil
	}
	return w.Code, evs
}

func (r *rig) secret(conn connectors.Connection, token string) string {
	key := catalog.PlatformConnectionID(telegram.Kind)
	if conn.ID != uuid.Nil {
		key = conn.ID.String()
	}
	return telegram.WebhookSecret(token, key)
}

var alice = telegramtest.User{ID: 501, Username: "alice", FirstName: "Alice"}
