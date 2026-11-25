// Package aitest is a fake OpenAI-compatible provider for the tests of the AI connectors: it records the
// requests it gets and answers from a script.
package aitest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// OriginalHost is the header the fake's transport puts the host the connector asked for in, so a test can
// tell api.openai.com from openrouter.ai.
const OriginalHost = "X-Original-Host"

// Request is one request the provider got.
type Request struct {
	Method string
	Path   string
	Header http.Header
	Body   []byte
}

// Host is the host the connector asked for (set when the request came through Transport).
func (r Request) Host() string { return r.Header.Get(OriginalHost) }

// JSON decodes the body of the request.
func (r Request) JSON(tb testing.TB) map[string]any {
	tb.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		tb.Fatalf("the request body is not JSON: %v", err)
	}
	return m
}

// Reply is what the provider answers.
type Reply struct {
	Status int
	Header map[string]string
	Body   string
}

// Server is the fake provider. Each request takes the next reply of the script; the last one repeats.
type Server struct {
	*httptest.Server
	mu       sync.Mutex
	requests []Request
	script   []Reply
	// Handler, when set, answers instead of the script.
	Handler func(Request) Reply
}

// New starts a provider over TLS (the connectors take https only); close it with the test.
func New(tb testing.TB, script ...Reply) *Server {
	tb.Helper()
	s := &Server{script: script}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		req := Request{Method: r.Method, Path: r.URL.Path, Header: r.Header.Clone(), Body: body}
		s.mu.Lock()
		s.requests = append(s.requests, req)
		n := len(s.requests)
		handler := s.Handler
		var scripted Reply
		switch {
		case len(s.script) == 0:
			scripted = Reply{Status: http.StatusInternalServerError, Body: `{"error":{"message":"no reply scripted"}}`}
		case n <= len(s.script):
			scripted = s.script[n-1]
		default:
			scripted = s.script[len(s.script)-1]
		}
		s.mu.Unlock()
		// The handler runs without the lock: it may ask the server for its requests.
		reply := scripted
		if handler != nil {
			reply = handler(req)
		}
		for k, v := range reply.Header {
			w.Header().Set(k, v)
		}
		if reply.Status == 0 {
			reply.Status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(reply.Status)
		_, _ = w.Write([]byte(reply.Body))
	}))
	tb.Cleanup(s.Close)
	return s
}

// Requests are the requests the provider got, in order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// Transport carries a connector's requests to the provider, whatever host they name: the host is
// replaced by the fake's, whose certificate the transport trusts.
func (s *Server) Transport() http.RoundTripper {
	u, _ := url.Parse(s.URL)
	base := s.Client().Transport
	return roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.Header.Set(OriginalHost, r.URL.Host)
		r.URL.Scheme, r.URL.Host = u.Scheme, u.Host
		return base.RoundTrip(r)
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Chat is the body of a chat completion that answers content.
func Chat(content string, in, out int) string {
	return mustMarshal(map[string]any{
		"model":   "test-model-2025-01-01",
		"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": content}}},
		"usage":   map[string]any{"prompt_tokens": in, "completion_tokens": out},
	})
}

// APIError is the body of an error answer.
func APIError(message, typ, code string) string {
	return mustMarshal(map[string]any{"error": map[string]any{"message": message, "type": typ, "code": code}})
}

// mustMarshal is the JSON of a value built from maps, slices and scalars, which cannot fail.
func mustMarshal(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic("aitest: " + err.Error())
	}
	return string(b)
}

// Usage records the usage of a call.
type Usage struct {
	mu      sync.Mutex
	Entries []catalog.UsageEntry
	// Err, when set, is what Record returns.
	Err error
}

// Record implements catalog.UsageRecorder.
func (u *Usage) Record(_ context.Context, e catalog.UsageEntry) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.Err != nil {
		return u.Err
	}
	u.Entries = append(u.Entries, e)
	return nil
}

// Total is the quantity recorded in a unit.
func (u *Usage) Total(unit string) int64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	var n int64
	for _, e := range u.Entries {
		if e.Unit == unit {
			n += e.Quantity
		}
	}
	return n
}

// Count is the number of entries recorded in a unit.
func (u *Usage) Count(unit string) int {
	u.mu.Lock()
	defer u.mu.Unlock()
	n := 0
	for _, e := range u.Entries {
		if e.Unit == unit {
			n++
		}
	}
	return n
}

