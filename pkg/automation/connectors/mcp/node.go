package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// NodeType is the type of the node that calls a tool of an MCP server.
const NodeType = "mcp_tool"

// Outcomes of mcp_tool.
const (
	OutcomeDone   = "done"
	OutcomeFailed = "failed"
)

// Params of mcp_tool, as JSON.
const (
	ParamTool       = "tool"
	ParamArguments  = "arguments"
	ParamToolSchema = "tool_schema"
	ParamTimeout    = "timeout" // milliseconds; the engine's call deadline reads it
)

// Timeout bounds of mcp_tool, in milliseconds. The engine caps a call by its CallLimits.MaxTimeout too.
const (
	MinTimeoutMs     = 1000
	MaxTimeoutMs     = 600000
	DefaultTimeoutMs = 60000
)

// ToolParams are the params of mcp_tool.
type ToolParams struct {
	Connection string              `json:"connection,omitempty"`
	Tool       string              `json:"tool"`                  // 1..200, a name of tools/list
	Arguments  map[string]Argument `json:"arguments,omitempty"`   // a key is an argument or a dotted path into an object argument
	ToolSchema *ToolPin            `json:"tool_schema,omitempty"` // filled when the graph is saved, if absent
	TimeoutMs  int                 `json:"timeout,omitempty"`     // 1000..600000, default 60000
}

// ToolOutput is the output of mcp_tool.
type ToolOutput struct {
	// Structured is structuredContent: typed by the pinned OutputSchema, else any.
	Structured json.RawMessage `json:"structured,omitempty"`
	// Text is the text blocks of content, joined by new lines; image, audio and resource blocks are
	// dropped.
	Text string `json:"text"`
}

const toolParamsSchema = `{
	"type": "object",
	"required": ["tool"],
	"properties": {
		"tool": {"type": "string", "minLength": 1, "maxLength": 200, "title": "Tool"},
		"arguments": {
			"type": "object", "title": "Arguments", "maxProperties": 200,
			"propertyNames": {"minLength": 1, "maxLength": 200},
			"additionalProperties": {
				"type": "object", "minProperties": 1, "maxProperties": 1,
				"properties": {"template": {"type": "string", "maxLength": 20000}, "cel": {"type": "string", "minLength": 1, "maxLength": 4000}, "value": {}},
				"additionalProperties": false
			}
		},
		"tool_schema": {
			"type": "object", "title": "Pinned tool schema",
			"required": ["input_schema", "hash"],
			"properties": {
				"input_schema": {"type": "object"},
				"output_schema": {"type": "object"},
				"hash": {"type": "string", "pattern": "^[0-9a-f]{64}$"},
				"read_only": {"type": "boolean"}
			},
			"additionalProperties": false
		},
		"timeout": {"type": "integer", "minimum": 1000, "maximum": 600000, "default": 60000, "title": "Timeout, ms"}
	},
	"additionalProperties": false
}`

var toolParams = graph.MustSchemaParams(toolParamsSchema)

// ToolNode is mcp_tool: a call of one tool of the connection's server (behavior call, not idempotent).
type ToolNode struct{ c *Connector }

var (
	_ catalog.CallNode      = (*ToolNode)(nil)
	_ catalog.PreparingNode = (*ToolNode)(nil)
	_ catalog.LocalizedNode = (*ToolNode)(nil)
)

func (n *ToolNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: NodeType, Title: "MCP tool", Description: "Calls a tool of an MCP server",
		Behavior: graph.BehaviorCall, Params: toolParams,
		Outcomes: []string{OutcomeDone, OutcomeFailed}, DoneOutcome: OutcomeDone, FailedOutcome: OutcomeFailed,
		Templates: []string{ParamArguments + "." + graph.AnyKey + ".template"},
		Values:    []string{ParamArguments + "." + graph.AnyKey + ".cel"},
		Output: expr.Fields{
			{Name: "structured", Field: expr.OptionalOf(expr.DynField)},
			{Name: "text", Field: expr.StringField},
		},
		OutputSchemas: map[string]string{"structured": ParamToolSchema + ".output_schema"},
		// A tool may have any effect: a dry run captures it, unless its server declares it read-only.
		DryRun: graph.DryRunCapture, ReadOnlyParam: ParamToolSchema + ".read_only",
	}
}

