package engine_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/mcp"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/mcp/mcptest"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// An MCP tool's schema is pinned in the version when the graph is saved. The server changes the tool's
// input schema afterwards: validation warns (with the fix that updates the pin), the version does not
// change, and runs keep calling the tool with the arguments of the pinned schema.
func TestMCPToolSchemaIsPinnedInTheVersion(t *testing.T) {
	srv := mcptest.New(t)
	srv.SetTool(mcptest.Tool{Name: "weather",
		InputSchema: map[string]any{"type": "object", "required": []any{"city"}, "properties": map[string]any{
			"city": map[string]any{"type": "string"}, "days": map[string]any{"type": "integer"}}},
		Handle: func(a map[string]any) mcptest.Result { return mcptest.Result{Text: "sunny in " + a["city"].(string)} },
	})
	_, port := srv.Host()
	p16, err := strconv.Atoi(port)
	require.NoError(t, err)
	conn := mcp.New(mcp.WithTLSConfig(srv.TLS()), mcp.WithAllowedPorts(uint16(p16)),
		mcp.WithAddressPolicy(func(_, address string) error {
			if address == srv.Addr {
				return nil
			}
			return mcp.ErrBlocked
		}))

	var store connectors.Store
	keys, err := connectors.NewKeyring("k1", map[string][]byte{"k1": []byte("0123456789abcdef0123456789abcdef")})
	require.NoError(t, err)
	e := newEnvWith(t, func(db *pgxpool.Pool) catalog.Availability {
		store = connectors.NewStore(db, keys, nil, nil, nil)
		return store
	})
	dir := connectors.NewDirectory(e.reg)
	require.NoError(t, dir.Register(conn))
	connectors.BindStore(dir, store)
	ctx := context.Background()
	saved0, err := store.Save(ctx, connectors.Connection{TenantID: e.tenant, Kind: mcp.Kind, Name: "tools", Status: connectors.StatusConnected},
		connectors.Credentials{mcp.KeyServerURL: srv.URL, mcp.KeyAuth: mcp.AuthNone})
	require.NoError(t, err)
	p := e.proc(func(c *engine.Config) { c.Credentials = store })

	g := graphJSON("Weather", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "call", "type": "mcp_tool", "params": {"tool": "weather", "arguments": {
			"city": {"template": "Tashkent"}, "days": {"cel": "event.amount + 1"}}}},
		 {"id": "ok", "type": "acme.note", "params": {"text": "{{ nodes.call.output.text }}"}},
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "call"},
		 {"from": {"node": "call", "outcome": "done"}, "to": "ok"}, {"from": {"node": "call", "outcome": "failed"}, "to": "fin"},
		 {"from": {"node": "ok", "outcome": "done"}, "to": "fin"}, {"from": {"node": "ok", "outcome": "failed"}, "to": "fin"}`)
	am := e.create(p, e.human(), g)

	v, err := p.rt.Service().GetVersion(ctx, e.human(), am.ID, 1)
	require.NoError(t, err)
	var saved struct {
		Nodes []struct {
			ID     string         `json:"id"`
			Params map[string]any `json:"params"`
		} `json:"nodes"`
	}
	require.NoError(t, json.Unmarshal(v.Graph, &saved))
	require.NotNil(t, saved.Nodes[0].Params["tool_schema"], "the save pinned the tool's schema")

	runOnce := func(key string) {
		t.Helper()
		before := len(srv.Calls())
		e.publish(e.tenant, orderCreated, key, order{Amount: 2}, 0)
		p.until(func() bool {
			return len(srv.Calls()) == before+1 && e.count(`SELECT count(*) FROM test_effects`) == before+1
		}, "the run calls the tool")
	}
	runOnce("o-1")
	require.Equal(t, map[string]any{"city": "Tashkent", "days": float64(3)}, srv.Calls()[0].Arguments)

	// The server changes the input schema.
	srv.SetTool(mcptest.Tool{Name: "weather",
		InputSchema: map[string]any{"type": "object", "required": []any{"city", "country"}, "properties": map[string]any{
			"city": map[string]any{"type": "string"}, "country": map[string]any{"type": "string"}}},
		Handle: func(a map[string]any) mcptest.Result { return mcptest.Result{Text: "sunny"} },
	})
	// The connector reuses a tools/list for five minutes per connection version: a change of the
	// connection lists again.
	require.NoError(t, store.SetStatus(ctx, e.tenant, saved0.ID, connectors.StatusConnected, ""))
	warnings, err := p.rt.Authoring().Validate(ctx, e.human(), v.Graph)
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	require.Equal(t, mcp.CodeSchemaChanged, warnings[0].Code)
	require.Equal(t, "nodes[0].params.tool_schema", warnings[0].Path)
	require.NotNil(t, warnings[0].Fix)
	require.Equal(t, "nodes[0].params.tool_schema", warnings[0].Fix.Remove)

	got, err := p.rt.Service().Get(ctx, e.human(), am.ID)
	require.NoError(t, err)
	require.Equal(t, 1, got.LatestVersion, "the version does not change")

	runOnce("o-2")
	require.Equal(t, map[string]any{"city": "Tashkent", "days": float64(3)}, srv.Calls()[1].Arguments, "runs keep the pinned arguments")
	effects := e.count(`SELECT count(*) FROM test_effects WHERE text LIKE 'sunny%'`)
	require.Equal(t, 2, effects)

	// A connection whose sign-in was refused: its calls fail as unauthorized, not as a missing connection.
	require.NoError(t, store.SetStatus(ctx, e.tenant, saved0.ID, connectors.StatusNeedsReauth, "refused"))
	e.publish(e.tenant, orderCreated, "o-3", order{Amount: 2}, 0)
	var code string
	p.until(func() bool {
		for _, r := range e.runs(p, e.human(), engine.RunFilter{}) {
			if r.Subject.Key != "o-3" || (r.Status != engine.RunDone && r.Status != engine.RunFailed) {
				continue
			}
			d := e.run(p, e.human(), r.RunID)
			if d.Error != nil {
				code = d.Error.Code
			}
			for _, st := range d.Steps {
				if st.NodeID == "call" && st.Error != nil {
					code = st.Error.Code
				}
			}
			return true
		}
		return false
	}, "the run ends")
	require.Equal(t, catalog.CallUnauthorized, code)

	// Test node of the same node answers unauthorized, not a validation failure.
	tested, err := p.rt.Authoring().TestNode(ctx, e.human(), engine.TestNodeInput{Graph: v.Graph, NodeID: "call"})
	require.NoError(t, err)
	require.NotNil(t, tested.Error)
	require.Equal(t, catalog.CallUnauthorized, tested.Error.Code)
	require.Len(t, srv.Calls(), 2, "nothing reached the server")
}
