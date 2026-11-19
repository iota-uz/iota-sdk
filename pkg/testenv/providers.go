package testenv

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Provider step outcomes. A fake adapter maps them onto its own protocol.
const (
	OutcomeSuccess = "success"
	OutcomeRefusal = "refusal"
	OutcomeTimeout = "timeout"
)

var (
	// ErrProviderRefused and ErrProviderTimeout are returned by ProviderStep.Err.
	ErrProviderRefused = errors.New("provider refused")
	ErrProviderTimeout = errors.New("provider timed out")
	// ErrUnscripted means a call reached the fake without a matching scripted
	// step. Fakes must fail instead of inventing success, which would make a
	// test green without exercising the scripted path.
	ErrUnscripted = errors.New("provider call is not scripted")
	// ErrUncorrelated means a call carried no scope correlation.
	ErrUncorrelated = errors.New("provider call has no scope correlation")
)

// ProviderDefinition is declared by the consumer's fake adapter. ResponseSchema
// (JSON Schema 2020-12) validates each scripted step's response.
type ProviderDefinition struct {
	Name           string `json:"name"`
	ResponseSchema any    `json:"responseSchema"`
}

// ProviderStep is one scripted answer. Operation, when set, matches only that
// operation. Hold names a gate: the call is recorded, then waits until the
// test releases the gate, the call is canceled, or the scope is forgotten.
type ProviderStep struct {
	Operation string         `json:"operation,omitempty"`
	Outcome   string         `json:"outcome"`
	Hold      string         `json:"hold,omitempty"`
	Response  map[string]any `json:"response,omitempty"`
}

// Err maps refusal and timeout outcomes to sentinel errors.
func (s ProviderStep) Err() error {
	switch s.Outcome {
	case OutcomeRefusal:
		return ErrProviderRefused
	case OutcomeTimeout:
		return ErrProviderTimeout
	}
	return nil
}

// ProviderScript is the payload accepted by Script: steps consumed in order.
type ProviderScript struct {
	Steps []ProviderStep `json:"steps"`
}

// ProviderCall is one observed call to a fake provider.
type ProviderCall struct {
	Sequence    int    `json:"sequence"`
	Operation   string `json:"operation"`
	Key         string `json:"key"`
	Outcome     string `json:"outcome"` // a step outcome, or held|canceled|unscripted
	Correlation string `json:"correlation"`
	Hold        string `json:"hold,omitempty"`
}

type providerDefinition struct {
	definition ProviderDefinition
	payload    *jsonschema.Schema
}

type providerState struct {
	scripts []json.RawMessage
	queue   []ProviderStep
	calls   []ProviderCall
	gates   map[string]*Gate
	done    chan struct{}
}

// Providers holds scoped scripts for consumer fake providers.
type Providers struct {
	registry    *Registry
	mu          sync.Mutex
	definitions map[string]*providerDefinition
	scopes      map[string]map[string]*providerState
	changed     chan struct{}
}

// NewProviders attaches provider controls to r.
func NewProviders(r *Registry) (*Providers, error) {
	p := &Providers{registry: r, definitions: map[string]*providerDefinition{}, scopes: map[string]map[string]*providerState{}, changed: make(chan struct{})}
	if err := r.Attach("providers", p); err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.providers = p
	r.mu.Unlock()
	return p, nil
}

// PayloadSchema returns the JSON Schema of Script payloads for a provider.
func (d ProviderDefinition) PayloadSchema() map[string]any {
	response := d.ResponseSchema
	if response == nil {
		response = map[string]any{"type": "object"}
	}
	return map[string]any{
		"type": "object", "required": []any{"steps"}, "additionalProperties": false,
		"properties": map[string]any{"steps": map[string]any{
			"type": "array", "minItems": 1,
			"items": map[string]any{
				"type": "object", "required": []any{"outcome"}, "additionalProperties": false,
				"properties": map[string]any{
					"operation": map[string]any{"type": "string"},
					"outcome":   map[string]any{"enum": []any{OutcomeSuccess, OutcomeRefusal, OutcomeTimeout}},
					"hold":      map[string]any{"type": "string", "minLength": 1},
					"response":  response,
				},
			},
		}},
	}
}

