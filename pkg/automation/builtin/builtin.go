// Package builtin registers the built-in node types every tenant's catalog has without a connector:
// set (values by CEL), set_state (the automation's memory between runs), code (JavaScript/TypeScript in the sandbox), http (an HTTPS request with SSRF
// protection and secrets), start_automation and run_automation (another graph, without or with waiting for
// its result), and the actions modules contribute (package actions).
//
// The automations module registers them in its catalog.Registry when it is composed (Register); an
// application that builds its own registry calls Register itself.
package builtin

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/access"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/actions"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/flow"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/http"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/set"
	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/setstate"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	sandbox "github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Deps are what the built-in nodes run with.
type Deps struct {
	// Engine runs Code nodes; Engines builds it on the first Code node a process runs (the sandbox is
	// costly to build). Neither: Code nodes fail with code_unavailable.
	Engine  sandbox.Engine
	Engines func(ctx context.Context) (sandbox.Engine, error)
	// Compiler compiles Code nodes (default sandbox.NewCompiler with a cache).
	Compiler sandbox.Compiler
	// Fetcher is the network of Code and HTTP nodes (default sandbox.NewGuardedFetcher).
	Fetcher sandbox.Fetcher
	// Secrets and Connections are the tenant's secrets and connections (nil: none).
	Secrets     access.Secrets
	Connections access.Connections
	// ConnectionUse is the permission a run-as subject needs to use connections from Code and HTTP nodes
	// ("": none).
	ConnectionUse string
	// Runs starts the runs of start_automation and run_automation (the engine; resolved on first use).
	Runs flow.Starter
	// Actions are the SDK actions to register with the nodes.
	Actions []actions.Action
}

// Nodes are the built-in node types.
func Nodes(d Deps) ([]catalog.NodeType, error) {
	if d.Compiler == nil {
		d.Compiler = sandbox.NewCompiler(sandbox.WithCache(256))
	}
	if d.Fetcher == nil {
		d.Fetcher = sandbox.NewGuardedFetcher(sandbox.WithMaxBody(http.MaxResponseBytes))
	}
	start, run := flow.New(d.Runs)
	out := []catalog.NodeType{
		set.New(), setstate.New(),
		code.New(code.Deps{Engine: d.Engine, Engines: d.Engines, Compiler: d.Compiler, Fetcher: d.Fetcher, Secrets: d.Secrets, Connections: d.Connections, ConnectionUse: d.ConnectionUse}),
		http.New(http.Deps{Fetcher: d.Fetcher, Secrets: d.Secrets, Connections: d.Connections, ConnectionUse: d.ConnectionUse}),
		start, run, actions.Front(),
	}
	acts, err := actions.Nodes(d.Actions...)
	if err != nil {
		return nil, err
	}
	return append(out, acts...), nil
}

// Register registers the built-in node types in the registry; a type registered twice is an error.
func Register(reg catalog.Registry, d Deps) error {
	const op = serrors.Op("automation.builtin.Register")
	nodes, err := Nodes(d)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	for _, n := range nodes {
		if err := reg.RegisterNode(n); err != nil {
			return serrors.Wrap(op, err)
		}
	}
	return nil
}
