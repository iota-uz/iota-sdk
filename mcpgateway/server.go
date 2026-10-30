// Package mcpgateway serves the shared pkg/mcp registry over the Model Context
// Protocol using the official Go SDK. It lives in its own Go module so its Go
// and dependency requirements never leak into the main SDK module.
package mcpgateway

import (
	"context"
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const principalKey = "iota.principal"

// NewServer publishes every registered tool. Tool failures travel as
// structuredContent with isError; only protocol problems become MCP errors.
func NewServer(engine *mcp.Engine, impl *sdk.Implementation, opts *sdk.ServerOptions) (*sdk.Server, error) {
	const op serrors.Op = "mcpgateway.NewServer"
	server := sdk.NewServer(impl, opts)
	failure, err := jsonschema.For[mcp.Failure](nil)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	for _, spec := range engine.Registry().Specs() {
		server.AddTool(toolOf(spec, failure), handler(engine, spec.Name))
	}
	return server, nil
}

func toolOf(spec mcp.Spec, failure *jsonschema.Schema) *sdk.Tool {
	destructive, openWorld := spec.Annotations.Destructive, spec.Annotations.OpenWorld
	t := &sdk.Tool{
		Name:        spec.Name,
		Description: spec.Description,
		InputSchema: spec.Input,
		Annotations: &sdk.ToolAnnotations{
			Title:           spec.Annotations.Title,
			ReadOnlyHint:    spec.Annotations.ReadOnly,
			DestructiveHint: &destructive,
			IdempotentHint:  spec.Annotations.Idempotent,
			OpenWorldHint:   &openWorld,
		},
		Meta: spec.Meta,
	}
	if spec.Output != nil {
		t.OutputSchema = ResultSchema(spec.Output, failure)
	}
	return t
}

// ResultSchema wraps a tool's data schema in the {ok, data} | {ok, error}
// envelope every adapter returns.
func ResultSchema(data, failure *jsonschema.Schema) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		OneOf: []*jsonschema.Schema{
			{
				Required:   []string{"ok", "data"},
				Properties: map[string]*jsonschema.Schema{"ok": {Const: constant(true)}, "data": data},
			},
			{
				Required:   []string{"ok", "error"},
				Properties: map[string]*jsonschema.Schema{"ok": {Const: constant(false)}, "error": failure},
			},
		},
	}
}

func constant(v any) *any { return &v }

func handler(engine *mcp.Engine, name string) sdk.ToolHandler {
	return func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		if p, ok := principalOf(req); ok {
			ctx = mcp.WithPrincipal(ctx, p)
		}
		ctx = mcp.WithRequestID(ctx, uuid.NewString())
		res := engine.Call(ctx, name, req.Params.Arguments)
		raw, err := json.Marshal(res)
		if err != nil {
			return nil, err
		}
		return &sdk.CallToolResult{
			Content:           []sdk.Content{&sdk.TextContent{Text: string(raw)}},
			StructuredContent: json.RawMessage(raw),
			IsError:           !res.OK,
		}, nil
	}
}

func principalOf(req *sdk.CallToolRequest) (mcp.Principal, bool) {
	if req.Extra == nil || req.Extra.TokenInfo == nil {
		return mcp.Principal{}, false
	}
	p, ok := req.Extra.TokenInfo.Extra[principalKey].(mcp.Principal)
	return p, ok
}
