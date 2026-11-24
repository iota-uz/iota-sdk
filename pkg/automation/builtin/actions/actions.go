// Package actions is the registry of SDK actions: commands a module or an application offers to
// automation graphs ("create a task", "change a status"), each a node type of the catalog without a change
// of the core. An action runs as a call node in the call's child instance, as the run-as subject: its
// Permission is checked against the run-as on every call (an author's right is never carried to the run)
// and an author without it does not see the action in the palette.
//
// A module contributes its actions at composition (Contribute, ContributeFunc); the automations module
// registers them in the catalog when the application starts, and a key registered twice fails the start.
//
// The node type of an action is "action." + its key. Its params are {args: {<name>: cel|tpl}}: an argument
// whose type is a string (string, uuid, date, datetime, int64, decimal, enum) is a {{ }} template, any
// other a CEL expression whose value is the argument. Args are validated against Spec().Args before Run.
package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// Action is a command a module offers to automation graphs.
type Action interface {
	Key() string // "core.user.block", "crm.client.assign": the native grammar of keys
	Spec() ActionSpec
	// Run executes inside the call's child instance; actor is the run-as subject with its current rights.
	// The invocation ID of the call (stable across its attempts) is InvocationID(ctx).
	Run(ctx context.Context, actor spi.Subject, args json.RawMessage) (outcome string, output json.RawMessage, err error)
}

// ActionSpec describes an action.
type ActionSpec struct {
	Label, Description manifest.Label // en/ru/uz/uz-Cyrl
	Group              string         // palette group
	Args               *manifest.Type // an object; validated before Run
	Output             *manifest.Type // an object
	Outcomes           []string       // default ["done"]; failed is every action's
	Permission         string         // checked against the run-as on every call; empty is not allowed
	Idempotent         bool           // the action dedupes by InvocationID itself
	Subjects           []string       // subject entities it applies to; empty = any
}

// Errors of an action's Run.
var (
	// ErrForbidden: the actor may not do it → failed, code forbidden, and nothing was done.
	ErrForbidden = errors.New("automation action: forbidden")
	// ErrInvalidArgs: the args are refused → failed, code invalid_args.
	ErrInvalidArgs = errors.New("automation action: invalid args")
)

// ActionError is a refusal with a code → failed with that code (an identifier: a-z, 0-9, _ . -).
type ActionError struct {
	Code    string
	Message string
}

func (e *ActionError) Error() string { return e.Code + ": " + e.Message }

// Codes of an action's failures.
const (
	CodeForbidden     = "forbidden"
	CodeInvalidArgs   = "invalid_args"
	CodeInvalidOutput = "invalid_output"
	CodeActionFailed  = "action_failed"
	OutcomeFailed     = "failed"
	OutcomeDone       = "done"
)

// TypePrefix starts the node type of an action.
const TypePrefix = "action."

// TypeOf is the node type of an action key.
func TypeOf(key string) string { return TypePrefix + key }

type invocationKey struct{}

// WithInvocation is ctx with the invocation ID of the call.
func WithInvocation(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, invocationKey{}, id)
}

// InvocationID is the invocation ID of the call an action runs in ("" outside of one): the same on every
// attempt, so an idempotent action dedupes by it.
func InvocationID(ctx context.Context) string {
	id, _ := ctx.Value(invocationKey{}).(string)
	return id
}

// Contribute adds actions to the application's composition.
func Contribute(b *composition.Builder, a ...Action) {
	list := append([]Action(nil), a...)
	composition.Contribute(b, func(*composition.Container) ([]Action, error) { return list, nil })
}

// ContributeFunc adds the actions a factory builds from the container (the services they call).
func ContributeFunc(b *composition.Builder, f func(*composition.Container) ([]Action, error)) {
	composition.Contribute(b, f)
}

