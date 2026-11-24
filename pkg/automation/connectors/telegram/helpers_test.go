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
	mu          sync.Mutex
	status      connectors.ConnectionStatus
	reason      string
	inbound     json.RawMessage
	exists      map[uuid.UUID]bool                   // connections of the application that are alive
	owner       uuid.UUID                            // the connection the inbound state is of
	actions     map[string]*connectors.InboundAction // updates the connector handled itself
	failHandled int                                  // InboundHandled calls that fail
	failWrites  int                                  // inbound writes that fail
	token       string                               // the token the connection holds now ("" : the tenant bot's)
	version     int
}

func (s *state) ConnectionExists(_ context.Context, id uuid.UUID) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exists[id], nil
}

func (s *state) Get(context.Context, uuid.UUID, uuid.UUID) (connectors.Connection, connectors.Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token := s.token
	if token == "" {
		token = tenantToken
	}
	return connectors.Connection{Version: s.version}, connectors.Credentials{"token": token}, nil
}

func (s *state) SetStatusIf(_ context.Context, _, _ uuid.UUID, st connectors.ConnectionStatus, reason string, cond func(json.RawMessage, int) bool) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cond != nil && !cond(s.inbound, s.version) {
		return false, nil
	}
	s.status, s.reason = st, reason
	return true, nil
}

// ClaimInbound, InboundHandled, ClaimDelivery, InboundCompleted and ReleaseInbound are
// connectors.InboundActions: the updates the connector handles itself, with owner tokens like the store's.
func (s *state) ClaimInbound(_ context.Context, connection, event, key string) (connectors.InboundAction, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := connection + "/" + event + "/" + key
	if a, ok := s.actions[k]; ok {
		out := *a
		out.Owner = uuid.Nil
		return out, false, nil
	}
	if s.actions == nil {
		s.actions = map[string]*connectors.InboundAction{}
	}
	a := &connectors.InboundAction{State: connectors.InboundClaimed, Owner: uuid.New()}
	s.actions[k] = a
	return *a, true, nil
}

func (s *state) InboundHandled(_ context.Context, connection, event, key string, owner, tenant uuid.UUID, reply string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failHandled > 0 {
		s.failHandled--
		return errors.New("storage is down")
	}
	a := s.actions[connection+"/"+event+"/"+key]
	if a == nil || a.Owner != owner || a.State != connectors.InboundClaimed {
		return connectors.ErrInboundLost
	}
	*a = connectors.InboundAction{State: connectors.InboundHandled, Tenant: tenant, Reply: reply}
	return nil
}

func (s *state) ClaimDelivery(_ context.Context, connection, event, key string) (connectors.InboundAction, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.actions[connection+"/"+event+"/"+key]
	if a == nil {
		return connectors.InboundAction{}, false, connectors.ErrNotFound
	}
	if a.State != connectors.InboundHandled {
		out := *a
		out.Owner = uuid.Nil
		return out, false, nil
	}
	a.State, a.Owner = connectors.InboundDelivering, uuid.New()
	return *a, true, nil
}

func (s *state) InboundCompleted(_ context.Context, connection, event, key string, owner uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a := s.actions[connection+"/"+event+"/"+key]
	if a == nil || a.Owner != owner || a.State != connectors.InboundDelivering {
		return connectors.ErrInboundLost
	}
	a.State = connectors.InboundDone
	return nil
}

func (s *state) ReleaseInbound(_ context.Context, connection, event, key string, owner uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := connection + "/" + event + "/" + key
	a := s.actions[k]
	switch {
	case a == nil || a.Owner != owner:
		return connectors.ErrInboundLost
	case a.State == connectors.InboundDelivering:
		a.State, a.Owner = connectors.InboundHandled, uuid.New()
	case a.State == connectors.InboundClaimed:
		delete(s.actions, k)
	default:
		return connectors.ErrInboundLost
	}
	return nil
}

// abandon takes the claims of the running steps away, as the store does with ones that are old: their
// owners never finished. A claimed update is forgotten, a delivering one is handled again.
func (s *state) abandon() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, a := range s.actions {
		switch a.State {
		case connectors.InboundClaimed:
			delete(s.actions, k)
		case connectors.InboundDelivering:
			a.State, a.Owner = connectors.InboundHandled, uuid.New()
		case connectors.InboundHandled, connectors.InboundDone:
		}
	}
}

func (s *state) UpdateInbound(_ context.Context, _, _ uuid.UUID, fn func(json.RawMessage) (json.RawMessage, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failWrites > 0 {
		s.failWrites--
		return errors.New("storage is down")
	}
	next, err := fn(s.inbound)
	if err != nil {
		return err
	}
	s.inbound = next
	return nil
}

func (s *state) InboundState(_ context.Context, _, id uuid.UUID) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id != s.owner {
		return json.RawMessage(`{}`), nil
	}
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
	r.st.exists = map[uuid.UUID]bool{r.conn.ID: true}
	r.st.owner = r.conn.ID
	// The connection's webhook is set up as the integrations service does: the secret Provision generates
	// is kept in its inbound state.
	r.provision()
	return r
}

// provision sets the tenant connection's webhook and keeps the secret it generated in the inbound state.
func (r *rig) provision() {
	r.t.Helper()
	state, err := r.prov(context.Background(), r.conn, r.creds(), []string{telegram.WarnPollingUnknown + "@111111"})
	require.NoError(r.t, err)
	r.st.mu.Lock()
	defer r.st.mu.Unlock()
	merged, err := connectors.MergeInbound(r.st.inbound, state)
	require.NoError(r.t, err)
	r.st.inbound = merged
}

// prov provisions a connection with the inbound state it has now, as the save's transaction hands it over.
func (r *rig) prov(ctx context.Context, conn connectors.Connection, creds connectors.Credentials, consent []string) (map[string]json.RawMessage, error) {
	r.st.mu.Lock()
	var held json.RawMessage
	if conn.ID == r.st.owner {
		held = r.st.inbound
	}
	r.st.mu.Unlock()
	return r.c.Provision(ctx, conn, creds, consent, held)
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

// secret is what Telegram sends back for a connection: the secret set with its webhook.
func (r *rig) secret(conn connectors.Connection, token string) string {
	if conn.ID != uuid.Nil {
		return r.bot.Webhook().Secret
	}
	return telegram.WebhookSecret(token, catalog.PlatformConnectionID(telegram.Kind))
}

var alice = telegramtest.User{ID: 501, Username: "alice", FirstName: "Alice"}
