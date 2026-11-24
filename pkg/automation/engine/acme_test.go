package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// The acme connector: calls of every kind (send, fetch, ask, http, classify), an inbound message and an
// answer event, a webhook and a poller.

const (
	kindAcme  = catalog.ConnectorKind("acme")
	evMessage = "acme.message.v1"
	evAnswer  = "acme.answer.v1"
	evReceipt = "acme.receipt.v1" // a wait-only event that is not the ask's answer
	acmeToken = "tok-SECRET-42"
	acmeSig   = "sig-SECRET-7"
)

type acmeCall struct {
	Node    string
	Key     string
	Attempt int
	Token   string // the correlation token of an ask
}

type acme struct {
	mu       sync.Mutex
	calls    []acmeCall
	entered  chan string // a "crash" call is running
	provider atomic.Int64
	closed   atomic.Int64
	polls    atomic.Int64
	active   atomic.Int64 // calls of mode hold running now
	peak     atomic.Int64 // the most that ran at once
	pollQ    [][]engine.InboundEvent
	pollErr  error
	onAsk    func(token string) // runs inside the ask's Execute (answers that come before its wait opens)
	// answerKey is the answer event the ask names (default acme.answer.v1).
	answerKey atomic.Value
}

func (a *acme) answer() string {
	if k, ok := a.answerKey.Load().(string); ok && k != "" {
		return k
	}
	return evAnswer
}

func newAcme() *acme { return &acme{entered: make(chan string, 16)} }

func (a *acme) record(c acmeCall) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, c)
}

func (a *acme) callsOf(node string) []acmeCall {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []acmeCall
	for _, c := range a.calls {
		if c.Node == node {
			out = append(out, c)
		}
	}
	return out
}

func (a *acme) Kind() catalog.ConnectorKind { return kindAcme }
func (a *acme) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{
		Kind: kindAcme, Title: map[string]string{"en": "Acme"}, Auth: connectors.AuthAPIKey, Inbound: true,
		Credentials: json.RawMessage(`{"type":"object","properties":{"token":{"type":"string","x-secret":true},"secret":{"type":"string","x-secret":true}},"additionalProperties":false}`),
	}
}
func (a *acme) Test(context.Context, connectors.Credentials) (connectors.Identity, error) {
	return connectors.Identity{Display: "@acme_bot", Kind: connectors.IdentityHandle}, nil
}

func (a *acme) Nodes() []catalog.NodeType {
	return []catalog.NodeType{
		&acmeSend{a: a, typ: "acme.send"}, &acmeSend{a: a, typ: "acme.fetch", idempotent: true},
		&acmeAsk{a: a}, &acmeHTTP{a: a}, &acmeClassify{a: a},
	}
}

type acmeEvents struct{}

func (acmeEvents) Requires() []catalog.ConnectorKind { return nil }
func (acmeEvents) Events() []graph.EventSpec {
	return []graph.EventSpec{
		{Key: evMessage, Title: "Message", Subject: graph.SubjectConversation,
			Payload: expr.Fields{{Name: "text", Field: expr.StringField}, {Name: "from", Field: expr.StringField}}},
		{Key: evAnswer, Title: "Answer", Subject: graph.SubjectConversation, NoTrigger: true,
			Payload: expr.Fields{{Name: "choice", Field: expr.StringField}}},
		{Key: evReceipt, Title: "Receipt", Subject: graph.SubjectConversation, NoTrigger: true,
			Payload: expr.Fields{{Name: "choice", Field: expr.StringField}}},
	}
}

func (a *acme) Triggers() []catalog.TriggerSource { return []catalog.TriggerSource{acmeEvents{}} }

// Verify checks the signature header against the connection's secret.
func (a *acme) Verify(r *http.Request, _ []byte, _ connectors.Connection, creds connectors.Credentials) error {
	if r.Header.Get("X-Sig") != creds["secret"] {
		return errors.New("bad signature")
	}
	return nil
}

// acmeUpdate is one update of the acme webhook.
type acmeUpdate struct {
	ID          string `json:"id"`
	Event       string `json:"event"`
	Text        string `json:"text,omitempty"`
	From        string `json:"from,omitempty"`
	Choice      string `json:"choice,omitempty"`
	Correlation string `json:"correlation,omitempty"`
	Self        bool   `json:"self,omitempty"`
}