// Input is a call input of a tenant's own connection of a kind to the provider; params are the params
// the engine passes (parsed, templates rendered).
func Input(kind catalog.ConnectorKind, s *Server, creds map[string]string, params map[string]any, u catalog.UsageRecorder) catalog.CallInput {
	c := catalog.Credentials{"api_key": "sk-test-key-123456", "base_url": s.URL}
	for k, v := range creds {
		c[k] = v
	}
	return catalog.CallInput{
		TenantID: uuid.New(), RunID: uuid.New(), NodeID: "n1", Attempt: 1, IdempotencyKey: fmt.Sprintf("run:n1:%s", uuid.NewString()),
		Params: params, Connection: &catalog.ConnectionInfo{ID: uuid.NewString(), Kind: kind, Name: "main"}, Credentials: c, Usage: u,
	}
}

// TestEvent is the trigger event of the graphs of Validate: {title, text, labels, upload_ids, meta}.
const TestEvent = "test.event"

type availability map[catalog.ConnectorKind][]catalog.ConnectionRef

func (a availability) Connected(context.Context, uuid.UUID) (map[catalog.ConnectorKind][]catalog.ConnectionRef, error) {
	return a, nil
}

type events struct{}

func (events) Requires() []catalog.ConnectorKind { return nil }
func (events) Events() []graph.EventSpec {
	return []graph.EventSpec{{
		Key: TestEvent, Title: "Test event", Subject: graph.SubjectNone,
		Payload: expr.Fields{
			{Name: "title", Field: expr.StringField},
			{Name: "text", Field: expr.StringField},
			{Name: "count", Field: expr.IntField},
			{Name: "urgent", Field: expr.BoolField},
			{Name: "labels", Field: expr.ListOf(expr.StringField)},
			{Name: "upload_ids", Field: expr.ListOf(expr.StringField)},
			{Name: "meta", Field: expr.MapOf(expr.StringField)},
		},
	}}
}

type builtin struct{ spec graph.NodeSpec }

func (b builtin) Spec() graph.NodeSpec              { return b.spec }
func (b builtin) Requires() []catalog.ConnectorKind { return nil }

// View is the catalog of a tenant that has one connection of each connector's kind.
func View(tb testing.TB, conns ...connectors.Connector) catalog.View {
	tb.Helper()
	avail := availability{}
	for _, c := range conns {
		avail[c.Kind()] = []catalog.ConnectionRef{{ID: uuid.NewString(), Name: "main"}}
	}
	reg := catalog.NewRegistry(avail)
	if err := reg.RegisterTriggers(events{}); err != nil {
		tb.Fatalf("register the test event: %v", err)
	}
	for _, spec := range graph.BuiltinNodes(nil, nil) {
		if err := reg.RegisterNode(builtin{spec}); err != nil {
			tb.Fatalf("register %s: %v", spec.Type, err)
		}
	}
	for _, c := range conns {
		if err := reg.RegisterConnector(c); err != nil {
			tb.Fatalf("register %s: %v", c.Kind(), err)
		}
	}
	v, err := reg.ForTenant(context.Background(), uuid.New())
	if err != nil {
		tb.Fatalf("the catalog: %v", err)
	}
	return v
}

// Validate checks a graph document against the view as the engine does on save.
func Validate(tb testing.TB, v catalog.View, doc string) []graph.GraphError {
	tb.Helper()
	g, errs := graph.Decode([]byte(doc))
	if len(errs) > 0 {
		tb.Fatalf("the graph does not decode: %v", errs)
	}
	return graph.Validate(v.Catalog, g, graph.Options{Rules: catalog.ConnectionRule(v)})
}

// Graph is a graph of the test event (trigger t) and one node n1 with the params, every outcome of it
// going nowhere.
func Graph(nodeType, params string, outcomes []string) string {
	edges := `{"from":{"node":"t","outcome":"fired"},"to":"n1"}`
	for _, o := range outcomes {
		edges += `,{"from":{"node":"n1","outcome":"` + o + `"},"to":"ignore"}`
	}
	return Doc(`{"id":"n1","type":"`+nodeType+`","params":`+params+`}`, edges)
}

// Doc is a graph of the test event (trigger t) with the nodes and edges given as comma-separated JSON.
func Doc(nodes, edges string) string {
	return `{"name":"g","triggers":[{"id":"t","event":"` + TestEvent + `"}],"nodes":[` + nodes + `],"edges":[` + edges + `]}`
}

// Interpreter is the interpreter of a valid graph, as the engine runs it.
func Interpreter(tb testing.TB, v catalog.View, doc string) *graph.Interpreter {
	tb.Helper()
	g, errs := graph.Decode([]byte(doc))
	if len(errs) > 0 {
		tb.Fatalf("the graph does not decode: %v", errs)
	}
	in, err := graph.NewInterpreter(v.Catalog, g, graph.Options{})
	if err != nil {
		tb.Fatalf("the interpreter: %v", err)
	}
	return in
}
