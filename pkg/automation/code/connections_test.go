package code

import (
	"context"
	"encoding/json"
	"errors"
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

// fakeOps is a caller that also serves fetch and operations.
type fakeOps struct {
	fakeConnections
	opCalls []OperationRequest
}

func (f *fakeOps) CallOperation(_ context.Context, req OperationRequest) (json.RawMessage, error) {
	f.mu.Lock()
	f.opCalls = append(f.opCalls, req)
	f.mu.Unlock()
	switch req.Operation {
	case "echo":
		return json.RawMessage(`{"args":` + string(req.Args) + `,"token":"` + connToken + `"}`), nil
	case "refuse":
		return nil, &ConnectionRefusedError{Code: "invalid_args", Message: "bad chat " + connToken, Retryable: true}
	case "boom":
		return nil, errors.New("provider said " + connToken)
	}
	return nil, nil
}

func TestConnectionBaseURLFetchAndOperations(t *testing.T) {
	fo := &fakeOps{}
	res := runCode(t, `export default async function run(ctx, env) {
		const c = env.connections.api;
		const r = await c.fetch("/v1/x", { method: "POST", headers: {"X-A": "1"}, body: "hi" });
		const j = await r.json();
		const g = await c.fetch("/y");
		const e = await c.echo({ n: 1 });
		const none = await c.nothing();
		return { outcome: "done", output: { base: c.baseUrl, j, g: (await g.json()).method, e, none } };
	}`, RunInput{
		ConnectionSpecs:  map[string]ConnectionSpec{"api": {BaseURL: "https://api.example.com/v1", Fetch: true, Operations: []OperationSpec{{Name: "echo"}, {Name: "nothing"}}}},
		ConnectionCaller: fo, Masked: []string{connToken},
	})
	if res.Err != nil {
		t.Fatalf("run failed: %v", res.Err)
	}
	if strings.Contains(string(res.Output), connToken) {
		t.Fatalf("token leaked: %s", res.Output)
	}
	var out struct {
		Base string         `json:"base"`
		J    map[string]any `json:"j"`
		G    string         `json:"g"`
		E    map[string]any `json:"e"`
		None any            `json:"none"`
	}
	if err := json.Unmarshal(res.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out.Base != "https://api.example.com/v1" || out.J["body"] != "hi" || out.G != "fetch" || out.None != nil {
		t.Fatalf("output: %s", res.Output)
	}
	if out.E["token"] != "***" || out.E["args"].(map[string]any)["n"] != float64(1) {
		t.Fatalf("operation result: %v", out.E)
	}
	if len(fo.calls) != 2 || !fo.calls[0].Fetch || fo.calls[0].HTTPMethod != "POST" || fo.calls[0].Header.Get("X-A") != "1" || fo.calls[1].HTTPMethod != "GET" {
		t.Fatalf("fetch calls: %+v", fo.calls)
	}
	if len(fo.opCalls) != 2 || string(fo.opCalls[0].Args) != `{"n":1}` || string(fo.opCalls[1].Args) != "null" {
		t.Fatalf("operation calls: %+v", fo.opCalls)
	}
}

func TestOperationErrorsAreCodedAndMasked(t *testing.T) {
	res := runCode(t, `export default async function run(ctx, env) {
		let a, b;
		try { await env.connections.api.refuse(); } catch (e) { a = { name: e.name, code: e.code, retryable: e.retryable, message: e.message }; }
		try { await env.connections.api.boom(); } catch (e) { b = e.message; }
		return { outcome: "done", output: { a, b } };
	}`, RunInput{
		ConnectionSpecs:  map[string]ConnectionSpec{"api": {Operations: []OperationSpec{{Name: "refuse"}, {Name: "boom"}}}},
		ConnectionCaller: &fakeOps{}, Masked: []string{connToken},
	})
	if res.Err != nil {
		t.Fatalf("run failed: %v", res.Err)
	}
	var out struct {
		A struct {
			Name      string `json:"name"`
			Code      string `json:"code"`
			Message   string `json:"message"`
			Retryable bool   `json:"retryable"`
		} `json:"a"`
		B string `json:"b"`
	}
	if err := json.Unmarshal(res.Output, &out); err != nil {
		t.Fatal(err)
	}
	if out.A.Name != "ConnectionError" || out.A.Code != "invalid_args" || !out.A.Retryable || out.A.Message != "bad chat ***" {
		t.Fatalf("refused: %+v", out.A)
	}
	if strings.Contains(out.B, connToken) || !strings.Contains(out.B, "provider said") {
		t.Fatalf("masked error: %q", out.B)
	}
}

func TestOperationsNeedAnOperatorAndCountAsFetches(t *testing.T) {
	spec := map[string]ConnectionSpec{"api": {Operations: []OperationSpec{{Name: "echo"}}}}
	res := runCode(t, `export default async function run(ctx, env) { await env.connections.api.echo(); return { outcome: "done" }; }`,
		RunInput{ConnectionSpecs: spec, ConnectionCaller: &fakeConnections{}})
	wantReason(t, res, ReasonConnectionMissing)
	res = runCode(t, `export default async function run(ctx, env) {
		await env.connections.api.echo(); await env.connections.api.echo(); await env.connections.api.echo();
		return { outcome: "done" };
	}`, RunInput{ConnectionSpecs: spec, ConnectionCaller: &fakeOps{}, Limits: Limits{MaxFetches: 2}})
	if res.Err == nil || !strings.Contains(res.Err.Message, "at most 2 fetches") {
		t.Fatalf("limit: %+v", res.Err)
	}
}

func TestReservedAndClashingConnectionNamesAreRefused(t *testing.T) {
	for _, spec := range []ConnectionSpec{
		{Methods: []string{"fetch"}}, {Operations: []OperationSpec{{Name: "baseUrl"}}},
		{Methods: []string{"get"}, Operations: []OperationSpec{{Name: "get"}}},
	} {
		_, err := testEngine(t).Run(context.Background(), mustCompile(t, LangJS, `export default async () => ({outcome: "done"})`),
			RunInput{Outcomes: []string{"done"}, ConnectionSpecs: map[string]ConnectionSpec{"crm": spec}})
		if err == nil {
			t.Fatalf("%+v was accepted", spec)
		}
	}
}

func TestDeclarationsForSpecs(t *testing.T) {
	d := DeclarationsForSpecs(expr.Scope{}, []string{"done"}, nil, map[string]ConnectionSpec{"tg": {
		BaseURL: "https://x.example.com", Fetch: true, Methods: []string{"get"},
		Operations: []OperationSpec{{Name: "sendMessage", Description: "Send a message", Params: "{ chat_id: string; text: string }", Result: "{ message_id: number }"}, {Name: "ping"}},
	}})
	for _, want := range []string{"readonly baseUrl: string;", "fetch(path: string, init?: CodeFetchInit): Promise<CodeFetchResponse>;",
		"/** Send a message */", "sendMessage(args: { chat_id: string; text: string }): Promise<{ message_id: number }>;", "ping(): Promise<unknown>;"} {
		if !strings.Contains(d, want) {
			t.Fatalf("declarations lack %q:\n%s", want, d)
		}
	}
}