func (n *ToolNode) Localize(spec graph.NodeSpec, locale string) graph.NodeSpec {
	switch locale {
	case "ru":
		spec.Title, spec.Description = "Инструмент MCP", "Вызывает инструмент MCP-сервера"
	case "uz":
		spec.Title, spec.Description = "MCP vositasi", "MCP server vositasini chaqiradi"
	case "oz":
		spec.Title, spec.Description = "MCP воситаси", "MCP сервер воситасини чақиради"
	}
	return spec
}

func (n *ToolNode) Requires() []catalog.ConnectorKind { return []catalog.ConnectorKind{Kind} }

// Idempotent is false: a tool may have any effect and its annotations are hints, so a call whose outcome
// is unknown is never repeated automatically.
func (n *ToolNode) Idempotent() bool { return false }

// decodeParams reads params into ToolParams. Arguments are typed only while unrendered (a save): once
// rendered, a cel entry holds its value, so they stay the generic maps the second result returns.
func decodeParams(p map[string]any, rendered bool) (ToolParams, map[string]any, error) {
	args, _ := p[ParamArguments].(map[string]any)
	typed := p
	if rendered {
		typed = make(map[string]any, len(p))
		for k, v := range p {
			if k != ParamArguments {
				typed[k] = v
			}
		}
	}
	raw, err := json.Marshal(typed)
	if err != nil {
		return ToolParams{}, nil, err
	}
	var out ToolParams
	if err := json.NewDecoder(bytes.NewReader(raw)).Decode(&out); err != nil {
		return ToolParams{}, nil, err
	}
	return out, args, nil
}

func callErr(code, msg string) error { return &catalog.CallError{Code: code, Message: msg} }

// Execute calls the tool once with the arguments rendered by the pinned schema.
func (n *ToolNode) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	p, rendered, err := decodeParams(in.Params, true)
	if err != nil {
		return catalog.CallResult{}, callErr(catalog.CallInvalidRequest, "the params of the node are not readable")
	}
	if p.ToolSchema == nil {
		return catalog.CallResult{}, callErr(catalog.CallInvalidRequest, "the node has no pinned tool schema: save the graph again")
	}
	cf, err := readConfig(in.Credentials)
	if err != nil {
		return catalog.CallResult{}, callErr(catalog.CallInvalidRequest, err.Error())
	}
	args, err := buildArguments(p.ToolSchema.InputSchema, rendered)
	if err != nil {
		return catalog.CallResult{}, callErr(catalog.CallInvalidRequest, err.Error())
	}
	var connID uuid.UUID
	if in.Connection != nil {
		connID, _ = uuid.Parse(in.Connection.ID)
	}
	token, refreshed, err := n.c.tokenFor(ctx, in.TenantID, connID, cf, false)
	if err != nil {
		return catalog.CallResult{}, err
	}
	res, err := n.c.call(ctx, cf, token, p.Tool, args)
	if refused401(err) && cf.auth == AuthOAuth2 && !refreshed && connID != uuid.Nil {
		// A 401 is a refusal before the tool ran: refresh once and send again.
		if token, _, err = n.c.tokenFor(ctx, in.TenantID, connID, cf, true); err != nil {
			return catalog.CallResult{}, err
		}
		res, err = n.c.call(ctx, cf, token, p.Tool, args)
	}
	if refused401(err) && cf.auth == AuthOAuth2 && connID != uuid.Nil {
		n.c.markReauth(ctx, in.TenantID, connID, cf.creds, token, "the server refused the access token after a refresh")
	}
	var rl *rateLimitedError
	if errors.As(err, &rl) {
		if rl.after > 0 {
			return catalog.CallResult{RetryAfter: rl.after}, nil
		}
		return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallRateLimited, Message: "the server asked to slow down (HTTP 429)", Retryable: true}
	}
	if err != nil {
		return catalog.CallResult{}, err
	}
	if in.Usage != nil && in.Connection != nil {
		_ = in.Usage.Record(ctx, catalog.UsageEntry{TenantID: in.TenantID, Kind: Kind, ConnectionID: in.Connection.ID, Unit: "request", Quantity: 1, RunID: &in.RunID, At: time.Now()})
	}
	sc := newScrubber(in.Credentials, token)
	out, err := toolOutput(*p.ToolSchema, res.res)
	if err != nil {
		var ce *catalog.CallError
		if errors.As(err, &ce) {
			return catalog.CallResult{}, &catalog.CallError{Code: ce.Code, Message: sc.text(ce.Message), Retryable: ce.Retryable}
		}
		return catalog.CallResult{}, err
	}
	out.Text, out.Structured = sc.text(out.Text), sc.raw(out.Structured)
	output := map[string]any{"text": out.Text}
	if len(out.Structured) > 0 {
		var v any
		if err := expr.UnmarshalJSON(out.Structured, &v); err != nil {
			return catalog.CallResult{}, callErr(catalog.CallInvalidRequest, "structuredContent is not JSON")
		}
		output["structured"] = v
	}
	return catalog.CallResult{Outcome: OutcomeDone, Output: output}, nil
}

