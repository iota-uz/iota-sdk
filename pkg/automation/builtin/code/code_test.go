package code_test

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	sandbox "github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

func TestValidateParamsReportsCompileErrors(t *testing.T) {
	n := code.New(code.Deps{})
	errs := n.ValidateParams(context.Background(), map[string]any{"lang": "ts", "source": "export default async function run( {\n  return 1;\n}"})
	if len(errs) != 1 || errs[0].Path != "params.source" || errs[0].Line == 0 {
		t.Fatalf("errs: %+v", errs)
	}
}

type conns map[string]access.Connection

func (c conns) Open(_ context.Context, _ uuid.UUID, name string) (access.Connection, error) {
	if v, ok := c[name]; ok {
		return v, nil
	}
	return access.Connection{}, access.ErrConnectionMissing
}

// api is the connector's API: it answers with the token it was sent, as a careless API would.
type api struct{ seen []string }

func (a *api) Fetch(_ context.Context, req sandbox.FetchRequest) (*sandbox.FetchResponse, error) {
	a.seen = append(a.seen, req.Method+" "+req.URL+" "+req.Header.Get("Authorization"))
	body := `{"deals": [{"id": 1}], "auth": "` + req.Header.Get("Authorization") + `"}`
	return &sandbox.FetchResponse{Status: 200, Header: nethttp.Header{"Content-Type": {"application/json"}}, Body: []byte(body), URL: req.URL}, nil
}

var (
	engOnce sync.Once
	eng     sandbox.Engine
	errEng  error
)

func testNode(t *testing.T, c conns, f sandbox.Fetcher) *code.Node {
	t.Helper()
	engOnce.Do(func() { eng, errEng = sandbox.NewEngine(context.Background()) })
	if errEng != nil {
		t.Fatal(errEng)
	}
	return code.New(code.Deps{Engine: eng, Connections: c, Fetcher: f})
}

func TestCodeReadsThroughAConnectionWithoutItsToken(t *testing.T) {
	ca := connectors.CodeAccess{BaseURL: "https://crm.example.com/api", Methods: []connectors.CodeMethod{{Name: "get", HTTPMethod: "GET"}, {Name: "list", HTTPMethod: "GET"}}}
	c := access.NewConnection("crm", "acme", ca, []string{"tok-5ecret-9"}, func(_ context.Context, h nethttp.Header) error {
		h.Set("Authorization", "Bearer tok-5ecret-9")
		return nil
	})
	a := &api{}
	n := testNode(t, conns{"crm": c}, a)
	res, err := n.Execute(context.Background(), catalog.CallInput{TenantID: uuid.New(), Deadline: time.Now().Add(time.Minute), Params: map[string]any{
		"lang": "ts", "connections": []any{"crm"}, "outcomes": []any{"done"},
		"source": `export default async function run(ctx, env) {
			const r = await env.connections.crm.list("/deals");
			const data = await r.json();
			env.log("auth was", data.auth);
			return { outcome: "done", output: { count: data.deals.length, auth: data.auth } };
		}`,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "done" {
		t.Fatalf("outcome %s: %v", res.Outcome, res.Output)
	}
	if len(a.seen) != 1 || a.seen[0] != "GET https://crm.example.com/api/deals Bearer tok-5ecret-9" {
		t.Fatalf("requests %v", a.seen)
	}
	raw, err := json.Marshal(res.Output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "tok-5ecret-9") {
		t.Fatalf("the token leaked: %s", raw)
	}
	out := res.Output["output"].(map[string]any)
	if out["count"] != float64(1) || out["auth"] != "Bearer ***" {
		t.Fatalf("output %v", out)
	}

	// A path to another host is refused before it reaches the network.
	res, err = n.Execute(context.Background(), catalog.CallInput{TenantID: uuid.New(), Deadline: time.Now().Add(time.Minute), Params: map[string]any{
		"lang": "js", "connections": []any{"crm"},
		"source": `export default async function run(ctx, env) { await env.connections.crm.get("https://evil.example.org/x"); return { outcome: "done" }; }`,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "failed" || res.Output["error"].(map[string]any)["code"] != "connection_origin_mismatch" {
		t.Fatalf("origin mismatch: %+v", res)
	}
	if len(a.seen) != 1 {
		t.Fatalf("the refused call reached the network: %v", a.seen)
	}

	// A connection that is not connected fails before the code runs.
	res, err = n.Execute(context.Background(), catalog.CallInput{TenantID: uuid.New(), Deadline: time.Now().Add(time.Minute), Params: map[string]any{
		"lang": "js", "connections": []any{"mail"}, "source": `export default async () => ({ outcome: "done" })`,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Output["error"].(map[string]any)["code"] != "connection_missing" {
		t.Fatalf("missing connection: %+v", res)
	}
}

func TestCodeSecretMissing(t *testing.T) {
	n := testNode(t, nil, nil)
	res, err := n.Execute(context.Background(), catalog.CallInput{TenantID: uuid.New(), Deadline: time.Now().Add(time.Minute), Params: map[string]any{
		"lang": "js", "secrets": []any{"API_KEY"}, "source": `export default async () => ({ outcome: "done" })`,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "failed" || res.Output["error"].(map[string]any)["code"] != "secret_missing" {
		t.Fatalf("%+v", res)
	}
}