// Register declares a provider fake.
func (p *Providers) Register(definition ProviderDefinition) error {
	if definition.Name == "" {
		return failure("invalid_input", "provider name required")
	}
	schema, err := compileSchema(definition.PayloadSchema())
	if err != nil {
		return failure("invalid_input", "provider response schema: "+err.Error())
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.definitions[definition.Name]; exists {
		return failure("scope_conflict", "provider already registered")
	}
	p.definitions[definition.Name] = &providerDefinition{definition: definition, payload: schema}
	return nil
}

// Definitions lists providers with their payload schemas.
func (p *Providers) Definitions() []map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]map[string]any, 0, len(p.definitions))
	for name, d := range p.definitions {
		out = append(out, map[string]any{"name": name, "payloadSchema": d.definition.PayloadSchema()})
	}
	sort.Slice(out, func(a, b int) bool { return out[a]["name"].(string) < out[b]["name"].(string) })
	return out
}

func (p *Providers) stateLocked(scopeID, provider string) *providerState {
	byProvider := p.scopes[scopeID]
	if byProvider == nil {
		byProvider = map[string]*providerState{}
		p.scopes[scopeID] = byProvider
	}
	state := byProvider[provider]
	if state == nil {
		state = &providerState{gates: map[string]*Gate{}, done: make(chan struct{})}
		byProvider[provider] = state
	}
	return state
}

func (p *Providers) notifyLocked() {
	close(p.changed)
	p.changed = make(chan struct{})
}