func (u acmeUpdate) event(connection string) engine.InboundEvent {
	payload := map[string]any{"text": u.Text, "from": u.From}
	if u.Event == evAnswer || u.Event == evReceipt {
		payload = map[string]any{"choice": u.Choice}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	from := u.From
	if from == "" {
		from = "anyone"
	}
	return engine.InboundEvent{Event: u.Event, DedupKey: u.ID, Correlation: u.Correlation, BySelf: u.Self, Payload: raw,
		Subject: engine.SubjectRef{Entity: graph.SubjectConversation, Key: connection + ":" + from}}
}

func (a *acme) Handle(_ context.Context, w http.ResponseWriter, body []byte, c connectors.Connection, _ connectors.Credentials) ([]engine.InboundEvent, error) {
	var u acmeUpdate
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}
	if u.Event == "" {
		u.Event = evMessage
	}
	_, _ = w.Write([]byte(`{"ok":true}`))
	return []engine.InboundEvent{u.event(c.ID.String())}, nil
}

func (a *acme) AnswerClosed(context.Context, engine.InboundEvent, connectors.Connection, connectors.Credentials) {
	a.closed.Add(1)
}

func (a *acme) Interval(connectors.Connection) time.Duration { return 30 * time.Second }

func (a *acme) Poll(_ context.Context, c connectors.Connection, _ connectors.Credentials, cursor json.RawMessage) ([]engine.InboundEvent, json.RawMessage, error) {
	a.polls.Add(1)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.pollErr != nil {
		return nil, nil, a.pollErr
	}
	n := 0
	if len(cursor) > 0 {
		_ = json.Unmarshal(cursor, &n)
	}
	if n >= len(a.pollQ) {
		return nil, cursor, nil
	}
	return a.pollQ[n], json.RawMessage(fmt.Sprint(n + 1)), nil
}

const acmeParams = `{"type":"object","properties":{"mode":{"type":"string"},"text":{"type":"string"},"timeout":{"type":"string"}},"additionalProperties":false}`

// acmeSend sends a message (acme.send: not idempotent; acme.fetch: idempotent). Its mode param picks
// what the provider does: ok, crash (the call runs until the process stops), crash_once, 429, reject.
type acmeSend struct {
	a          *acme
	typ        string
	idempotent bool
}

func (n *acmeSend) Spec() graph.NodeSpec {
	return graph.NodeSpec{Type: n.typ, Title: n.typ, Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture,
		Params: graph.MustSchemaParams(acmeParams), Templates: []string{"text"},
		Outcomes: []string{"sent", "failed"}, FailedOutcome: "failed", DoneOutcome: "sent",
		Output: expr.Fields{{Name: "message_id", Field: expr.OptionalOf(expr.StringField)}}}
}
func (n *acmeSend) Requires() []catalog.ConnectorKind { return nil }
func (n *acmeSend) Idempotent() bool                  { return n.idempotent }

func (n *acmeSend) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	mode, _ := in.Params["mode"].(string)
	prior := len(n.a.callsOf(n.typ))
	n.a.record(acmeCall{Node: n.typ, Key: in.IdempotencyKey, Attempt: in.Attempt})
	switch {
	case mode == "crash" || (mode == "crash_once" && prior == 0):
		n.a.entered <- n.typ
		<-ctx.Done()
		return catalog.CallResult{}, ctx.Err()
	case mode == "hold":
		now := n.a.active.Add(1)
		for {
			peak := n.a.peak.Load()
			if now <= peak || n.a.peak.CompareAndSwap(peak, now) {
				break
			}
		}
		time.Sleep(300 * time.Millisecond)
		n.a.active.Add(-1)
	case mode == "hold_long":
		time.Sleep(2500 * time.Millisecond)
	case mode == "unauth":
		return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallUnauthorized, Message: "bad token", Retryable: true}
	case mode == "unavail":
		return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallUnavailable, Message: "down", Retryable: true}
	case mode == "429":
		return catalog.CallResult{RetryAfter: 10 * time.Minute}, nil
	case mode == "reject":
		return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallRejected, Message: "the provider refused token " + in.Credentials["token"]}
	}
	return catalog.CallResult{Outcome: "sent", Output: map[string]any{"message_id": "m-" + in.IdempotencyKey}}, nil
}

