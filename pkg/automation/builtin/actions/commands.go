package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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

// ErrorPrefix starts the code of an action command's domain error; the call's output.error.code is the
// code after it, dots and all.
const ErrorPrefix = "automation."

// templates are the actions by their command definition (Native): the very contract a set holds, so a
// set's action is never another set's command of the same ID, nor an ordinary command.
var (
	templatesMu sync.RWMutex
	templates   = map[native.Definition]Action{}
)

// TemplateOf is the action a command definition of a set is (Native made it), if it is one.
func TemplateOf(d native.Definition) (Action, bool) {
	templatesMu.RLock()
	defer templatesMu.RUnlock()
	a, ok := templates[d]
	return a, ok
}

// Native is the one registration of an action: its command contract for the module's native.Set. The ID
// is the key, the permission the action's, the input its args and the output {outcome, output}; the
// implementation is the action build returns from the application's dependencies (the services it
// calls), run in a transaction of the host as the invocation's subject. template gives the key and the
// spec; its dependencies are not used. The automations module makes the node type of every action command
// of the application's set (NodesOf).
func Native(template Action, build func(native.Deps) (Action, error)) (native.Item, error) {
	key, spec := template.Key(), template.Spec()
	if _, err := newNode(template); err != nil {
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
	declared := []string{ErrorPrefix + CodeForbidden, ErrorPrefix + CodeActionFailed, ErrorPrefix + CodeInvalidOutput}
	for _, c := range spec.Errors {
		declared = append(declared, ErrorPrefix+c)
	}
	def, err := native.NewCommand[json.RawMessage, json.RawMessage](key, native.Spec{
		Permission: spec.Permission, Label: spec.Label, Description: label(spec.Description, "en", ""), Errors: declared,
		Input: input, Output: &manifest.Type{Kind: manifest.KindObject, Properties: outputProps},
	}, func(d native.Deps) (native.CommandFunc[json.RawMessage, json.RawMessage], error) {
		a, err := build(d)
		if err != nil {
			return nil, err
		}
		if a == nil || a.Key() != key {
			return nil, fmt.Errorf("automation action %s: build returned another action", key)
		}
		return func(ctx context.Context, c native.CommandContext, in json.RawMessage) (json.RawMessage, error) {
			var out json.RawMessage
			err := c.Transaction(ctx, func(ctx context.Context, tx native.Tx) error {
				var runErr error
				out, runErr = Run(ctx, a, tx.Subject(), in)
				return runErr
			})
			return out, err
		}, nil
	})
	if err != nil {
		return nil, fmt.Errorf("automation action %s: %w", key, err)
	}
	templatesMu.Lock()
	templates[native.Definition(def)] = template
	templatesMu.Unlock()
	return def, nil
}

// MustNative is Native for actions declared in Go: it panics on an invalid action.
func MustNative(template Action, build func(native.Deps) (Action, error)) native.Item {
	it, err := Native(template, build)
	if err != nil {
		panic(err)
	}
	return it
}

// NodesOf are the node types of the action commands of a native set (Native).
func NodesOf(set *native.Set) ([]catalog.NodeType, error) {
	if set == nil {
		return nil, nil
	}
	var list []Action
	for _, d := range set.Contracts() {
		if a, ok := TemplateOf(d); ok {
			list = append(list, a)
		}
	}
	return Nodes(list...)
}

// Run runs an action as subject: the body of its command. A refusal is a *pool.WireError the engine maps
// to the node's failure: denied (forbidden), validation (invalid_args), domain (an ActionError's code, or
// action_failed); an infrastructure error stays one.
func Run(ctx context.Context, a Action, subject spi.Subject, args json.RawMessage) (json.RawMessage, error) {
	key, spec := a.Key(), a.Spec()
	if !subject.Has(spec.Permission) {
		return nil, &pool.WireError{Kind: pool.ErrKindDenied, Code: ErrorPrefix + CodeForbidden, Message: "permission " + spec.Permission + " is required"}
	}
	if spec.Args != nil {
		if err := manifest.ValidateValue(*spec.Args, args); err != nil {
			return nil, &pool.WireError{Kind: pool.ErrKindValidation, Code: ErrorPrefix + CodeInvalidArgs, Message: err.Error()}
		}
	}
	outcome, out, err := a.Run(ctx, subject, args)
	if err != nil {
		var ae *ActionError
		switch {
		case errors.Is(err, ErrForbidden):
			// A refusal of the action itself (the service's own check): a domain failure the graph handles.
			return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: ErrorPrefix + CodeForbidden, Message: "the run-as may not " + key}
		case errors.Is(err, ErrInvalidArgs):
			return nil, &pool.WireError{Kind: pool.ErrKindValidation, Code: ErrorPrefix + CodeInvalidArgs, Message: err.Error()}
		case errors.As(err, &ae) && slices.Contains(spec.Errors, ae.Code):
			return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: ErrorPrefix + ae.Code, Message: nulFree(ae.Message)}
		case errors.As(err, &ae):
			return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: ErrorPrefix + CodeActionFailed, Message: nulFree(ae.Code + ": " + ae.Message)}
		case ctx.Err() != nil, host.IsInfrastructure(err):
			return nil, err
		}
		return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: ErrorPrefix + CodeActionFailed, Message: nulFree(err.Error())}
	}
	if outcome == "" {
		outcome = OutcomeDone
		if len(spec.Outcomes) > 0 {
			outcome = spec.Outcomes[0]
		}
	}
	if len(out) > 0 && string(out) != "null" && spec.Output != nil {
		if err := manifest.ValidateValue(*spec.Output, out); err != nil {
			return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: ErrorPrefix + CodeInvalidOutput, Message: err.Error()}
		}
	}
	// The host and the workflow record the output in jsonb, which refuses U+0000: a NUL byte of the
	// action's text becomes U+FFFD before anything records it.
	raw, err := json.Marshal(commandOutput{Outcome: outcome, Output: catalog.WithoutNUL(out)})
	if err != nil {
		return nil, err
	}
	return catalog.WithoutNUL(raw), nil
}

// nulFree is text without U+0000, which the records of the host refuse.
func nulFree(s string) string { return strings.ReplaceAll(s, "\x00", "\uFFFD") }