// Script validates payload against the provider's schema and appends its steps.
func (p *Providers) Script(ctx context.Context, scopeID, provider string, payload any) error {
	if err := ctx.Err(); err != nil {
		return failure("timeout", err.Error())
	}
	if err := p.registry.owned(scopeID); err != nil {
		return err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return failure("invalid_input", err.Error())
	}
	var document any
	if err := json.Unmarshal(encoded, &document); err != nil {
		return failure("invalid_input", err.Error())
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	definition := p.definitions[provider]
	if definition == nil {
		return failure("unknown_scenario", "unknown provider: "+provider)
	}
	if err := definition.payload.Validate(document); err != nil {
		return failure("invalid_input", err.Error())
	}
	var script ProviderScript
	if err := json.Unmarshal(encoded, &script); err != nil {
		return failure("invalid_input", err.Error())
	}
	state := p.stateLocked(scopeID, provider)
	state.scripts = append(state.scripts, json.RawMessage(encoded))
	state.queue = append(state.queue, script.Steps...)
	return nil
}

// Calls returns the scope's observed calls to provider in sequence order.
func (p *Providers) Calls(scopeID, provider string) ([]ProviderCall, error) {
	if err := p.registry.owned(scopeID); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.definitions[provider] == nil {
		return nil, failure("unknown_scenario", "unknown provider: "+provider)
	}
	state := p.scopes[scopeID][provider]
	if state == nil {
		return []ProviderCall{}, nil
	}
	return append([]ProviderCall{}, state.calls...), nil
}

// Release opens a hold gate. Releasing before the call arrives lets it pass.
func (p *Providers) Release(scopeID, provider, gate string) error {
	if err := p.registry.owned(scopeID); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.definitions[provider] == nil {
		return failure("unknown_scenario", "unknown provider: "+provider)
	}
	state := p.stateLocked(scopeID, provider)
	g := state.gates[gate]
	if g == nil {
		g = NewGate()
		state.gates[gate] = g
	}
	g.Release()
	return nil
}

// WaitHeld blocks until a call is held at gate, so a test can act while the
// provider call is in flight.
func (p *Providers) WaitHeld(ctx context.Context, scopeID, provider, gate string) error {
	if err := p.registry.owned(scopeID); err != nil {
		return err
	}
	for {
		p.mu.Lock()
		if p.definitions[provider] == nil {
			p.mu.Unlock()
			return failure("unknown_scenario", "unknown provider: "+provider)
		}
		state := p.stateLocked(scopeID, provider)
		g := state.gates[gate]
		if g == nil {
			g = NewGate()
			state.gates[gate] = g
		}
		changed, done := p.changed, state.done
		p.mu.Unlock()
		select {
		case <-g.Entered():
			return nil
		case <-done:
			return failure("scope_conflict", "provider state forgotten")
		case <-ctx.Done():
			return failure("timeout", ctx.Err().Error())
		case <-changed:
		}
	}
}

// Call is used by the consumer's fake provider. It takes the scope from ctx,
// records the call, consumes the next matching step and honors its gate.
func (p *Providers) Call(ctx context.Context, provider, operation, key string) (ProviderStep, error) {
	correlation, ok := CorrelationFrom(ctx)
	if !ok {
		return ProviderStep{}, ErrUncorrelated
	}
	scopeID := correlation.ScopeID
	if err := p.registry.owned(scopeID); err != nil {
		return ProviderStep{}, err
	}
	p.mu.Lock()
	if p.definitions[provider] == nil {
		p.mu.Unlock()
		return ProviderStep{}, failure("unknown_scenario", "unknown provider: "+provider)
	}
	state := p.stateLocked(scopeID, provider)
	call := ProviderCall{Sequence: len(state.calls) + 1, Operation: operation, Key: key, Correlation: correlation.String()}
	index := -1
	for i, step := range state.queue {
		if step.Operation == "" || step.Operation == operation {
			index = i
			break
		}
	}
	if index < 0 {
		call.Outcome = "unscripted"
		state.calls = append(state.calls, call)
		p.notifyLocked()
		p.mu.Unlock()
		return ProviderStep{}, ErrUnscripted
	}
	step := state.queue[index]
	state.queue = append(state.queue[:index:index], state.queue[index+1:]...)
	call.Outcome, call.Hold = step.Outcome, step.Hold
	var gate *Gate
	if step.Hold != "" {
		gate = state.gates[step.Hold]
		if gate == nil {
			gate = NewGate()
			state.gates[step.Hold] = gate
		}
		call.Outcome = "held"
	}
	position := len(state.calls)
	state.calls = append(state.calls, call)
	done := state.done
	p.mu.Unlock()
	if gate == nil {
		return step, nil
	}
	gate.enterOnce.Do(func() { close(gate.entered) })
	p.mu.Lock()
	p.notifyLocked()
	p.mu.Unlock()
	outcome, err := step.Outcome, error(nil)
	select {
	case <-gate.release:
	case <-done:
		outcome, err = "canceled", failure("scope_conflict", "provider state forgotten")
	case <-ctx.Done():
		outcome, err = "canceled", ctx.Err()
	}
	p.mu.Lock()
	if position < len(state.calls) && state.calls[position].Sequence == call.Sequence {
		state.calls[position].Outcome = outcome
	}
	p.notifyLocked()
	p.mu.Unlock()
	if err != nil {
		return ProviderStep{}, err
	}
	return step, nil
}

// ForgetScope removes only scopeID's scripts and calls and ends its held calls.
func (p *Providers) ForgetScope(_ context.Context, scopeID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, state := range p.scopes[scopeID] {
		close(state.done)
	}
	delete(p.scopes, scopeID)
	p.notifyLocked()
	return nil
}

// SnapshotScope captures submitted scripts (not consumption) for replay.
func (p *Providers) SnapshotScope(scopeID string) (json.RawMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	snapshot := map[string][]json.RawMessage{}
	for provider, state := range p.scopes[scopeID] {
		if len(state.scripts) > 0 {
			snapshot[provider] = append([]json.RawMessage(nil), state.scripts...)
		}
	}
	if len(snapshot) == 0 {
		return nil, nil
	}
	return json.Marshal(snapshot)
}

// RestoreScope re-submits captured scripts to a fresh scope.
func (p *Providers) RestoreScope(ctx context.Context, scopeID string, snapshot json.RawMessage) error {
	var scripts map[string][]json.RawMessage
	if err := json.Unmarshal(snapshot, &scripts); err != nil {
		return failure("invalid_input", "provider snapshot: "+err.Error())
	}
	providers := make([]string, 0, len(scripts))
	for provider := range scripts {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		for _, script := range scripts[provider] {
			if err := p.Script(ctx, scopeID, provider, script); err != nil {
				return err
			}
		}
	}
	return nil
}
