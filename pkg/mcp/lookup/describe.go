package lookup

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// DescribeName is the catalog method every application exposes.
const DescribeName = "describe"

const (
	toolSelector  = "tool"
	summaryLength = 200
)

// DescribeConfig lets the application add its own selectors (a report, an
// entity) and catalog next to the operation catalog the SDK builds from the
// registry.
type DescribeConfig struct {
	Policy mcp.Policy
	// Selectors are extra input properties answered by Select, e.g. report or
	// entity. They are exclusive with tool and with each other only if Select
	// says so.
	Selectors map[string]*jsonschema.Schema
	// Select answers a call naming at least one selector; the values are the
	// raw selector arguments.
	Select func(ctx context.Context, selectors map[string]json.RawMessage) (any, error)
	// Catalog adds the application's compact catalog under "domain" in the
	// answer to describe({}).
	Catalog func(ctx context.Context) (any, error)
}

// Operation is one entry of the compact catalog.
type Operation struct {
	Name        string   `json:"name"`
	Title       string   `json:"title,omitempty"`
	Summary     string   `json:"summary"`
	SideEffects bool     `json:"side_effects"`
	Via         []string `json:"via"`
}

type Usage struct {
	Lookup   string `json:"lookup"`
	Execute  string `json:"execute,omitempty"`
	Describe string `json:"describe"`
	Result   string `json:"result"`
}

type Catalog struct {
	Operations []Operation `json:"operations"`
	Usage      Usage       `json:"usage"`
	Domain     any         `json:"domain,omitempty"`
}

// Detail is the full contract of one operation. OutputSchema describes data
// inside the Result envelope.
type Detail struct {
	Name         string             `json:"name"`
	Title        string             `json:"title,omitempty"`
	Description  string             `json:"description"`
	SideEffects  bool               `json:"side_effects"`
	Idempotent   bool               `json:"idempotent"`
	Destructive  bool               `json:"destructive"`
	Via          []string           `json:"via"`
	InputSchema  *jsonschema.Schema `json:"input_schema"`
	OutputSchema *jsonschema.Schema `json:"output_schema,omitempty"`
	Meta         map[string]any     `json:"meta,omitempty"`
}

// Describe returns the describe operation. It is an ordinary read-only
// operation of the registry, so it is reachable through lookup and as
// tools.describe inside execute.
func Describe(engine *mcp.Engine, cfg DescribeConfig) (mcp.Tool, error) {
	const op serrors.Op = "lookup.Describe"
	if engine == nil {
		return nil, serrors.New(serrors.Invalid, "engine is required").WithOp(op)
	}
	if len(cfg.Selectors) > 0 && cfg.Select == nil {
		return nil, serrors.New(serrors.Invalid, "selectors need a Select handler").WithOp(op)
	}
	if _, clash := cfg.Selectors[toolSelector]; clash {
		return nil, serrors.New(serrors.Invalid, "selector \"tool\" is reserved").WithOp(op)
	}
	props := map[string]*jsonschema.Schema{
		toolSelector: {Type: "string", MinLength: jsonschema.Ptr(1),
			Description: "an operation name; returns its description and input and output JSON Schema"},
	}
	maps.Copy(props, cfg.Selectors)
	description := "Without arguments: the compact catalog of operations available to you and how to call them. " +
		"With tool: one operation's description and the JSON Schema of its input and of data in its Result."
	if names := slices.Sorted(maps.Keys(cfg.Selectors)); len(names) > 0 {
		description += " Also accepts " + strings.Join(names, ", ") + " for details of the application's own catalog."
	}
	return &describe{engine: engine, cfg: cfg, spec: mcp.Spec{
		Name:        DescribeName,
		Description: description,
		Input: &jsonschema.Schema{
			Type:                 "object",
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			Properties:           props,
		},
		Annotations: mcp.Annotations{Title: "Describe", ReadOnly: true, Idempotent: true},
		Policy:      cfg.Policy,
	}}, nil
}

type describe struct {
	engine *mcp.Engine
	cfg    DescribeConfig
	spec   mcp.Spec
}

