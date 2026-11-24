// Package code is the built-in node type code: JavaScript or TypeScript an author writes as
// `export default async function run(ctx, env)`, run in the sandbox of pkg/automation/code in the call's
// child instance. ctx is the step's context (event, nodes, the subject's variables, subject, run, tenant,
// now); env has the secrets and connections the node declares, fetch when the node allows the network,
// log and crypto. The code's outcome is one of the node's declared outcomes; any failure is failed with
// output.error.code the sandbox's reason (timeout, exception, network_blocked, unknown_outcome, ...).
//
// A Code node is not idempotent: after an attempt whose outcome is unknown it is not run again.
package code

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	sandbox "github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// Type is the node type.
const Type = "code"

// OutcomeFailed is the node's own outcome of every failure.
const OutcomeFailed = sandbox.OutcomeFailed

// Codes of a Code node's failures besides the sandbox's reasons.
const (
	CodeCompileFailed = string(sandbox.ReasonCompileFailed)
	CodeUnavailable   = "code_unavailable"
)

const paramsSchema = `{"type": "object", "properties": {
	"lang": {"enum": ["ts", "js"], "default": "ts"},
	"source": {"type": "string", "minLength": 1, "maxLength": 100000, "description": "export default async function run(ctx, env) { ... }"},
	"secrets": {"type": "array", "maxItems": 50, "x-unique": true, "default": [],
		"items": {"type": "string", "pattern": "^[A-Z][A-Z0-9_]{0,63}$"}},
	"connections": {"type": "array", "maxItems": 20, "x-unique": true, "default": [],
		"items": {"type": "string", "pattern": "^[a-z][a-z0-9_]{0,39}$"}},
	"fetch": {"type": "boolean", "default": false, "description": "env.fetch reaches https URLs through the network policy"},
	"timeout": {"type": "string", "pattern": "^[1-9][0-9]{0,5}(s|m)$", "x-max-ms": 300000, "default": "30s", "description": "at most 5m"},
	"outcomes": {"type": "array", "minItems": 1, "maxItems": 20, "default": ["done"],
		"items": {"type": "string", "pattern": "^[a-z][a-z0-9_]{0,39}$"}}
}, "required": ["source"], "additionalProperties": false}`

// Deps are what the node runs code with.
type Deps struct {
	// Engine runs the code; Engines is called on first use when Engine is nil (the sandbox is costly to
	// build, so a process that runs no code does not build it). Neither: every run fails code_unavailable.
	Engine  sandbox.Engine
	Engines func(ctx context.Context) (sandbox.Engine, error)
	// Compiler compiles sources (default sandbox.NewCompiler with a cache).
	Compiler sandbox.Compiler
	// Fetcher serves env.fetch and env.connections (default sandbox.NewGuardedFetcher()).
	Fetcher sandbox.Fetcher
	// Secrets and Connections are the tenant's (nil: none).
	Secrets     access.Secrets
	Connections access.Connections
}

// Node is the code node type.
type Node struct {
	d      Deps
	once   sync.Once
	engine sandbox.Engine
	err    error
}

var (
	_ catalog.CallNode      = (*Node)(nil)
	_ catalog.LocalizedNode = (*Node)(nil)
	_ engine.CodeNode       = (*Node)(nil)
	_ engine.NodeValidator  = (*Node)(nil)
)

// New is the code node type.
func New(d Deps) *Node {
	if d.Compiler == nil {
		d.Compiler = sandbox.NewCompiler(sandbox.WithCache(256))
	}
	if d.Fetcher == nil {
		d.Fetcher = sandbox.NewGuardedFetcher()
	}
	return &Node{d: d}
}

func (n *Node) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: Type, Title: "Code", Group: "code", Behavior: graph.BehaviorCall,
		Description: "Runs JavaScript or TypeScript in a sandbox: export default async function run(ctx, env) returns { outcome, output }.",
		Params:      graph.MustSchemaParams(paramsSchema),
		Outcomes:    []string{OutcomeFailed}, OutcomesFrom: &graph.ParamRef{Param: "outcomes"}, FailedOutcome: OutcomeFailed,
		Output: expr.Fields{
			{Name: "output", Field: expr.DynField},
			{Name: "logs", Field: expr.ListOf(expr.StringField)},
		},
		StepContext: true,
		DryRun:      graph.DryRunCapture,
	}
}