// acmeAsk asks a question; its answer is acme.answer.v1 with the correlation token.
type acmeAsk struct{ a *acme }

func (n *acmeAsk) Spec() graph.NodeSpec {
	return graph.NodeSpec{Type: "acme.ask", Title: "Ask", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture,
		Params: graph.MustSchemaParams(acmeParams), Templates: []string{"text"},
		Outcomes: []string{"yes", "no", "timeout", "failed"}, FailedOutcome: "failed",
		Output: expr.Fields{{Name: "choice", Field: expr.OptionalOf(expr.StringField)}, {Name: "message_id", Field: expr.OptionalOf(expr.StringField)}}}
}
func (n *acmeAsk) Requires() []catalog.ConnectorKind { return nil }
func (n *acmeAsk) Idempotent() bool                  { return false }
func (n *acmeAsk) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	c, _ := engine.CorrelationFrom(ctx)
	prior := len(n.a.callsOf("acme.ask"))
	n.a.record(acmeCall{Node: "acme.ask", Key: in.IdempotencyKey, Attempt: in.Attempt, Token: c.Token})
	if n.a.onAsk != nil {
		n.a.onAsk(c.Token)
	}
	if mode, _ := in.Params["mode"].(string); mode == "429_once" && prior == 0 {
		return catalog.CallResult{RetryAfter: 50 * time.Millisecond}, nil
	}
	return catalog.CallResult{Outcome: "yes", Output: map[string]any{"message_id": "q-1"}}, nil
}
func (n *acmeAsk) Await(catalog.CallInput, catalog.CallResult) (engine.Await, error) {
	return engine.Await{Event: n.a.answer(), Timeout: time.Hour, Single: true}, nil
}
func (n *acmeAsk) AnswerEvent(catalog.CallInput) (string, bool) { return n.a.answer(), true }

