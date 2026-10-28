package mcp

import (
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Tool names double as Python attributes in the execute runtime.
var toolName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)

// Registry is the single source of tool contracts for every adapter: MCP
// schemas, chat tool definitions and Python bindings are all derived from it.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]*entry
}

type entry struct {
	tool  Tool
	spec  Spec
	input *compiledSchema
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]*entry)}
}

func (r *Registry) Register(tools ...Tool) error {
	const op serrors.Op = "mcp.Registry.Register"
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range tools {
		if t == nil {
			return serrors.New(serrors.Invalid, "nil tool").WithOp(op)
		}
		spec := t.Spec()
		if !toolName.MatchString(spec.Name) {
			return serrors.New(serrors.Invalid, "invalid tool name "+spec.Name).WithOp(op)
		}
		if _, dup := r.tools[spec.Name]; dup {
			return serrors.New(serrors.AlreadyExists, "tool "+spec.Name+" is already registered").WithOp(op)
		}
		if spec.Input == nil || spec.Input.Type != "object" {
			return serrors.New(serrors.Invalid, "tool "+spec.Name+" input schema must be an object").WithOp(op)
		}
		input, err := compileSchema(spec.Input)
		if err != nil {
			return serrors.WrapContext(op, err, "compile input schema of "+spec.Name)
		}
		r.tools[spec.Name] = &entry{tool: t, spec: spec, input: input}
	}
	return nil
}

// Specs returns the registered contracts ordered by name.
func (r *Registry) Specs() []Spec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	specs := make([]Spec, 0, len(r.tools))
	for _, e := range r.tools {
		specs = append(specs, e.spec)
	}
	slices.SortFunc(specs, func(a, b Spec) int { return strings.Compare(a.Name, b.Name) })
	return specs
}

func (r *Registry) lookup(name string) (*entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.tools[name]
	return e, ok
}
