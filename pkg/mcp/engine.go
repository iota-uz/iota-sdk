package mcp

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Authorizer admits a principal to a policy. It runs on every call against
// current account state and returns the context domain services expect
// (tenant, user). Returning an error denies the call.
type Authorizer interface {
	Authorize(ctx context.Context, p Principal, policy Policy) (context.Context, error)
}

type AuthorizerFunc func(ctx context.Context, p Principal, policy Policy) (context.Context, error)

func (f AuthorizerFunc) Authorize(ctx context.Context, p Principal, policy Policy) (context.Context, error) {
	return f(ctx, p, policy)
}

// AuditEntry deliberately omits arguments and results: they may hold
// business data or secrets and belong to the domain's own history.
type AuditEntry struct {
	RequestID   string
	Tool        string
	Subject     string
	TenantID    string
	ClientID    string
	Nested      bool
	Outcome     Code
	OperationID string
	// Item is the index of the change of OperationID an entry is about (the audit of guarded changes):
	// with OperationID it identifies the entry, which may be delivered more than once.
	Item     *int
	Duration time.Duration
}

type Auditor interface {
	Record(ctx context.Context, e AuditEntry)
}

type Engine struct {
	registry *Registry
	authz    Authorizer
	audit    Auditor
}

type EngineOption func(*Engine)

func WithAuditor(a Auditor) EngineOption {
	return func(e *Engine) { e.audit = a }
}

func NewEngine(registry *Registry, authz Authorizer, opts ...EngineOption) (*Engine, error) {
	const op serrors.Op = "mcp.NewEngine"
	if registry == nil || authz == nil {
		return nil, serrors.New(serrors.Invalid, "registry and authorizer are required").WithOp(op)
	}
	e := &Engine{registry: registry, authz: authz}
	for _, o := range opts {
		o(e)
	}
	return e, nil
}

func (e *Engine) Registry() *Registry { return e.registry }

// Operations returns the operations the caller of ctx may call now: every
// tool except host-only ones, filtered by the same authorizer Call runs.
// Hiding a tool here is a convenience for discovery; Call still enforces
// policy on every invocation.
func (e *Engine) Operations(ctx context.Context) []Spec {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return nil
	}
	verdicts := make(map[string]bool)
	var out []Spec
	for _, spec := range e.registry.Specs() {
		if spec.HostOnly {
			continue
		}
		key := policyKey(spec.Policy)
		allowed, seen := verdicts[key]
		if !seen {
			_, err := e.authz.Authorize(ctx, p, spec.Policy)
			allowed = err == nil
			verdicts[key] = allowed
		}
		if allowed {
			out = append(out, spec)
		}
	}
	return out
}

// Operation returns one operation if the caller of ctx may call it now.
func (e *Engine) Operation(ctx context.Context, name string) (Spec, bool) {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return Spec{}, false
	}
	spec, ok := e.registry.Spec(name)
	if !ok || spec.HostOnly {
		return Spec{}, false
	}
	if _, err := e.authz.Authorize(ctx, p, spec.Policy); err != nil {
		return Spec{}, false
	}
	return spec, true
}

// Allows reports whether the caller of ctx passes policy now, without
// running any handler.
func (e *Engine) Allows(ctx context.Context, policy Policy) bool {
	p, ok := PrincipalFrom(ctx)
	if !ok {
		return false
	}
	_, err := e.authz.Authorize(ctx, p, policy)
	return err == nil
}

func policyKey(p Policy) string {
	scopes := slices.Clone(p.Scopes)
	perms := slices.Clone(p.Permissions)
	slices.Sort(scopes)
	slices.Sort(perms)
	return strings.Join(scopes, " ") + "|" + strings.Join(perms, " ")
}

type nestedKey struct{}

// WithNestedCaller marks calls made by isolated code: host-only tools are
// invisible to it.
func WithNestedCaller(ctx context.Context) context.Context {
	return context.WithValue(ctx, nestedKey{}, true)
}