// maxErrorText bounds the tool's own text in a rejected outcome's message.
const maxErrorText = 2000

// toolOutput is the node's output of a tools/call result: rejected when the tool reports an error or its
// structuredContent does not match the pinned output schema.
func toolOutput(pin ToolPin, res *mcpsdk.CallToolResult) (ToolOutput, error) {
	var text []string
	for _, c := range res.Content {
		if t, ok := c.(*mcpsdk.TextContent); ok {
			text = append(text, t.Text)
		}
	}
	joined := strings.Join(text, "\n")
	if res.IsError {
		msg := strings.TrimSpace(joined)
		if msg == "" {
			msg = "the tool reported an error"
		}
		return ToolOutput{}, callErr(catalog.CallRejected, clip(msg, maxErrorText))
	}
	out := ToolOutput{Text: joined}
	if res.StructuredContent == nil && len(pin.OutputSchema) > 0 {
		return ToolOutput{}, callErr(catalog.CallRejected, "the tool declares an output schema and gave no structuredContent")
	}
	if res.StructuredContent != nil {
		raw, err := json.Marshal(res.StructuredContent)
		if err != nil {
			return ToolOutput{}, callErr(catalog.CallInvalidRequest, "structuredContent is not JSON")
		}
		schema, err := compiledOutput(pin)
		if err != nil {
			return ToolOutput{}, callErr(catalog.CallRejected, "the pinned output schema does not compile")
		}
		if schema != nil {
			v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				return ToolOutput{}, callErr(catalog.CallInvalidRequest, "structuredContent is not JSON")
			}
			if err := schema.Validate(v); err != nil {
				return ToolOutput{}, callErr(catalog.CallRejected, "structuredContent does not match the pinned output schema")
			}
		}
		out.Structured = raw
	}
	return out, nil
}

