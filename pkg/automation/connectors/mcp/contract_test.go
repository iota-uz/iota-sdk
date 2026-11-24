package mcp

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/mcp/mcptest"
)

// TestContract is the connector's contract with an MCP server: Test, tools/list, the pin of a tool and
// one call with the pinned schema. CONTRACT_TARGET=fake (the default) runs it against mcptest;
// CONTRACT_TARGET=live against a real server:
//
//	MCP_CONTRACT_URL    the Streamable HTTP endpoint (https)
//	MCP_CONTRACT_TOKEN  a bearer token (optional)
//	MCP_CONTRACT_TOOL   a tool without side effects to call
//	MCP_CONTRACT_ARGS   its arguments as a JSON object of {"<name>": {"value": ...}} (optional)
func TestContract(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var (
		c     *Connector
		creds connectors.Credentials
		tool  string
		args  map[string]any
	)
	st := newMemStore()
	switch target := os.Getenv("CONTRACT_TARGET"); target {
	case "", "fake":
		srv := mcptest.New(t)
		srv.SetTool(weatherTool(func(a map[string]any) mcptest.Result {
			return mcptest.Result{Text: "sunny in " + a["city"].(string), Structured: map[string]any{"temp": 20}}
		}))
		srv.RequireToken("contract-token")
		c = testConnector(t, srv, st, nil)
		creds = connectors.Credentials{KeyServerURL: srv.URL, KeyAuth: AuthBearer, KeyToken: "contract-token"}
		tool, args = "weather", map[string]any{"city": map[string]any{"value": "Tashkent"}}
	case "live":
		url := os.Getenv("MCP_CONTRACT_URL")
		tool = os.Getenv("MCP_CONTRACT_TOOL")
		if url == "" || tool == "" {
			t.Skip("CONTRACT_TARGET=live needs MCP_CONTRACT_URL and MCP_CONTRACT_TOOL")
		}
		c = New(WithStore(st))
		creds = connectors.Credentials{KeyServerURL: url, KeyAuth: AuthNone}
		if tok := os.Getenv("MCP_CONTRACT_TOKEN"); tok != "" {
			creds[KeyAuth], creds[KeyToken] = AuthBearer, tok
		}
		args = map[string]any{}
		if raw := os.Getenv("MCP_CONTRACT_ARGS"); raw != "" {
			require.NoError(t, json.Unmarshal([]byte(raw), &args))
		}
	default:
		t.Fatalf("CONTRACT_TARGET is fake or live, not %q", target)
	}

	identity, err := c.Test(ctx, creds)
	require.NoError(t, err)
	require.NotEmpty(t, identity.Display)

	id := st.add(tenant, creds)
	tools, err := c.Tools(ctx, tenant, id)
	require.NoError(t, err)
	require.NotEmpty(t, tools)
	for _, tl := range tools {
		require.NotEmpty(t, tl.Name)
		require.True(t, json.Valid(tl.InputSchema), tl.Name)
	}

	params := map[string]any{"tool": tool, "arguments": args}
	prep, err := c.Node().Prepare(ctx, catalog.PrepareInput{TenantID: tenant, NodeID: "n1", Params: params,
		Connection: &catalog.ConnectionInfo{ID: id.String(), Kind: Kind}, Fill: true})
	require.NoError(t, err)
	require.Empty(t, prep.Errors)
	require.NotNil(t, prep.Params[ParamToolSchema], "the tool's schema is pinned")

	res, err := c.Node().Execute(ctx, callInput(id, creds, prep.Params))
	require.NoError(t, err)
	require.Equal(t, OutcomeDone, res.Outcome)
	_, isText := res.Output["text"].(string)
	require.True(t, isText)
}