func (n *acmeAsk) Answer(payload json.RawMessage) (string, json.RawMessage, error) {
	var p struct {
		Choice string `json:"choice"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return "", nil, err
	}
	out, err := json.Marshal(map[string]any{"choice": p.Choice})
	return p.Choice, out, err
}

// acmeHTTP is a GET that echoes the authorization header it sent.
type acmeHTTP struct{ a *acme }

func (n *acmeHTTP) Spec() graph.NodeSpec {
	return graph.NodeSpec{Type: "acme.http", Title: "HTTP", Behavior: graph.BehaviorCall, DryRun: graph.DryRunExecute,
		Params: graph.MustSchemaParams(acmeParams), Outcomes: []string{"ok", "failed"}, FailedOutcome: "failed",
		Output: expr.Fields{{Name: "headers", Field: expr.OptionalOf(expr.MapOf(expr.StringField))}}}
}
func (n *acmeHTTP) Requires() []catalog.ConnectorKind { return nil }
func (n *acmeHTTP) Idempotent() bool                  { return true }
func (n *acmeHTTP) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	n.a.record(acmeCall{Node: "acme.http", Key: in.IdempotencyKey, Attempt: in.Attempt})
	engine.Log(ctx, "GET /me with Bearer %s", in.Credentials["token"])
	return catalog.CallResult{Outcome: "ok", Output: map[string]any{"headers": map[string]any{"Authorization": "Bearer " + in.Credentials["token"]}}}, nil
}

// acmeClassify is a model's decision: sales or support. Its provider calls are counted.
type acmeClassify struct{ a *acme }

func (n *acmeClassify) Spec() graph.NodeSpec {
	return graph.NodeSpec{Type: "acme.classify", Title: "Classify", Behavior: graph.BehaviorCall, DryRun: graph.DryRunExecute,
		Params: graph.MustSchemaParams(acmeParams), Templates: []string{"text"}, Outcomes: []string{"sales", "support", "failed"}, FailedOutcome: "failed",
		Output: expr.Fields{{Name: "label", Field: expr.OptionalOf(expr.StringField)}}}
}
func (n *acmeClassify) Requires() []catalog.ConnectorKind { return nil }
func (n *acmeClassify) Idempotent() bool                  { return true }
func (n *acmeClassify) Execute(_ context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	n.a.provider.Add(1)
	text, _ := in.Params["text"].(string)
	label := "support"
	if strings.Contains(text, "buy") {
		label = "sales"
	}
	return catalog.CallResult{Outcome: label, Output: map[string]any{"label": label}}, nil
}
func (n *acmeClassify) DecisionKey(in catalog.CallInput) (string, bool) {
	text, _ := in.Params["text"].(string)
	return engine.DecisionKey("classify", text, "acme-1", nil), true
}

// acmeEnv is an env whose catalog has the acme connector with one connection of the tenant.
type acmeEnv struct {
	*env
	acme  *acme
	store connectors.Store
	dir   connectors.Directory
	conn  connectors.Connection
}

func newAcmeEnv(t *testing.T) *acmeEnv {
	t.Helper()
	var store connectors.Store
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	e := newEnvWith(t, func(db *pgxpool.Pool) catalog.Availability {
		store = connectors.NewStore(db, keys, nil, nil, nil)
		return store
	})
	ae := &acmeEnv{env: e, acme: newAcme(), store: store}
	ae.dir = connectors.NewDirectory(e.reg)
	require.NoError(t, ae.dir.Register(ae.acme))
	require.NoError(t, e.reg.RegisterNode(engine.NewHumanNode()))
	ae.conn, err = store.Save(context.Background(), connectors.Connection{TenantID: e.tenant, Kind: kindAcme, Name: "main", DisplayName: "Main"},
		connectors.Credentials{"token": acmeToken, "secret": acmeSig})
	require.NoError(t, err)
	return ae
}

// proc is a process whose calls open the acme connection.
func (ae *acmeEnv) proc(tune ...func(*engine.Config)) *proc {
	return ae.env.proc(append([]func(*engine.Config){func(c *engine.Config) {
		c.Credentials = ae.store
		c.Tasks = engine.NewHumanBridge(0)
		c.CallLimits.BackoffBase = 10 * time.Millisecond
	}}, tune...)...)
}

// hook posts one update to the webhook of the connection (sig: the signature header).
func (ae *acmeEnv) hook(h http.Handler, path, sig string, u acmeUpdate) *httptest.ResponseRecorder {
	ae.t.Helper()
	body, err := json.Marshal(u)
	require.NoError(ae.t, err)
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(body)))
	req.Header.Set("X-Sig", sig)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func (ae *acmeEnv) handler(p *proc, tenantOf func(*http.Request) (uuid.UUID, bool)) http.Handler {
	return engine.InboundHandler(engine.InboundConfig{Connections: ae.store, Registry: ae.reg, Inbound: p.rt.Inbound(), DB: ae.db,
		Connectors: ae.dir, TenantOf: tenantOf})
}

func (ae *acmeEnv) hookPath() string { return "/automations/hooks/" + ae.conn.ID.String() }

// drive runs the process in the background until stop is called: the dispatcher, the workflow engine
// (with a context the stop cancels, as a process that dies) and maintenance.
func (p *proc) drive() func() {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		for ctx.Err() == nil {
			_, _ = p.disp.RunOnce(ctx)
			_, _ = p.wf.RunOnce(ctx)
			_ = p.rt.Maintain(ctx)
			time.Sleep(20 * time.Millisecond)
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

// fakeAudience: groups and roles of users.
type fakeAudience struct{ members map[string][]int64 }

func (f fakeAudience) IsMember(_ context.Context, _ uuid.UUID, user int64, a workflow.TaskAssignee) (bool, error) {
	if a.Kind == workflow.TaskAssignUser {
		return a.UserID != nil && *a.UserID == user, nil
	}
	for _, u := range f.members[a.Kind+":"+a.Ref] {
		if u == user {
			return true, nil
		}
	}
	return false, nil
}

func (f fakeAudience) Users(_ context.Context, _ uuid.UUID, a workflow.TaskAssignee, limit int) ([]int64, error) {
	if a.Kind == workflow.TaskAssignUser {
		return []int64{*a.UserID}, nil
	}
	out := f.members[a.Kind+":"+a.Ref]
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func graphJSON(name, trigger string, nodes string, edges string) string {
	return fmt.Sprintf(`{"name": %q, "triggers": [%s], "nodes": [%s], "edges": [%s]}`, name, trigger, nodes, edges)
}