func (d *describe) Spec() mcp.Spec { return d.spec }

func (d *describe) Call(ctx context.Context, raw json.RawMessage) (any, error) {
	var args map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&args); err != nil {
		return nil, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "input does not match the tool schema")
	}
	name, hasTool := args[toolSelector]
	delete(args, toolSelector)
	switch {
	case hasTool && len(args) > 0:
		return nil, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "ask about one tool or the application catalog, not both",
			mcp.Issue{Path: "/" + toolSelector, Code: "exclusive", Message: "tool excludes other selectors"})
	case hasTool:
		var tool string
		if err := json.Unmarshal(name, &tool); err != nil {
			return nil, mcp.Fail(mcp.CodeInvalidInput, mcp.RecoveryFixInput, "tool must be a string")
		}
		return d.detail(ctx, tool)
	case len(args) > 0:
		return d.cfg.Select(ctx, args)
	default:
		return d.catalog(ctx)
	}
}

func (d *describe) executable(ctx context.Context) bool {
	spec, ok := d.engine.Registry().Spec(ExecuteName)
	return ok && spec.EntryPoint && d.engine.Allows(ctx, spec.Policy)
}

func via(spec mcp.Spec, executable bool) []string {
	out := []string{}
	if spec.Annotations.ReadOnly {
		out = append(out, ToolName)
	}
	if executable {
		out = append(out, ExecuteName)
	}
	return out
}

func (d *describe) catalog(ctx context.Context) (*Catalog, error) {
	executable := d.executable(ctx)
	out := &Catalog{
		Operations: []Operation{},
		Usage: Usage{
			Lookup:   `lookup({method: "<read method>", input: {...}}) — one read, no Python; input is required, {} when empty`,
			Describe: `lookup({method: "describe", input: {tool: "<name>"}}) — input and output JSON Schema of one operation`,
			Result:   `every operation returns {ok: true, data} or {ok: false, error: {code, message, issues, recovery}}`,
		},
	}
	if executable {
		out.Usage.Execute = `execute({code: "r = await tools.<name>(...)\nemit(r)"}) — Python composition and every operation with side effects; ` +
			`emit(value) returns data, tools return the same Result as lookup`
	}
	for _, spec := range d.engine.Operations(ctx) {
		ways := via(spec, executable)
		if len(ways) == 0 {
			continue
		}
		out.Operations = append(out.Operations, Operation{
			Name: spec.Name, Title: spec.Annotations.Title, Summary: summary(spec.Description),
			SideEffects: !spec.Annotations.ReadOnly, Via: ways,
		})
	}
	if d.cfg.Catalog != nil {
		domain, err := d.cfg.Catalog(ctx)
		if err != nil {
			return nil, err
		}
		out.Domain = domain
	}
	return out, nil
}

func (d *describe) detail(ctx context.Context, name string) (*Detail, error) {
	spec, ok := d.engine.Operation(ctx, name)
	ways := via(spec, d.executable(ctx))
	if !ok || len(ways) == 0 {
		return nil, mcp.Fail(mcp.CodeNotFound, mcp.RecoveryFixInput,
			"operation "+name+" is not in your catalog; call describe without arguments",
			mcp.Issue{Path: "/" + toolSelector, Code: "not_found", Message: "unknown or unavailable operation"})
	}
	return &Detail{
		Name: spec.Name, Title: spec.Annotations.Title, Description: spec.Description,
		SideEffects: !spec.Annotations.ReadOnly, Idempotent: spec.Annotations.Idempotent,
		Destructive: spec.Annotations.Destructive, Via: ways,
		InputSchema: spec.Input, OutputSchema: spec.Output, Meta: spec.Meta,
	}, nil
}

// summary keeps the first sentence so the catalog stays compact; the full
// description is one describe({tool}) away.
func summary(description string) string {
	s := strings.TrimSpace(description)
	if i := strings.Index(s, ". "); i >= 0 {
		s = s[:i+1]
	}
	if utf8.RuneCountInString(s) <= summaryLength {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:summaryLength-1])) + "…"
}