// Contributed lists the contributed actions of a compiled container.
func Contributed(c *composition.Container) ([]Action, error) {
	return composition.Contributions[Action](c)
}

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-zA-Z0-9_]*)+$`)

// Nodes are the node types of actions, checked: a key in the native grammar, each once, a permission, an
// object of args and of output whose fields are field names, outcomes that are ids.
func Nodes(list ...Action) ([]catalog.NodeType, error) {
	seen := map[string]bool{}
	out := make([]catalog.NodeType, 0, len(list))
	for _, a := range list {
		if a == nil {
			continue
		}
		n, err := newNode(a)
		if err != nil {
			return nil, err
		}
		if seen[a.Key()] {
			return nil, fmt.Errorf("automation action %s is registered twice", a.Key())
		}
		seen[a.Key()] = true
		out = append(out, n)
	}
	return out, nil
}

// Register registers the node types of actions in the catalog registry.
func Register(reg catalog.Registry, list ...Action) error {
	nodes, err := Nodes(list...)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		if err := reg.RegisterNode(n); err != nil {
			return err
		}
	}
	return nil
}

// node is the node type of one action.
type node struct {
	a        Action
	spec     ActionSpec
	outcomes []string
	output   expr.Fields
	argKinds map[string]bool // arg name → a template (a string kind)
	params   graph.ParamsSpec
}

var (
	_ catalog.CallNode         = (*node)(nil)
	_ catalog.PermissionedNode = (*node)(nil)
	_ catalog.LocalizedNode    = (*node)(nil)
)

func newNode(a Action) (*node, error) {
	key := a.Key()
	if !keyPattern.MatchString(key) || len(key) > 100 {
		return nil, fmt.Errorf("automation action %q: a key is dotted lower camel segments, as core.user.block", key)
	}
	spec := a.Spec()
	if spec.Permission == "" {
		return nil, fmt.Errorf("automation action %s: an action names the permission it needs", key)
	}
	n := &node{a: a, spec: spec, argKinds: map[string]bool{}}
	outcomes := spec.Outcomes
	if len(outcomes) == 0 {
		outcomes = []string{OutcomeDone}
	}
	for _, o := range outcomes {
		if !graph.IsID(o) || o == OutcomeFailed {
			return nil, fmt.Errorf("automation action %s: outcome %q is an id other than failed", key, o)
		}
	}
	n.outcomes = append(slices.Clone(outcomes), OutcomeFailed)
	props := map[string]any{}
	var required []string
	if spec.Args != nil {
		if spec.Args.Kind != manifest.KindObject {
			return nil, fmt.Errorf("automation action %s: args are an object", key)
		}
		for _, p := range spec.Args.Properties {
			if !expr.IsName(p.Name) {
				return nil, fmt.Errorf("automation action %s: arg %q is not a field name", key, p.Name)
			}
			tpl := stringKind(p.Type.Kind)
			n.argKinds[p.Name] = tpl
			desc := "CEL"
			if tpl {
				desc = "{{ }} template"
			}
			props[p.Name] = map[string]any{"type": "string", "minLength": 1, "maxLength": 8000, "description": desc}
			if !p.Type.Optional {
				required = append(required, p.Name)
			}
		}
	}
	args := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		sort.Strings(required)
		args["required"] = required
	}
	schema := map[string]any{"type": "object", "properties": map[string]any{"args": args}, "additionalProperties": false}
	if len(required) > 0 {
		schema["required"] = []string{"args"}
	} else {
		args["default"] = map[string]any{}
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	if n.params, err = graph.SchemaParams(raw); err != nil {
		return nil, fmt.Errorf("automation action %s: %w", key, err)
	}
	if spec.Output != nil {
		if spec.Output.Kind != manifest.KindObject {
			return nil, fmt.Errorf("automation action %s: output is an object", key)
		}
		f, err := catalog.FieldFromManifest(manifest.Type{Kind: manifest.KindObject, Properties: spec.Output.Properties})
		if err != nil {
			return nil, fmt.Errorf("automation action %s: output: %w", key, err)
		}
		n.output = expr.Fields(f.Props)
	}
	return n, nil
}

// stringKind reports a type whose wire value is a string: its argument is a template.
func stringKind(k manifest.TypeKind) bool {
	switch k { //nolint:exhaustive // every other kind is a CEL value
	case manifest.KindString, manifest.KindUUID, manifest.KindDate, manifest.KindDateTime, manifest.KindInt64,
		manifest.KindDecimal, manifest.KindEnum:
		return true
	}
	return false
}

func (n *node) Spec() graph.NodeSpec {
	var templates, values []string
	for _, name := range sortedNames(n.argKinds) {
		if n.argKinds[name] {
			templates = append(templates, "args."+name)
		} else {
			values = append(values, "args."+name)
		}
	}
	return graph.NodeSpec{
		Type: TypeOf(n.a.Key()), Title: label(n.spec.Label, "en", n.a.Key()), Description: label(n.spec.Description, "en", ""),
		Group: n.spec.Group, Behavior: graph.BehaviorCall, Params: n.params, Subjects: slices.Clone(n.spec.Subjects),
		Outcomes: slices.Clone(n.outcomes), FailedOutcome: OutcomeFailed,
		Templates: templates, Values: values, Output: n.output.Clone(),
		DryRun: graph.DryRunCapture,
	}
}

func (n *node) Requires() []catalog.ConnectorKind { return nil }
func (n *node) Permission() string                { return n.spec.Permission }
func (n *node) Idempotent() bool                  { return n.spec.Idempotent }

func (n *node) Localize(spec graph.NodeSpec, locale string) graph.NodeSpec {
	spec.Title = label(n.spec.Label, locale, spec.Title)
	spec.Description = label(n.spec.Description, locale, spec.Description)
	return spec
}

func label(l manifest.Label, locale, fallback string) string {
	if v := l[locale]; v != "" {
		return v
	}
	if v := l["en"]; v != "" {
		return v
	}
	return fallback
}

// Execute validates the args and runs the action as the run-as. The engine checked Permission already
// (PermissionedNode); it is checked again here, so no path runs an action for a subject without it.
func (n *node) Execute(ctx context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	if !in.RunAs.Has(n.spec.Permission) {
		return catalog.CallResult{}, &catalog.CallError{Code: CodeForbidden, Message: "the run-as may not " + n.a.Key()}
	}
	args, _ := in.Params["args"].(map[string]any)
	if args == nil {
		args = map[string]any{}
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return catalog.CallResult{}, &catalog.CallError{Code: CodeInvalidArgs, Message: "the args are not JSON"}
	}
	if n.spec.Args != nil {
		if err := manifest.ValidateValue(*n.spec.Args, raw); err != nil {
			return catalog.CallResult{}, &catalog.CallError{Code: CodeInvalidArgs, Message: err.Error()}
		}
	}
	ctx = WithInvocation(ctx, catalog.InvocationIDOf(in.IdempotencyKey))
	outcome, out, err := n.a.Run(ctx, in.RunAs, raw)
	if err != nil {
		var ae *ActionError
		switch {
		case errors.Is(err, ErrForbidden):
			return catalog.CallResult{}, &catalog.CallError{Code: CodeForbidden, Message: "the run-as may not " + n.a.Key()}
		case errors.Is(err, ErrInvalidArgs):
			return catalog.CallResult{}, &catalog.CallError{Code: CodeInvalidArgs, Message: err.Error()}
		case errors.As(err, &ae):
			return catalog.CallResult{}, &catalog.CallError{Code: ae.Code, Message: ae.Message}
		case ctx.Err() != nil, host.IsInfrastructure(err):
			return catalog.CallResult{}, err
		}
		return catalog.CallResult{}, &catalog.CallError{Code: CodeActionFailed, Message: err.Error()}
	}
	if outcome == "" {
		outcome = n.outcomes[0]
	}
	if !slices.Contains(n.outcomes, outcome) || outcome == OutcomeFailed {
		return catalog.CallResult{}, &catalog.CallError{Code: graph.FailUnknownOutcome, Message: fmt.Sprintf("%s returned outcome %q", n.a.Key(), outcome)}
	}
	output := map[string]any{}
	if len(out) > 0 && string(out) != "null" {
		if n.spec.Output != nil {
			if err := manifest.ValidateValue(*n.spec.Output, out); err != nil {
				return catalog.CallResult{}, &catalog.CallError{Code: CodeInvalidOutput, Message: err.Error()}
			}
		}
		if err := json.Unmarshal(out, &output); err != nil {
			return catalog.CallResult{}, &catalog.CallError{Code: CodeInvalidOutput, Message: "the output is not a JSON object"}
		}
	}
	return catalog.CallResult{Outcome: outcome, Output: output}, nil
}

func sortedNames(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
