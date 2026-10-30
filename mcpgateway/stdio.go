package mcpgateway

import (
	"context"
	"net/http"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServeStdio serves a local stdio client by forwarding to the protected HTTP
// endpoint with the account's own access: client must attach the user's
// bearer token (e.g. an oauth2 transport). There is no separate stdio auth.
func ServeStdio(ctx context.Context, endpoint string, client *http.Client, impl *sdk.Implementation) error {
	remote, err := sdk.NewClient(impl, nil).Connect(ctx, &sdk.StreamableClientTransport{Endpoint: endpoint, HTTPClient: client}, nil)
	if err != nil {
		return err
	}
	defer func() { _ = remote.Close() }()
	server, err := Proxy(ctx, remote, impl)
	if err != nil {
		return err
	}
	return server.Run(ctx, &sdk.StdioTransport{})
}

// Proxy mirrors the remote tool contracts unchanged, so schemas and failures
// seen over stdio are exactly those of the HTTP server.
func Proxy(ctx context.Context, remote *sdk.ClientSession, impl *sdk.Implementation) (*sdk.Server, error) {
	server := sdk.NewServer(impl, nil)
	for t, err := range remote.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		name := t.Name
		server.AddTool(t, func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
			return remote.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: req.Params.Arguments})
		})
	}
	return server, nil
}
