package access_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

const secretToken = "tok-9f3a-SECRET"

// fakeKind is a connector with code access, per-connection base URL and operations.
type fakeKind struct {
	connectors.Connector
	access  connectors.CodeAccess
	baseErr error
	calls   []connectors.CodeCall
}

func (f *fakeKind) Descriptor() connectors.Descriptor {
	return connectors.Descriptor{Kind: "acme", Auth: connectors.AuthAPIKey,
		Credentials: json.RawMessage(`{"properties":{"token":{"x-secret":true},"url":{}}}`)}
}
func (f *fakeKind) CodeAccess() connectors.CodeAccess { return f.access }
func (f *fakeKind) Authorize(_ context.Context, creds connectors.Credentials, h http.Header) error {
	h.Set("Authorization", "Bearer "+creds["token"])
	return nil
}
func (f *fakeKind) CodeBaseURL(creds connectors.Credentials) (string, error) {
	if f.baseErr != nil {
		return "", f.baseErr
	}
	return creds["url"], nil
}
func (f *fakeKind) CallCodeOperation(_ context.Context, call connectors.CodeCall) (any, error) {
	f.calls = append(f.calls, call)
	switch call.Operation {
	case "send":
		return map[string]any{"ok": true, "echo": call.Input.Credentials["token"], "conn": call.Input.Connection.Name}, nil
	case "refuse":
		return nil, &connectors.CodeOperationError{Code: "rate_limited", Message: "slow down " + secretToken, Retryable: true}
	case "boom":
		return nil, errors.New("provider failed with " + secretToken)
	}
	return nil, nil
}

type fakeDir struct {
	connectors.Directory
	k connectors.Connector
}

func (d fakeDir) Get(catalog.ConnectorKind) (connectors.Connector, bool) { return d.k, true }

type fakeStore struct {
	connectors.ConnectionStore
	conn  connectors.Connection
	creds connectors.Credentials
}

func (s fakeStore) List(context.Context, uuid.UUID, catalog.ConnectorKind) ([]connectors.Connection, error) {
	return []connectors.Connection{s.conn}, nil
}
func (s fakeStore) Get(context.Context, uuid.UUID, uuid.UUID) (connectors.Connection, connectors.Credentials, error) {
	return s.conn, s.creds, nil
}

func openWith(t *testing.T, kind *fakeKind, status connectors.ConnectionStatus) (access.Connection, error) {
	t.Helper()
	store := fakeStore{
		conn:  connectors.Connection{ID: uuid.New(), Kind: "acme", Name: "crm", Status: status},
		creds: connectors.Credentials{"token": secretToken, "url": "https://tenant.example.com/api"},
	}
	return access.StoreConnections(store, fakeDir{k: kind}, nil).Open(context.Background(), uuid.New(), "crm")
}

func TestOpenResolvesBaseURLAndBindsOperations(t *testing.T) {
	kind := &fakeKind{access: connectors.CodeAccess{Fetch: true, Operations: []connectors.CodeOperation{{Name: "send"}}}}
	c, err := openWith(t, kind, connectors.StatusConnected)
	if err != nil {
		t.Fatal(err)
	}
	if c.Access.BaseURL != "https://tenant.example.com/api" || !c.HasOperator() {
		t.Fatalf("connection: %+v", c.Access)
	}
	// A base URL that cannot be resolved refuses the connection without echoing a credential.
	kind.baseErr = errors.New("bad url " + secretToken)
	_, err = openWith(t, kind, connectors.StatusConnected)
	if !errors.Is(err, connectors.ErrCodeConnectionBroken) || strings.Contains(err.Error(), secretToken) {
		t.Fatalf("unresolvable base URL: %v", err)
	}
}

func TestDisabledAndBrokenConnectionsAreTypedRefusals(t *testing.T) {
	kind := &fakeKind{access: connectors.CodeAccess{BaseURL: "https://api.example.com"}}
	for status, want := range map[connectors.ConnectionStatus]struct {
		err  error
		code string
	}{
		connectors.StatusDisabled:    {connectors.ErrCodeConnectionDisabled, access.CodeConnectionDisabled},
		connectors.StatusNeedsReauth: {connectors.ErrCodeConnectionBroken, access.CodeConnectionBroken},
		connectors.StatusError:       {connectors.ErrCodeConnectionBroken, access.CodeConnectionBroken},
	} {
		_, err := openWith(t, kind, status)
		if !errors.Is(err, want.err) || errors.Is(err, access.ErrConnectionMissing) {
			t.Fatalf("%s: %v", status, err)
		}
		store := fakeStore{conn: connectors.Connection{ID: uuid.New(), Kind: "acme", Name: "crm", Status: status}}
		_, err = access.OpenAll(context.Background(), access.StoreConnections(store, fakeDir{k: kind}, nil), uuid.New(), []string{"crm"})
		var ce *catalog.CallError
		if !errors.As(err, &ce) || ce.Code != want.code {
			t.Fatalf("%s: OpenAll %v", status, err)
		}
	}
	if access.CodeConnectionDisabled != "connection_disabled" || access.CodeConnectionBroken != "connection_broken" {
		t.Fatal("codes changed")
	}
}

