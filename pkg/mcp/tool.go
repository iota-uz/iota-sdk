package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Annotations are behavioural hints published to clients; they never replace
// Policy enforcement.
type Annotations struct {
	Title       string
	ReadOnly    bool
	Destructive bool
	Idempotent  bool
	OpenWorld   bool
}

// Policy is enforced on every call, including calls made from execute and
// re-checks when a prepared plan is applied. Both lists must be satisfied.
type Policy struct {
	Scopes      []string
	Permissions []string
}

type Spec struct {
	Name        string
	Description string
	Input       *jsonschema.Schema
	Output      *jsonschema.Schema
	Annotations Annotations
	Policy      Policy
	Meta        map[string]any
	// HostOnly keeps the tool out of the Python tools namespace, e.g. execute
	// itself, so isolated code cannot recurse into it.
	HostOnly bool
}

type Tool interface {
	Spec() Spec
	// Call receives input already validated against Spec().Input.
	Call(ctx context.Context, input json.RawMessage) (any, error)
}

// New builds a typed tool; Input and Output schemas left nil are inferred
// from In and Out so the published schema and the decoder cannot drift.
func New[In, Out any](spec Spec, handler func(context.Context, In) (Out, error)) (Tool, error) {
	const op serrors.Op = "mcp.New"
	if spec.Input == nil {
		s, err := jsonschema.For[In](nil)
		if err != nil {
			return nil, serrors.WrapContext(op, err, "infer input schema of "+spec.Name)
		}
		spec.Input = s
	}
	if spec.Output == nil && reflect.TypeFor[Out]().Kind() != reflect.Interface {
		s, err := jsonschema.For[Out](nil)
		if err != nil {
			return nil, serrors.WrapContext(op, err, "infer output schema of "+spec.Name)
		}
		spec.Output = s
	}
	return &typedTool[In, Out]{spec: spec, handler: handler}, nil
}

func MustNew[In, Out any](spec Spec, handler func(context.Context, In) (Out, error)) Tool {
	t, err := New(spec, handler)
	if err != nil {
		panic(err)
	}
	return t
}

type typedTool[In, Out any] struct {
	spec    Spec
	handler func(context.Context, In) (Out, error)
}

func (t *typedTool[In, Out]) Spec() Spec { return t.spec }

func (t *typedTool[In, Out]) Call(ctx context.Context, input json.RawMessage) (any, error) {
	var in In
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.UseNumber()
	if err := dec.Decode(&in); err != nil {
		return nil, Fail(CodeInvalidInput, RecoveryFixInput, "input does not match the tool schema")
	}
	return t.handler(ctx, in)
}