func (n *Node) Requires() []catalog.ConnectorKind { return nil }

// Idempotent: a run whose outcome is unknown is not repeated.
func (n *Node) Idempotent() bool { return false }

func (n *Node) Localize(spec graph.NodeSpec, locale string) graph.NodeSpec {
	switch locale {
	case "ru":
		spec.Title, spec.Description = "Код", "Выполняет JavaScript или TypeScript в песочнице: export default async function run(ctx, env) возвращает { outcome, output }."
	case "uz":
		spec.Title, spec.Description = "Kod", "JavaScript yoki TypeScript kodini qum qutisida bajaradi: export default async function run(ctx, env) { outcome, output } qaytaradi."
	case "uz-Cyrl":
		spec.Title, spec.Description = "Код", "JavaScript ёки TypeScript кодини қум қутисида бажаради: export default async function run(ctx, env) { outcome, output } қайтаради."
	}
	return spec
}

// params are a Code node's params.
type params struct {
	Lang        string   `json:"lang"`
	Source      string   `json:"source"`
	Secrets     []string `json:"secrets"`
	Connections []string `json:"connections"`
	Fetch       bool     `json:"fetch"`
	Timeout     string   `json:"timeout"`
	Outcomes    []string `json:"outcomes"`
}

func parse(raw map[string]any) (params, error) {
	var p params
	b, err := json.Marshal(raw)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, err
	}
	if p.Lang == "" {
		p.Lang = string(sandbox.LangTS)
	}
	if len(p.Outcomes) == 0 {
		p.Outcomes = []string{"done"}
	}
	return p, nil
}

func (p params) source() sandbox.Source {
	lang := sandbox.Lang(p.Lang)
	return sandbox.Source{Code: p.Source, Lang: lang, Filename: "code." + string(lang)}
}

// Review is what a human sees before enabling a version with the node.
func (n *Node) Review(raw map[string]any) engine.CodeNodeReview {
	p, _ := parse(raw)
	return engine.CodeNodeReview{Lang: p.Lang, Source: p.Source, Secrets: p.Secrets, Fetch: p.Fetch, Connections: p.Connections}
}

// ValidateParams compiles the source when a graph is saved: a syntax error or an import is
// nodes[i].params.source with its line and column; an outcome named failed is refused.
func (n *Node) ValidateParams(ctx context.Context, raw map[string]any) []graph.GraphError {
	p, err := parse(raw)
	if err != nil || p.Source == "" {
		return nil // the params schema reports it
	}
	var out []graph.GraphError
	for i, o := range p.Outcomes {
		if o == OutcomeFailed {
			out = append(out, graph.GraphError{Path: fmt.Sprintf("params.outcomes[%d]", i), Code: graph.CodeParams,
				Message: `"failed" is the node's own outcome of every failure`})
		}
	}
	if _, err := n.d.Compiler.Compile(ctx, p.source()); err != nil {
		var ce *sandbox.CompileError
		if errors.As(err, &ce) {
			out = append(out, graph.GraphError{Path: "params.source", Code: graph.CodeParams, Message: ce.Message, Line: ce.Line, Column: ce.Column})
		} else {
			out = append(out, graph.GraphError{Path: "params.source", Code: graph.CodeParams, Message: "the source could not be compiled: " + err.Error()})
		}
	}
	return out
}

func (n *Node) engineFor(ctx context.Context) (sandbox.Engine, error) {
	if n.d.Engine != nil {
		return n.d.Engine, nil
	}
	n.once.Do(func() {
		if n.d.Engines == nil {
			n.err = errors.New("this process runs no code")
			return
		}
		n.engine, n.err = n.d.Engines(context.WithoutCancel(ctx))
	})
	return n.engine, n.err
}