func TestFetchInjectsTheAuthorizationAndTheCodeCannotOverrideIt(t *testing.T) {
	kind := &fakeKind{access: connectors.CodeAccess{Fetch: true}}
	c, err := openWith(t, kind, connectors.StatusConnected)
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	g := &access.Gateway{Fetcher: rec, Connections: map[string]access.Connection{"crm": c}}
	_, err = g.CallConnection(context.Background(), code.ConnectionRequest{
		Connection: "crm", Method: "fetch", Fetch: true, HTTPMethod: "POST", Path: "/items", Body: []byte("x"),
		Header: http.Header{"authorization": {"Bearer attacker"}, "X-Custom": {"1"}, "Connection": {"close"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := rec.got
	if got.Method != http.MethodPost || got.URL != "https://tenant.example.com/api/items" || !got.SameOrigin {
		t.Fatalf("request %+v", got)
	}
	if v := got.Header.Values("Authorization"); len(v) != 1 || v[0] != "Bearer "+secretToken {
		t.Fatalf("authorization: %v", v)
	}
	if got.Header.Get("X-Custom") != "1" || got.Header.Get("Connection") != "" {
		t.Fatalf("headers: %v", got.Header)
	}
	// Off-origin URLs are refused, and a connection without Fetch has none.
	if _, err := g.CallConnection(context.Background(), code.ConnectionRequest{Connection: "crm", Fetch: true, Path: "https://evil.example.org/"}); err == nil {
		t.Fatal("off-origin fetch")
	}
	noFetch := access.NewConnection("n", "acme", connectors.CodeAccess{BaseURL: "https://api.example.com"}, nil, nil)
	g.Connections["n"] = noFetch
	if _, err := g.CallConnection(context.Background(), code.ConnectionRequest{Connection: "n", Fetch: true, Path: "/"}); err == nil {
		t.Fatal("fetch without Fetch")
	}
}

func TestOperationsCheckPermissionAndMaskResultAndError(t *testing.T) {
	kind := &fakeKind{access: connectors.CodeAccess{Operations: []connectors.CodeOperation{
		{Name: "send", Permission: "Mail.Send"}, {Name: "refuse"}, {Name: "boom"}, {Name: "open"},
	}}}
	c, err := openWith(t, kind, connectors.StatusConnected)
	if err != nil {
		t.Fatal(err)
	}
	g := &access.Gateway{Connections: map[string]access.Connection{"crm": c}, Input: catalog.CallInput{RunAs: spi.Subject{}}}
	call := func(op string) (json.RawMessage, error) {
		return g.CallOperation(context.Background(), code.OperationRequest{Connection: "crm", Operation: op, Args: []byte(`{"a":1}`)})
	}

	_, err = call("send")
	var refused *code.ConnectionRefusedError
	if !errors.As(err, &refused) || refused.Code != access.CodePermissionDenied || len(kind.calls) != 0 {
		t.Fatalf("a run-as without the permission: %v, %d calls", err, len(kind.calls))
	}

	g.Input.RunAs = spi.Subject{Permissions: []string{"Mail.Send"}}
	res, err := call("send")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(res), secretToken) || !strings.Contains(string(res), `"conn":"crm"`) {
		t.Fatalf("result: %s", res)
	}
	if got := kind.calls[0]; string(got.Args) != `{"a":1}` || got.Input.Connection == nil || got.Input.Connection.Name != "crm" || got.Input.Credentials["token"] != secretToken {
		t.Fatalf("call given to the operator: %+v", got)
	}

	_, err = call("refuse")
	if !errors.As(err, &refused) || refused.Code != "rate_limited" || !refused.Retryable || strings.Contains(refused.Message, secretToken) {
		t.Fatalf("coded refusal: %v", err)
	}
	_, err = call("boom")
	if err == nil || strings.Contains(err.Error(), secretToken) || !strings.Contains(err.Error(), "provider failed") {
		t.Fatalf("error masking: %v", err)
	}
	if _, err = call("nope"); err == nil {
		t.Fatal("an undeclared operation was called")
	}
	if res, err = call("open"); err != nil || string(res) != "null" {
		t.Fatalf("nil result: %s %v", res, err)
	}
}
