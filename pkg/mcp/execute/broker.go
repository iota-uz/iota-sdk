package execute

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/pykernel"
)

const emitName = "emit"

// runState collects what a run hands back explicitly: emitted values and the
// operations its tool calls produced. Payloads of nested calls stay out.
type runState struct {
	mu           sync.Mutex
	maxEmit      int
	emitBytes    int
	emitted      []json.RawMessage
	truncated    bool
	operationIDs []string
}

func (s *runState) emit(v json.RawMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.emitBytes+len(v) > s.maxEmit {
		s.truncated = true
		return
	}
	s.emitBytes += len(v)
	s.emitted = append(s.emitted, v)
}

func (s *runState) operation(id string) {
	if id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !slices.Contains(s.operationIDs, id) {
		s.operationIDs = append(s.operationIDs, id)
	}
}

func (s *runState) snapshot() ([]json.RawMessage, bool, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	emitted := append([]json.RawMessage{}, s.emitted...)
	ops := append([]string{}, s.operationIDs...)
	return emitted, s.truncated, ops
}

type runKey struct{}

func withRun(ctx context.Context, s *runState) context.Context {
	return context.WithValue(ctx, runKey{}, s)
}

func runFrom(ctx context.Context) (*runState, bool) {
	s, ok := ctx.Value(runKey{}).(*runState)
	return s, ok
}

// capabilities binds every non-host-only registry tool into the Python
// tools namespace. The kernel sends only a name and arguments; identity
// comes from the exec context the host passed to Exec.
func (r *Runner) capabilities() (pykernel.CapabilitySet, error) {
	specs := r.cfg.Engine.Registry().Specs()
	caps := make([]pykernel.Capability, 0, len(specs)+1)
	for _, spec := range specs {
		if spec.HostOnly {
			continue
		}
		access := pykernel.AccessWrite
		if spec.Annotations.ReadOnly {
			access = pykernel.AccessRead
		}
		caps = append(caps, pykernel.CapabilityFunc(spec.Name, access, signature(spec), r.invoker(spec.Name)))
	}
	caps = append(caps, pykernel.CapabilityFunc(emitName, pykernel.AccessRead, pykernel.CapabilitySignature{
		Params: []pykernel.ParamSpec{{Name: "value", Required: true}},
		Doc:    "Return a JSON-serializable value to the caller as structured data.",
	}, emit))
	return pykernel.NewCapabilitySet(caps...)
}

func (r *Runner) invoker(name string) func(context.Context, pykernel.CallArgs) (any, error) {
	return func(ctx context.Context, args pykernel.CallArgs) (any, error) {
		if args == nil {
			args = pykernel.CallArgs{}
		}
		raw, err := json.Marshal(args)
		if err != nil {
			return nil, err
		}
		res := r.cfg.Engine.Call(mcp.WithNestedCaller(ctx), name, raw)
		if s, ok := runFrom(ctx); ok {
			s.operation(mcp.OperationIDOf(res))
		}
		return res, nil
	}
}

var errOutsideRun = errors.New("emit is only available inside execute")

func emit(ctx context.Context, args pykernel.CallArgs) (any, error) {
	s, ok := runFrom(ctx)
	if !ok {
		return nil, errOutsideRun
	}
	raw, err := json.Marshal(args["value"])
	if err != nil {
		return nil, err
	}
	s.emit(raw)
	return json.RawMessage("null"), nil
}

var pythonTypes = map[string]string{
	"string": "str", "integer": "int", "number": "float", "boolean": "bool", "object": "dict", "array": "list",
}

func signature(spec mcp.Spec) pykernel.CapabilitySignature {
	names := make([]string, 0, len(spec.Input.Properties))
	for name := range spec.Input.Properties {
		names = append(names, name)
	}
	slices.Sort(names)
	params := make([]pykernel.ParamSpec, 0, len(names))
	for _, name := range names {
		params = append(params, pykernel.ParamSpec{
			Name:     name,
			Type:     pythonTypes[spec.Input.Properties[name].Type],
			Required: slices.Contains(spec.Input.Required, name),
		})
	}
	return pykernel.CapabilitySignature{
		Params:    params,
		Returns:   "dict",
		Doc:       doc(spec),
		Namespace: "tools",
		Async:     true,
	}
}

func doc(spec mcp.Spec) string {
	var b strings.Builder
	b.WriteString(spec.Description)
	b.WriteString("\n\nReturns {\"ok\": true, \"data\": ...} or {\"ok\": false, \"error\": {...}}.\nInput schema: ")
	b.WriteString(schemaJSON(spec.Input))
	return b.String()
}

func schemaJSON(s *jsonschema.Schema) string {
	raw, err := json.Marshal(s)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