// Prepare checks the node against the server when the graph is saved: the tool must exist, a node
// without a pin gets the tool's schema pinned, a pin whose server schema changed since is a warning, and
// the arguments are checked against the pin.
func (n *ToolNode) Prepare(ctx context.Context, in catalog.PrepareInput) (catalog.PrepareResult, error) {
	var res catalog.PrepareResult
	if in.Connection == nil {
		return res, nil
	}
	connID, err := uuid.Parse(in.Connection.ID)
	if err != nil {
		return res, nil //nolint:nilerr // a platform credential: no connection of a tenant to ask
	}
	p, _, err := decodeParams(in.Params, false)
	if err != nil {
		return res, nil //nolint:nilerr // the params check of the graph reports it
	}
	if p.ToolSchema != nil {
		if err := p.ToolSchema.check(); err != nil {
			res.Errors = append(res.Errors, catalog.PrepareIssue{Path: ParamToolSchema, Code: CodeSchemaInvalid,
				Message: "the pinned tool schema was changed by hand: " + err.Error() + "; remove tool_schema to pin the server's"})
			return res, nil //nolint:nilerr // the refusal is the result
		}
	}
	tools, listErr := n.c.Tools(ctx, in.TenantID, connID)
	pin := p.ToolSchema
	// readOnly is what the server says now: only a tool it lists as read-only, with the pinned schema,
	// runs for real in a dry run; anything uncertain is captured.
	readOnly := false
	switch {
	case listErr != nil && pin == nil:
		res.Errors = append(res.Errors, catalog.PrepareIssue{Path: ParamTool, Code: CodeToolsUnavailable,
			Message: "the tools of the server could not be listed (" + category(listErr) + "): the tool's schema cannot be pinned"})
		return res, nil
	case listErr != nil:
		res.Warnings = append(res.Warnings, catalog.PrepareIssue{Path: ParamTool, Code: CodeToolsUnavailable,
			Message: "the tools of the server could not be listed (" + category(listErr) + "): the tool was not checked"})
	default:
		var tool *Tool
		for i := range tools {
			if tools[i].Name == p.Tool {
				tool = &tools[i]
				break
			}
		}
		if tool == nil {
			res.Errors = append(res.Errors, catalog.PrepareIssue{Path: ParamTool, Code: CodeToolNotFound,
				Message: fmt.Sprintf("the server has no tool %s", p.Tool)})
			return res, nil
		}
		current, err := PinOf(*tool)
		if err != nil {
			res.Errors = append(res.Errors, catalog.PrepareIssue{Path: ParamTool, Code: CodeSchemaInvalid,
				Message: "the server's schema of the tool is not JSON"})
			return res, nil //nolint:nilerr // the refusal is the result
		}
		if pin == nil || pin.Hash == current.Hash {
			readOnly = current.ReadOnly
		}
		if pin == nil {
			pin = &current
			pinned, err := pinValue(current)
			if err != nil {
				return res, err
			}
			res.Params = make(map[string]any, len(in.Params)+1)
			for k, v := range in.Params {
				res.Params[k] = v
			}
			res.Params[ParamToolSchema] = pinned
		} else if pin.Hash != current.Hash {
			res.Warnings = append(res.Warnings, catalog.PrepareIssue{Path: ParamToolSchema, Code: CodeSchemaChanged,
				Message:   "the server changed the schema of " + p.Tool + " since it was pinned: runs keep the pinned one; update the pin to use the new schema",
				FixRemove: ParamToolSchema, FixLabel: "Update the pinned schema"})
		}
	}
	// The author's read_only is never trusted: a save or a try sets it from the server (a stored version
	// being enabled keeps its params; the dry run that would read them checks again).
	if in.Fill && pin.ReadOnly != readOnly {
		fixed := *pin
		fixed.ReadOnly = readOnly
		pinned, err := pinValue(fixed)
		if err != nil {
			return res, err
		}
		if res.Params == nil {
			res.Params = make(map[string]any, len(in.Params))
			for k, v := range in.Params {
				res.Params[k] = v
			}
		}
		res.Params[ParamToolSchema] = pinned
	}
	for _, i := range checkArguments(pin.InputSchema, p.Arguments, in.TypeOf) {
		res.Errors = append(res.Errors, catalog.PrepareIssue{Path: i.path, Code: i.code, Message: i.message})
	}
	return res, nil
}

// pinValue is a pin as a param value (the JSON the graph stores).
func pinValue(p ToolPin) (any, error) {
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var v any
	err = expr.UnmarshalJSON(raw, &v)
	return v, err
}

// category is the class of a failure for a message: never the server's text.
func category(err error) string {
	var rl *rateLimitedError
	if errors.As(err, &rl) {
		return catalog.CallRateLimited
	}
	var ce *catalog.CallError
	if errors.As(err, &ce) {
		return ce.Code
	}
	if errors.Is(err, connectors.ErrNeedsReauth) {
		return catalog.CallUnauthorized
	}
	if errors.Is(err, connectors.ErrNotConnected) || errors.Is(err, connectors.ErrNotFound) {
		return "connection_unavailable"
	}
	return catalog.CallUnavailable
}
