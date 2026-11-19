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
// (JSON Schema 2020-12) validates each scripted step's response; success steps
// must carry one. Set CredentialResponses when responses carry credentials
// (tokens, secrets): such scripts work in the scope but manifest export of a
// scope that has them is refused instead of serializing the credential.
type ProviderDefinition struct {
	Name                string `json:"name"`
	ResponseSchema      any    `json:"responseSchema"`
	CredentialResponses bool   `json:"credentialResponses,omitempty"`
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
	// prepared counts scripts submitted before the scenario callback ran.
	prepared int
	queue    []ProviderStep
	calls    []ProviderCall
	gates    map[string]*Gate
	// holding counts calls currently blocked at each gate.
	holding map[string]int
	done    chan struct{}
}

// providerSnapshot splits scripts by replay phase.
type providerSnapshot struct {
	BeforePrepare []json.RawMessage `json:"beforePrepare,omitempty"`
	AfterPrepare  []json.RawMessage `json:"afterPrepare,omitempty"`
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
				"if":   map[string]any{"properties": map[string]any{"outcome": map[string]any{"const": OutcomeSuccess}}},
				"then": map[string]any{"required": []any{"response"}},
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

// Register declares a provider fake. The schema is snapshotted, so discovery
// and validation always agree.
func (p *Providers) Register(definition ProviderDefinition) error {
	if definition.Name == "" {
		return failure("invalid_input", "provider name required")
	}
	if definition.ResponseSchema != nil {
		snapshot, err := jsonCopy(definition.ResponseSchema)
		if err != nil {
			return failure("invalid_input", "provider response schema: "+err.Error())
		}
		definition.ResponseSchema = snapshot
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
		// Each caller gets an independent copy of the registered snapshot.
		schema, err := jsonCopy(d.definition.PayloadSchema())
		if err != nil {
			continue
		}
		out = append(out, map[string]any{"name": name, "payloadSchema": schema, "credentialResponses": d.definition.CredentialResponses})
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
		state = &providerState{gates: map[string]*Gate{}, holding: map[string]int{}, done: make(chan struct{})}
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
	release, err := p.registry.admit(scopeID)
	if err != nil {
		return err
	}
	defer release()
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

// Release opens a hold gate. Releasing before the call arrives lets it pass
// without being held.
func (p *Providers) Release(scopeID, provider, gate string) error {
	release, err := p.registry.admit(scopeID)
	if err != nil {
		return err
	}
	defer release()
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

// WaitHeld blocks until a call is blocked at gate right now, so a test can
// act while the provider call is in flight. The wait stays bound to the
// provider state it started with: if that state is forgotten, it fails.
func (p *Providers) WaitHeld(ctx context.Context, scopeID, provider, gate string) error {
	release, err := p.registry.admit(scopeID)
	if err != nil {
		return err
	}
	p.mu.Lock()
	if p.definitions[provider] == nil {
		p.mu.Unlock()
		release()
		return failure("unknown_scenario", "unknown provider: "+provider)
	}
	state := p.stateLocked(scopeID, provider)
	p.mu.Unlock()
	release()
	for {
		p.mu.Lock()
		current := p.scopes[scopeID][provider]
		held := state.holding[gate] > 0
		changed := p.changed
		p.mu.Unlock()
		if current != state {
			return failure("scope_conflict", "provider state forgotten")
		}
		if held {
			return nil
		}
		select {
		case <-state.done:
			return failure("scope_conflict", "provider state forgotten")
		case <-ctx.Done():
			return failure("timeout", ctx.Err().Error())
		case <-changed:
		}
	}
}

// Call is used by the consumer's fake provider. It takes the scope from ctx,
// records the call, consumes the first queued step whose operation is empty
// or equal to operation, and honors its gate.
func (p *Providers) Call(ctx context.Context, provider, operation, key string) (ProviderStep, error) {
	correlation, ok := CorrelationFrom(ctx)
	if !ok {
		return ProviderStep{}, ErrUncorrelated
	}
	scopeID := correlation.ScopeID
	release, err := p.registry.admit(scopeID)
	if err != nil {
		return ProviderStep{}, err
	}
	p.mu.Lock()
	if p.definitions[provider] == nil {
		p.mu.Unlock()
		release()
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
		release()
		return ProviderStep{}, ErrUnscripted
	}
	step := state.queue[index]
	// Remove in place: no reallocation per call, order of the rest kept.
	copy(state.queue[index:], state.queue[index+1:])
	state.queue[len(state.queue)-1] = ProviderStep{}
	state.queue = state.queue[:len(state.queue)-1]
	call.Outcome, call.Hold = step.Outcome, step.Hold
	var gate *Gate
	if step.Hold != "" {
		gate = state.gates[step.Hold]
		if gate == nil {
			gate = NewGate()
			state.gates[step.Hold] = gate
		}
		select {
		case <-gate.release:
			gate = nil // released before arrival: passes without being held
		default:
			call.Outcome = "held"
			state.holding[step.Hold]++
		}
	}
	position := len(state.calls)
	state.calls = append(state.calls, call)
	done := state.done
	p.notifyLocked()
	p.mu.Unlock()
	// Admission covers recording only; blocking at a gate must not delay disposal.
	release()
	if gate == nil {
		return step, nil
	}
	gate.enterOnce.Do(func() { close(gate.entered) })
	outcome, err := step.Outcome, error(nil)
	select {
	case <-gate.release:
	case <-done:
		outcome, err = "canceled", failure("scope_conflict", "provider state forgotten")
	case <-ctx.Done():
		outcome, err = "canceled", ctx.Err()
	}
	p.mu.Lock()
	state.holding[step.Hold]--
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

func (p *Providers) beforePrepare(scopeID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, state := range p.scopes[scopeID] {
		state.prepared = len(state.scripts)
	}
}

// SnapshotScope captures submitted scripts (not consumption) for replay,
// split into those present before the scenario callback and those after.
// Scopes holding scripts of a CredentialResponses provider are refused.
func (p *Providers) SnapshotScope(scopeID string) (json.RawMessage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	snapshot := map[string]providerSnapshot{}
	for provider, state := range p.scopes[scopeID] {
		if len(state.scripts) == 0 {
			continue
		}
		if d := p.definitions[provider]; d != nil && d.definition.CredentialResponses {
			return nil, failure("invalid_input", "provider "+provider+" scripts carry credentials; manifest export refused")
		}
		snapshot[provider] = providerSnapshot{
			BeforePrepare: append([]json.RawMessage(nil), state.scripts[:state.prepared]...),
			AfterPrepare:  append([]json.RawMessage(nil), state.scripts[state.prepared:]...),
		}
	}
	if len(snapshot) == 0 {
		return nil, nil
	}
	return json.Marshal(snapshot)
}

// RestoreScope re-submits the scripts of one replay phase to a fresh scope.
func (p *Providers) RestoreScope(ctx context.Context, scopeID string, snapshot json.RawMessage, phase RestorePhase) error {
	var scripts map[string]providerSnapshot
	if err := json.Unmarshal(snapshot, &scripts); err != nil {
		return failure("invalid_input", "provider snapshot: "+err.Error())
	}
	providers := make([]string, 0, len(scripts))
	for provider := range scripts {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		part := scripts[provider].AfterPrepare
		if phase == RestoreBeforePrepare {
			part = scripts[provider].BeforePrepare
		}
		for _, script := range part {
			if err := p.Script(ctx, scopeID, provider, script); err != nil {
				return err
			}
		}
	}
	return nil
}

func jsonCopy(value any) (any, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var copied any
	if err := json.Unmarshal(encoded, &copied); err != nil {
		return nil, err
	}
	return copied, nil
}
