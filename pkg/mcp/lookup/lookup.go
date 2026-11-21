// Package lookup provides the two-entry-point surface of an IOTA SDK
// application: lookup runs one read-only operation from the shared registry
// without Python, and describe publishes the caller's catalog and the JSON
// Schema of each operation from the same registry. Composition and writes go
// through execute (pkg/mcp/execute), which binds the same operations into its
// Python tools namespace, so a read returns the same Result either way.
package lookup

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	// ToolName is the read entry point.
	ToolName = "lookup"
	// ExecuteName is the entry point that runs operations with side effects.
	ExecuteName = "execute"
)

type Input struct {
	Method string          `json:"method"`
	Input  json.RawMessage `json:"input"`
}

type Config struct {
	// Policy admits the caller to lookup itself; every method adds its own.
	Policy mcp.Policy
	// About is prepended to the published description, e.g. what the
	// application is.
	About string
}

// New returns the lookup entry point. Register it in the same registry as
// the operations it reaches.
func New(engine *mcp.Engine, cfg Config) (mcp.Tool, error) {
	const op serrors.Op = "lookup.New"
	if engine == nil {
		return nil, serrors.New(serrors.Invalid, "engine is required").WithOp(op)
	}
	description := "Run one read-only method and return its Result: {ok: true, data} or {ok: false, error}. " +
		"input is the method's input object, {} when it takes no arguments. " +
		"Start with lookup({method: \"describe\", input: {}}) for the methods available to you, and " +
		"lookup({method: \"describe\", input: {tool: \"<name>\"}}) for one method's input and output JSON Schema. " +
		"Methods with side effects, and several calls composed in one run, go through execute, where the same " +
		"methods are await tools.<name>(...) and return the same Result."
	if cfg.About != "" {
		description = cfg.About + "\n\n" + description
	}
	return &tool{engine: engine, spec: mcp.Spec{
		Name:        ToolName,
		Description: description,
		Input: &jsonschema.Schema{
			Type:                 "object",
			Required:             []string{"method", "input"},
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			Properties: map[string]*jsonschema.Schema{
				"method": {Type: "string", MinLength: jsonschema.Ptr(1), Description: "a read-only method from describe"},
				"input":  {Type: "object", Description: "the method's input; {} when it takes no arguments"},
			},
		},
		Annotations: mcp.Annotations{Title: "Lookup", ReadOnly: true, Idempotent: true},
		Policy:      cfg.Policy,
		EntryPoint:  true,
	}}, nil
}

type tool struct {
	engine *mcp.Engine
	spec   mcp.Spec
}

func (t *tool) Spec() mcp.Spec { return t.spec }

// Call classifies the method from the server registry before anything runs:
// a method missing from the caller's catalog is not_found, and an available
// method with side effects is refused before its handler with a pointer to
// execute. The method itself then runs through Engine.Call, which checks its
// policy and input schema again.
func (t *tool) Call(ctx context.Context, raw json.RawMessage) (any, error) {
	var in Input
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&in); err != nil {
		return nil, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "input does not match the tool schema")
	}
	spec, ok := t.engine.Operation(ctx, in.Method)
	if !ok {
		return nil, mcp.Fail(mcp.CodeNotFound, mcp.RecoveryFixInput,
			"method "+in.Method+" is not in your catalog; call lookup({method: \"describe\", input: {}})",
			mcp.Issue{Path: "/method", Code: "not_found", Message: "unknown or unavailable method"})
	}
	if !spec.Annotations.ReadOnly {
		return nil, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput,
			"method "+in.Method+" has side effects and runs only through execute: r = await tools."+in.Method+"(...)",
			mcp.Issue{Path: "/method", Code: "side_effects", Message: "use execute"})
	}
	return t.engine.Call(ctx, in.Method, in.Input), nil
}
