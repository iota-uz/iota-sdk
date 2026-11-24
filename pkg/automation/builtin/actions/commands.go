package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// Type is the action node: {"action": key, "args": {...}} dispatches to the node type TypeOf(key).
const Type = "action"

const frontSchema = `{"type": "object", "properties": {
	"action": {"type": "string", "pattern": "^[a-z][a-z0-9]*(\\.[a-z][a-zA-Z0-9_]*)+$", "description": "the key of an action: core.user.block"},
	"args": {"type": "object", "description": "the action's args: a {{ }} template for a string, CEL otherwise"}
}, "required": ["action"], "additionalProperties": false}`

// front is the action node type. A node of it always dispatches to the action it names; only a node
// without a key keeps it, and is refused by its params schema.
type front struct{}

// Front is the action node type (graph.Dispatch to "action." + key).
func Front() catalog.NodeType { return front{} }

func (front) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: Type, Title: "Action", Group: "actions", Behavior: graph.BehaviorCall,
		Description: "Runs an SDK action as the run-as: the action param names it, args are its arguments.",
		Params:      graph.MustSchemaParams(frontSchema), Outcomes: []string{OutcomeFailed}, FailedOutcome: OutcomeFailed,
		Dispatch: &graph.Dispatch{Param: "action", Prefix: TypePrefix},
		DryRun:   graph.DryRunCapture,
	}
}

func (front) Requires() []catalog.ConnectorKind { return nil }
func (front) Idempotent() bool                  { return false }
func (front) Execute(context.Context, catalog.CallInput) (catalog.CallResult, error) {
	return catalog.CallResult{}, &catalog.CallError{Code: catalog.CallInvalidRequest, Message: "an action node names its action"}
}

func (front) Localize(spec graph.NodeSpec, locale string) graph.NodeSpec {
	switch locale {
	case "ru":
		spec.Title, spec.Description = "Действие", "Выполняет действие SDK от имени run-as: параметр action называет его, args — его аргументы."
	case "uz":
		spec.Title, spec.Description = "Amal", "SDK amalini run-as nomidan bajaradi: action parametri uni nomlaydi, args — uning argumentlari."
	case "uz-Cyrl":
		spec.Title, spec.Description = "Амал", "SDK амалини run-as номидан бажаради: action параметри уни номлайди, args — унинг аргументлари."
	}
	return spec
}

// commandOutput is what an action's command returns.
type commandOutput struct {
	Outcome string          `json:"outcome"`
	Output  json.RawMessage `json:"output,omitempty"`
}

// Index is the contributed actions by key; the automations module provides it from the composition.
type Index struct {
	mu sync.RWMutex
	m  map[string]Action
}

// NewIndex indexes actions; a key twice or an action Nodes refuses is an error.
func NewIndex(list ...Action) (*Index, error) {
	if _, err := Nodes(list...); err != nil {
		return nil, err
	}
	m := make(map[string]Action, len(list))
	for _, a := range list {
		if a != nil {
			m[a.Key()] = a
		}
	}
	return &Index{m: m}, nil
}

// Add indexes more actions (the automations module adds the contributed ones when the application
// starts); a key already indexed is an error and nothing is added.
func (i *Index) Add(list ...Action) error {
	if _, err := Nodes(list...); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.m == nil {
		i.m = map[string]Action{}
	}
	for _, a := range list {
		if a == nil {
			continue
		}
		if _, dup := i.m[a.Key()]; dup {
			return fmt.Errorf("automation action %s is registered twice", a.Key())
		}
	}
	for _, a := range list {
		if a != nil {
			i.m[a.Key()] = a
		}
	}
	return nil
}