// Execute compiles and runs the code once.
func (n *Node) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	p, err := parse(in.Params)
	if err != nil {
		return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "the params of the node are invalid"}
	}
	compiled, err := n.d.Compiler.Compile(ctx, p.source())
	if err != nil {
		var ce *sandbox.CompileError
		if errors.As(err, &ce) {
			return n.failed(CodeCompileFailed, ce.Error(), nil), nil
		}
		return catalog.CallResult{}, err
	}
	secrets, err := access.ReadSecrets(ctx, n.d.Secrets, in, p.Secrets)
	if err != nil {
		var ce *catalog.CallError
		if errors.As(err, &ce) {
			return n.failed(ce.Code, ce.Message, nil), nil
		}
		return catalog.CallResult{}, err
	}
	conns, err := access.OpenAll(ctx, n.d.Connections, in.TenantID, p.Connections)
	if err != nil {
		var ce *catalog.CallError
		if errors.As(err, &ce) {
			return n.failed(ce.Code, ce.Message, nil), nil
		}
		return catalog.CallResult{}, err
	}
	var masked []string
	methods := make(map[string][]string, len(conns))
	for name, c := range conns {
		masked = append(masked, c.Values...)
		methods[name] = c.Access.MethodNames()
	}
	eng, err := n.engineFor(ctx)
	if err != nil {
		return n.failed(CodeUnavailable, "code cannot run in this process", nil), nil
	}
	timeout := sandbox.DefaultTimeout
	if ms, ok := graph.DurationMs(p.Timeout); ok {
		timeout = time.Duration(ms) * time.Millisecond
	}
	if left := time.Until(in.Deadline); !in.Deadline.IsZero() && left < timeout {
		timeout = max(left, time.Millisecond)
	}
	ctxJSON, err := json.Marshal(nonNil(in.Context))
	if err != nil {
		return n.failed(sandbox.ReasonOutputInvalid, "the step's context is not JSON", nil), nil
	}
	run := sandbox.RunInput{
		Ctx: ctxJSON, Secrets: secrets, Outcomes: p.Outcomes, Limits: sandbox.Limits{Timeout: timeout},
		Tenant: in.TenantID.String(), Masked: masked,
	}
	if p.Fetch {
		run.Fetcher = n.d.Fetcher
	}
	if len(conns) > 0 {
		run.Connections = methods
		run.ConnectionCaller = &access.Gateway{Fetcher: n.d.Fetcher, Connections: conns}
	}
	res, err := eng.Run(ctx, compiled, run)
	if err != nil {
		if errors.Is(err, sandbox.ErrBusy) && ctx.Err() == nil {
			// Nothing ran: the engine repeats the call later.
			return catalog.CallResult{RetryAfter: time.Second}, nil
		}
		return catalog.CallResult{}, err
	}
	logs := res.Logs
	if logs == nil {
		logs = []string{}
	}
	if res.Err != nil {
		if res.Err.Reason == sandbox.ReasonOutcomeUnknown {
			// The code ended without a result: what it did before is unknown.
			return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallOutcomeUnknown, Message: res.Err.Message}
		}
		msg := res.Err.Message
		if res.Err.Line > 0 {
			msg = fmt.Sprintf("%s (at %d:%d)", msg, res.Err.Line, res.Err.Column)
		}
		return n.failed(res.Err.Reason, msg, logs), nil
	}
	var output any
	if len(res.Output) > 0 {
		if err := json.Unmarshal(res.Output, &output); err != nil {
			return n.failed(sandbox.ReasonOutputInvalid, "the output is not JSON", logs), nil
		}
	}
	return catalog.CallResult{Outcome: res.Outcome, Output: map[string]any{"output": output, "logs": logs}}, nil
}

// failed is the failed outcome with output.error and the logs the code wrote (already masked).
func (n *Node) failed(code any, message string, logs []string) catalog.CallResult {
	c := fmt.Sprint(code)
	if logs == nil {
		logs = []string{}
	}
	out := graph.FailedOutput(c, message, false)
	out["logs"] = logs
	out["output"] = nil
	return catalog.CallResult{Outcome: OutcomeFailed, Output: out}
}

func nonNil(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}
