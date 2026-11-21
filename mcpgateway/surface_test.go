package mcpgateway_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/mcpgateway"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/chattools"
	"github.com/iota-uz/iota-sdk/pkg/mcp/lookup"
)

// TestGateway_LookupSurface: once an application registers the lookup entry
// point, the gateway and the chat publish entry points only; operations are
// reached through lookup and a direct call of the former top-level tool is an
// unknown tool.
func TestGateway_LookupSurface(t *testing.T) {
	a := newApp(t)
	describe, err := lookup.Describe(a.engine, lookup.DescribeConfig{Policy: mcp.Policy{Scopes: []string{scope}}})
	require.NoError(t, err)
	entry, err := lookup.New(a.engine, lookup.Config{Policy: mcp.Policy{Scopes: []string{scope}}})
	require.NoError(t, err)
	require.NoError(t, a.engine.Registry().Register(describe, entry))

	server, err := mcpgateway.NewServer(a.engine, &sdk.Implementation{Name: "test-erp", Version: "v2"}, nil)
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle("/mcp", a.oauth.Handler(server, nil))
	a.server.Config.Handler = mux
	cs := a.connect(t, a.token(t, nil))
	ctx := context.Background()

	listed, err := cs.ListTools(ctx, nil)
	require.NoError(t, err)
	require.Len(t, listed.Tools, 1)
	assert.Equal(t, lookup.ToolName, listed.Tools[0].Name)
	assert.True(t, listed.Tools[0].Annotations.ReadOnlyHint)

	chat, err := chattools.Tools(a.engine)
	require.NoError(t, err)
	require.Len(t, chat, 1)
	assert.Equal(t, lookup.ToolName, chat[0].Name())

	args := json.RawMessage(`{"method":"quote","input":{"customer":"acme","amount":"10.50"}}`)
	res, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: lookup.ToolName, Arguments: args})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.JSONEq(t, `{"ok":true,"data":{"total":"10.50 UZS"}}`, mustJSON(t, res.StructuredContent))
	viaChat, err := chat[0].Call(a.chatContext(), string(args))
	require.NoError(t, err)
	assert.JSONEq(t, viaChat, mustJSON(t, res.StructuredContent))

	unknown, err := cs.CallTool(ctx, &sdk.CallToolParams{Name: lookup.ToolName, Arguments: json.RawMessage(`{"method":"nope","input":{}}`)})
	require.NoError(t, err)
	assert.True(t, unknown.IsError)
	assert.Contains(t, mustJSON(t, unknown.StructuredContent), `"code":"not_found"`)

	_, err = cs.CallTool(ctx, &sdk.CallToolParams{Name: "quote", Arguments: json.RawMessage(`{"customer":"acme","amount":"1"}`)})
	assert.Error(t, err, "the former top-level tool is no longer published")
}