// Get is the action of a key.
func (i *Index) Get(key string) (Action, bool) {
	if i == nil {
		return nil, false
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	a, ok := i.m[key]
	return a, ok
}

// Keys are the keys of the index, sorted.
func (i *Index) Keys() []string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	out := make([]string, 0, len(i.m))
	for k := range i.m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Run runs the action of a key as subject: the body of its command. A refusal is a *pool.WireError the
// engine maps to the node's failure: denied (forbidden), validation (invalid_args), domain (an
// ActionError's code, or action_failed); an infrastructure error stays one.
func (i *Index) Run(ctx context.Context, key string, subject spi.Subject, args json.RawMessage) (json.RawMessage, error) {
	a, ok := i.Get(key)
	if !ok {
		return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: "automation.action_missing", Message: "no action " + key + " in this process"}
	}
	spec := a.Spec()
	if !subject.Has(spec.Permission) {
		return nil, &pool.WireError{Kind: pool.ErrKindDenied, Code: "automation.forbidden", Message: "permission " + spec.Permission + " is required"}
	}
	if spec.Args != nil {
		if err := manifest.ValidateValue(*spec.Args, args); err != nil {
			return nil, &pool.WireError{Kind: pool.ErrKindValidation, Code: "automation." + CodeInvalidArgs, Message: err.Error()}
		}
	}
	outcome, out, err := a.Run(ctx, subject, args)
	if err != nil {
		var ae *ActionError
		switch {
		case errors.Is(err, ErrForbidden):
			return nil, &pool.WireError{Kind: pool.ErrKindDenied, Code: "automation.forbidden", Message: "the run-as may not " + key}
		case errors.Is(err, ErrInvalidArgs):
			return nil, &pool.WireError{Kind: pool.ErrKindValidation, Code: "automation." + CodeInvalidArgs, Message: err.Error()}
		case errors.As(err, &ae):
			return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: "automation." + ae.Code, Message: ae.Message}
		case ctx.Err() != nil, host.IsInfrastructure(err):
			return nil, err
		}
		return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: "automation." + CodeActionFailed, Message: err.Error()}
	}
	if outcome == "" {
		outcome = OutcomeDone
		if len(spec.Outcomes) > 0 {
			outcome = spec.Outcomes[0]
		}
	}
	if len(out) > 0 && string(out) != "null" && spec.Output != nil {
		if err := manifest.ValidateValue(*spec.Output, out); err != nil {
			return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: "automation." + CodeInvalidOutput, Message: err.Error()}
		}
	}
	return json.Marshal(commandOutput{Outcome: outcome, Output: out})
}

// Commands are the native command contracts of actions, for the module's native.Set: the ID is the key,
// the permission the action's, the input its args and the output {outcome, output}. The implementation
// runs the contributed action of the key (the Index the composition provides) in a transaction of the
// host, as the invocation's subject. templates give the specs; their dependencies are not used.
func Commands(templates ...Action) ([]native.Item, error) {
	out := make([]native.Item, 0, len(templates))
	for _, a := range templates {
		key, spec := a.Key(), a.Spec()
		if _, err := newNode(a); err != nil {
			return nil, err
		}
		input := &manifest.Type{Kind: manifest.KindObject}
		if spec.Args != nil {
			input = spec.Args
		}
		outputProps := []manifest.Property{{Name: "outcome", Type: manifest.Type{Kind: manifest.KindString}}}
		if spec.Output != nil {
			o := *spec.Output
			o.Optional = true
			outputProps = append(outputProps, manifest.Property{Name: "output", Type: o})
		}
		def, err := native.NewCommand[json.RawMessage, json.RawMessage](key, native.Spec{
			Permission: spec.Permission, Label: spec.Label, Description: label(spec.Description, "en", ""),
			Input: input, Output: &manifest.Type{Kind: manifest.KindObject, Properties: outputProps},
		}, func(d native.Deps) (native.CommandFunc[json.RawMessage, json.RawMessage], error) {
			idx, err := native.Use[*Index](d)
			if err != nil {
				return nil, err
			}
			return func(ctx context.Context, c native.CommandContext, in json.RawMessage) (json.RawMessage, error) {
				var out json.RawMessage
				err := c.Transaction(ctx, func(ctx context.Context, tx native.Tx) error {
					var runErr error
					out, runErr = idx.Run(ctx, key, tx.Subject(), in)
					return runErr
				})
				return out, err
			}, nil
		})
		if err != nil {
			return nil, fmt.Errorf("automation action %s: %w", key, err)
		}
		out = append(out, def)
	}
	return out, nil
}

// Local is catalog.Commands that runs the index's actions in process as subject, with the refusals
// mapped as the engine maps the command host's: a test of a node, a process without a command host.
func (i *Index) Local(subject spi.Subject) catalog.Commands {
	return localCommands{i: i, subject: subject}
}

type localCommands struct {
	i       *Index
	subject spi.Subject
}

func (l localCommands) Invoke(ctx context.Context, command string, input json.RawMessage) (json.RawMessage, error) {
	out, err := l.i.Run(ctx, command, l.subject, input)
	if err == nil {
		return out, nil
	}
	var we *pool.WireError
	if !errors.As(err, &we) {
		return nil, err
	}
	code := we.Code
	if j := strings.LastIndex(code, "."); j >= 0 {
		code = code[j+1:]
	}
	if we.Kind == pool.ErrKindDenied {
		code = CodeForbidden
	}
	return nil, &catalog.CallError{Code: code, Message: we.Message}
}
