package code

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

// fakeConnections answers env.connections calls: a path outside the origin is refused, the rest echo
// what they got, and one response carries the token, which must be masked.
type fakeConnections struct {
	mu    sync.Mutex
	calls []ConnectionRequest
}

const connToken = "tok-3c9d1e7f-SECRET"

func (f *fakeConnections) CallConnection(_ context.Context, req ConnectionRequest) (*FetchResponse, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	f.mu.Unlock()
	if strings.Contains(req.Path, "://") && !strings.HasPrefix(req.Path, "https://api.example.com/") {
		return nil, &ConnectionRefusedError{Code: ReasonConnectionOriginMismatch, Message: "the path leaves the connector's origin"}
	}
	body, err := json.Marshal(map[string]string{"method": req.Method, "path": req.Path, "body": string(req.Body), "echo": "Bearer " + connToken})
	if err != nil {
		return nil, err
	}
	return &FetchResponse{Status: 200, StatusText: "OK", Header: http.Header{"Content-Type": {"application/json"}}, Body: body, URL: "https://api.example.com" + req.Path}, nil
}

func TestConnectionsCallThroughTheCaller(t *testing.T) {
	fc := &fakeConnections{}
	res := runCode(t, `export default async function run(ctx, env) {
		const r = await env.connections.crm.get("/v1/deals?limit=2");
		const data = await r.json();
		env.log("got", data.echo);
		const p = await env.connections.crm.post("/v1/deals", {name: "x"});
		return { outcome: "done", output: { first: data, second: await p.json(), keys: Object.keys(env.connections) } };
	}`, RunInput{Connections: map[string][]string{"crm": {"get", "post"}}, ConnectionCaller: fc, Masked: []string{connToken}})
	if res.Err != nil {
		t.Fatalf("run failed: %v", res.Err)
	}
	if strings.Contains(string(res.Output), connToken) || strings.Contains(strings.Join(res.Logs, "\n"), connToken) {
		t.Fatalf("the token leaked: %s %v", res.Output, res.Logs)
	}
	var out struct {
		First  map[string]any `json:"first"`
		Second map[string]any `json:"second"`
		Keys   []string       `json:"keys"`
	}
	if err := json.Unmarshal(res.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out.First["path"] != "/v1/deals?limit=2" || out.First["method"] != "get" || out.First["echo"] != "Bearer ***" {
		t.Fatalf("first call: %v", out.First)
	}
	if out.Second["body"] != `{"name":"x"}` || out.Second["method"] != "post" {
		t.Fatalf("second call: %v", out.Second)
	}
	if len(out.Keys) != 1 || out.Keys[0] != "crm" {
		t.Fatalf("connections: %v", out.Keys)
	}
	if len(fc.calls) != 2 {
		t.Fatalf("calls: %v", fc.calls)
	}
}

func TestConnectionOriginMismatchFailsTheRun(t *testing.T) {
	res := runCode(t, `export default async function run(ctx, env) {
		await env.connections.crm.get("https://evil.example.org/steal");
		return { outcome: "done" };
	}`, RunInput{Connections: map[string][]string{"crm": {"get"}}, ConnectionCaller: &fakeConnections{}})
	wantReason(t, res, ReasonConnectionOriginMismatch)
}

func TestUndeclaredConnectionIsMissing(t *testing.T) {
	res := runCode(t, `export default async function run(ctx, env) {
		if (env.connections.crm !== undefined) throw new Error("undeclared connection is visible");
		return { outcome: "done" };
	}`, RunInput{ConnectionCaller: &fakeConnections{}})
	if res.Err != nil {
		t.Fatalf("run failed: %v", res.Err)
	}
	// A call without a caller is refused as missing.
	res = runCode(t, `export default async function run(ctx, env) {
		await env.connections.crm.get("/x");
		return { outcome: "done" };
	}`, RunInput{Connections: map[string][]string{"crm": {"get"}}})
	wantReason(t, res, ReasonConnectionMissing)
}

func TestInvalidConnectionNamesAreRefused(t *testing.T) {
	_, err := testEngine(t).Run(context.Background(), mustCompile(t, LangJS, `export default async () => ({outcome: "done"})`),
		RunInput{Outcomes: []string{"done"}, Connections: map[string][]string{"Bad-Name": {"get"}}})
	if err == nil {
		t.Fatal("an invalid connection name was accepted")
	}
	_, err = testEngine(t).Run(context.Background(), mustCompile(t, LangJS, `export default async () => ({outcome: "done"})`),
		RunInput{Outcomes: []string{"done"}, Connections: map[string][]string{"crm": {"then"}}})
	if err == nil {
		t.Fatal("a then method was accepted")
	}
}

func TestDeclarationsListConnections(t *testing.T) {
	d := DeclarationsWith(expr.Scope{}, []string{"done"}, nil, map[string][]string{"crm": {"list", "get"}})
	for _, want := range []string{"interface CodeConnections", "readonly crm: {", "get(path: string, body?: unknown): Promise<CodeFetchResponse>;", "readonly connections: CodeConnections;"} {
		if !strings.Contains(d, want) {
			t.Fatalf("declarations lack %q:\n%s", want, d)
		}
	}
}