func isNested(ctx context.Context) bool {
	nested, _ := ctx.Value(nestedKey{}).(bool)
	return nested
}

// Call is the one executor behind every adapter: identity, policy, schema
// validation and the handler run here and nowhere else.
func (e *Engine) Call(ctx context.Context, name string, input json.RawMessage) Result {
	started := time.Now()
	p, ok := PrincipalFrom(ctx)
	if e.audit != nil {
		ctx = context.WithValue(ctx, auditKey{}, &auditScope{audit: e.audit, base: AuditEntry{
			RequestID: RequestIDFrom(ctx), Subject: p.Subject(), TenantID: p.TenantID.String(), ClientID: p.ClientID, Nested: isNested(ctx),
		}})
	}
	res := e.call(ctx, p, ok, name, input)
	if e.audit != nil {
		entry := AuditEntry{
			RequestID: RequestIDFrom(ctx),
			Tool:      name,
			Subject:   p.Subject(),
			TenantID:  p.TenantID.String(),
			ClientID:  p.ClientID,
			Nested:    isNested(ctx),
			Duration:  time.Since(started),
		}
		if res.Error != nil {
			entry.Outcome = res.Error.Code
		}
		entry.OperationID = OperationIDOf(res)
		e.audit.Record(ctx, entry)
	}
	return res
}

func (e *Engine) call(ctx context.Context, p Principal, authenticated bool, name string, input json.RawMessage) Result {
	if !authenticated {
		return Failed(Fail(CodeForbidden, RecoveryReauthenticate, "the caller is not authenticated"))
	}
	ent, ok := e.registry.lookup(name)
	if !ok || (ent.spec.HostOnly && isNested(ctx)) {
		return Failed(Fail(CodeNotFound, RecoveryFixInput, "unknown tool "+name))
	}
	authCtx, err := e.authz.Authorize(ctx, p, ent.spec.Policy)
	if err != nil {
		return Failed(FailureFrom(err))
	}
	if issues := ent.input.validate(input); len(issues) > 0 {
		return Failed(Fail(CodeInvalidInput, RecoveryFixInput, "input does not match the tool schema", issues...))
	}
	if len(input) == 0 {
		input = json.RawMessage("{}")
	}
	out, err := ent.tool.Call(authCtx, input)
	if err != nil {
		return Failed(FailureFrom(err))
	}
	if res, ok := out.(Result); ok {
		return res
	}
	res, err := Success(out)
	if err != nil {
		return Failed(Fail(CodeDependencyUnavailable, RecoveryRetryLater, "the tool returned an unserializable result"))
	}
	return res
}

type auditKey struct{}

// auditScope is the call an Audit event belongs to.
type auditScope struct {
	audit Auditor
	base  AuditEntry
}

// AuditorFrom is the auditor of the engine running the call of ctx, if it has one.
func AuditorFrom(ctx context.Context) (Auditor, bool) {
	s, ok := ctx.Value(auditKey{}).(*auditScope)
	if !ok {
		return nil, false
	}
	return s.audit, true
}

// Audit records an event of the running call in the engine's audit with the call's principal, client and
// request: what a handler decided inside a successful result (a refusal it reports as a plan issue, each
// change it applied). tool names the event, such as "changes:automation.update". Outside a call of an
// engine with an auditor it does nothing.
func Audit(ctx context.Context, tool string, outcome Code, operationID string) {
	s, ok := ctx.Value(auditKey{}).(*auditScope)
	if !ok {
		return
	}
	entry := s.base
	entry.Tool, entry.Outcome, entry.OperationID = tool, outcome, operationID
	s.audit.Record(ctx, entry)
}

func operationIDOf(data json.RawMessage) string {
	var head struct {
		OperationID string `json:"operation_id"`
	}
	_ = json.Unmarshal(data, &head)
	return head.OperationID
}

// OperationIDOf extracts operation_id from a result so callers can report
// receipts of writes that happened before a run stopped.
func OperationIDOf(r Result) string {
	if r.Error != nil {
		return r.Error.OperationID
	}
	return operationIDOf(r.Data)
}
