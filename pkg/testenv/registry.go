package testenv

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type scenario struct {
	definition    Definition
	input, output *jsonschema.Schema
	prepare       PrepareFunc
	cleanup       CleanupFunc
}
type scope struct {
	mu       chan struct{}
	request  string
	result   *Result
	scenario *scenario
	disposed atomic.Bool
}

func newScope() *scope { return &scope{mu: make(chan struct{}, 1)} }
func (s *scope) lock(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case s.mu <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (s *scope) unlock() { <-s.mu }

type Registry struct {
	mu             sync.Mutex
	scenarios      map[string]*scenario
	scopes         map[string]*scope
	capabilities   map[string]bool
	dedicated      bool
	environmentID  string
	active         map[string]bool
	exclusiveScope string
}

func NewEnvironmentRegistry(capabilities []string, dedicated bool, environmentID string) *Registry {
	r := NewRegistry(capabilities, dedicated)
	r.environmentID = environmentID
	return r
}

func NewRegistry(capabilities []string, dedicated bool) *Registry {
	r := &Registry{scenarios: map[string]*scenario{}, scopes: map[string]*scope{}, capabilities: map[string]bool{}, dedicated: dedicated}
	r.active = map[string]bool{}
	for _, c := range capabilities {
		r.capabilities[c] = true
	}
	return r
}
func compileSchema(value any) (*jsonschema.Schema, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var document any
	if err := json.Unmarshal(encoded, &document); err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	if err := c.AddResource("urn:testenv:schema", document); err != nil {
		return nil, err
	}
	return c.Compile("urn:testenv:schema")
}
func (r *Registry) Register(d Definition, prepare PrepareFunc, cleanup CleanupFunc) error {
	encoded, err := json.Marshal(d)
	if err != nil {
		return failure("invalid_input", err.Error())
	}
	if err := json.Unmarshal(encoded, &d); err != nil {
		return failure("invalid_input", err.Error())
	}
	if d.Name == "" || d.Version == "" || prepare == nil || cleanup == nil || (d.Isolation != "shared" && d.Isolation != "dedicated") {
		return failure("invalid_input", "invalid scenario definition")
	}
	in, err := compileSchema(d.InputSchema)
	if err != nil {
		return fmt.Errorf("input schema: %w", err)
	}
	out, err := compileSchema(d.OutputSchema)
	if err != nil {
		return fmt.Errorf("output schema: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.scenarios[d.Name]; exists {
		return failure("scope_conflict", "scenario already registered")
	}
	r.scenarios[d.Name] = &scenario{d, in, out, prepare, cleanup}
	return nil
}

// AllowScope is called by the environment owner, never by the HTTP client.
func (r *Registry) AllowScope(id string) error {
	if id == "" {
		return failure("invalid_input", "empty scope")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.scopes[id]; exists {
		return failure("scope_conflict", "scope already exists")
	}
	r.scopes[id] = newScope()
	return nil
}

// ReserveScope is available only to an environment-bound registry. The control
// client may allocate a physical scope beneath that environment's namespace.
func (r *Registry) ReserveScope(id string) error {
	if r.environmentID == "" || !strings.HasPrefix(id, r.environmentID+"-") || len(id) > 256 || strings.ContainsAny(id, "/\\\x00") {
		return failure("scope_conflict", "scope is outside this environment namespace")
	}
	r.mu.Lock()
	sc := r.scopes[id]
	if sc == nil {
		r.scopes[id] = newScope()
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()
	if sc.disposed.Load() {
		return failure("scope_conflict", "scope disposed")
	}
	return nil
}
func (r *Registry) Definitions() []Definition {
	r.mu.Lock()
	defer r.mu.Unlock()
	ds := make([]Definition, 0, len(r.scenarios))
	for _, s := range r.scenarios {
		ds = append(ds, s.definition)
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i].Name < ds[j].Name })
	encoded, _ := json.Marshal(ds)
	var copies []Definition
	_ = json.Unmarshal(encoded, &copies)
	return copies
}
func cloneResult(result Result) Result {
	b, _ := json.Marshal(result)
	var copy Result
	_ = json.Unmarshal(b, &copy)
	return copy
}
func (r *Registry) Prepare(ctx context.Context, input Input) (Result, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return Result{}, failure("invalid_input", err.Error())
	}
	if err := json.Unmarshal(encoded, &input); err != nil {
		return Result{}, failure("invalid_input", err.Error())
	}
	if input.Seed == "" {
		return Result{}, failure("invalid_input", "empty seed")
	}
	if _, err := time.Parse(time.RFC3339, input.Now); err != nil {
		return Result{}, failure("invalid_input", "now must be RFC3339")
	}
	r.mu.Lock()
	s := r.scenarios[input.Name]
	sc := r.scopes[input.ScopeID]
	r.mu.Unlock()
	if s == nil {
		return Result{}, failure("unknown_scenario", "unknown scenario")
	}
	if s.definition.Version != input.Version {
		return Result{}, failure("version_mismatch", "scenario version mismatch")
	}
	if sc == nil {
		return Result{}, failure("scope_conflict", "scope is not owned by this environment")
	}
	if s.definition.Isolation == "dedicated" && !r.dedicated {
		return Result{}, failure("missing_capability", "scenario requires a dedicated environment")
	}
	for _, c := range s.definition.RequiredCapabilities {
		if !r.capabilities[c] {
			return Result{}, failure("missing_capability", c)
		}
	}
	if err := s.input.Validate(input.Params); err != nil {
		return Result{}, failure("invalid_input", err.Error())
	}
	if err := sc.lock(ctx); err != nil {
		return Result{}, failure("timeout", err.Error())
	}
	defer sc.unlock()
	if sc.disposed.Load() {
		return Result{}, failure("scope_conflict", "scope disposed")
	}
	if sc.request != "" && sc.request != string(encoded) {
		return Result{}, failure("scope_conflict", "scope has another input")
	}
	if sc.result != nil {
		return cloneResult(*sc.result), nil
	}
	if sc.scenario != nil {
		return Result{}, failure("execution_failed", "previous preparation failed; dispose scope before retry")
	}
	if err := ctx.Err(); err != nil {
		return Result{}, failure("timeout", err.Error())
	}
	r.mu.Lock()
	if r.exclusiveScope != "" && r.exclusiveScope != input.ScopeID {
		r.mu.Unlock()
		return Result{}, failure("scope_conflict", "environment has a dedicated scenario lease")
	}
	if s.definition.Isolation == "dedicated" {
		for active := range r.active {
			if active != input.ScopeID {
				r.mu.Unlock()
				return Result{}, failure("scope_conflict", "dedicated scenario requires an empty environment")
			}
		}
		r.exclusiveScope = input.ScopeID
	}
	r.active[input.ScopeID] = true
	r.mu.Unlock()
	sc.request = string(encoded)
	sc.scenario = s
	result, err := s.prepare(ctx, input)
	if err == nil {
		if result.Data == nil {
			result.Data = map[string]any{}
		}
		dataBytes, encodingErr := json.Marshal(result.Data)
		if encodingErr != nil {
			err = encodingErr
		} else {
			err = json.Unmarshal(dataBytes, &result.Data)
		}
	}
	if err == nil {
		err = s.output.Validate(result.Data)
	}
	if err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if cleanupErr := s.cleanup(cleanupCtx, input.ScopeID); cleanupErr != nil {
			return Result{}, failure("cleanup_failed", cleanupErr.Error())
		}
		return Result{}, failure("execution_failed", err.Error())
	}
	result.ScopeID = input.ScopeID
	result.Name = input.Name
	result.Version = input.Version
	result.Seed = input.Seed
	result.Now = input.Now
	if result.Refs == nil {
		result.Refs = map[string]EntityRef{}
	}
	if result.Data == nil {
		result.Data = map[string]any{}
	}
	copy := cloneResult(result)
	sc.result = &copy
	return cloneResult(result), nil
}
func (r *Registry) Dispose(ctx context.Context, id string) error {
	r.mu.Lock()
	sc := r.scopes[id]
	r.mu.Unlock()
	if sc == nil {
		return failure("scope_conflict", "scope is not owned by this environment")
	}
	if err := sc.lock(ctx); err != nil {
		return failure("timeout", err.Error())
	}
	defer sc.unlock()
	if sc.disposed.Load() {
		return nil
	}
	if sc.scenario != nil {
		if err := sc.scenario.cleanup(ctx, id); err != nil {
			return failure("cleanup_failed", err.Error())
		}
	}
	sc.disposed.Store(true)
	sc.result = nil
	r.mu.Lock()
	delete(r.active, id)
	if r.exclusiveScope == id {
		r.exclusiveScope = ""
	}
	r.mu.Unlock()
	return nil
}
