// Package native is the Go authoring API of native (Go-implemented)
// operations, commands and queries of the extension runtime.
//
// A native resource is a public contract in the module's namespace (for
// example "crm.operations.upsertClient") with an optional Go default
// implementation. Contracts are declared once, as package-level values, and
// registered in Default, so the catalog and the developer kit publish them
// without a running application:
//
//	var UpsertClient = native.MustRegister(native.Operation("crm.operations.upsertClient",
//	    native.Spec{Description: "..."},
//	    func(d native.Deps) (native.OperationFunc[UpsertIn, UpsertOut], error) {
//	        svc, err := native.Use[*ClientService](d)
//	        if err != nil {
//	            return nil, err
//	        }
//	        return func(ctx context.Context, tx native.Tx, in UpsertIn) (UpsertOut, error) { ... }, nil
//	    })
//
//	func Natives(s *native.Set) error { return s.Add(UpsertClient, ClientUpdated, native.Entity(...)) }
//
// The Go default is a factory over the application's dependencies (Deps, the
// composition container), so one process can host several applications.
//
// Every entry of a native command goes through a Dispatcher. Without the
// extension runtime the Direct dispatcher calls the Go default; with the
// runtime the invocation host dispatches it like package code: input/output
// validation, permission, guards of packages, invocation records, nesting
// limit, and a package replacement bound to the command (one active
// implementation, no fallback on error).
package native

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Kind is the invocation kind of a native contract. Atomicity is part of the
// public contract: a replacement of an atomic command gets a Tx, a replacement
// of an ordinary command gets a CommandContext.
type Kind string

const (
	KindOperation Kind = "operation"
	KindAtomic    Kind = "atomic"
	KindCommand   Kind = "command"
	KindQuery     Kind = "query"
)

// Spec is the public part of a native contract besides its ID and types.
type Spec struct {
	// Permission is the RBAC permission name (permission.Name(), for example
	// "Client.Update") the subject must hold to invoke the contract, whatever
	// implementation is bound to it. RBAC names are not catalog resources, so
	// the host enforces it from the native definition; it is not part of the
	// published resource.
	Permission string
	// Errors lists the declared domain error IDs (catalog resources).
	Errors      []string
	Label       manifest.Label
	Description string
	// Input and Output override the types derived from the Go input and
	// output structs (TypeOf).
	Input  *manifest.Type
	Output *manifest.Type
}

// Deps resolves the dependencies of a Go default implementation.
// *composition.Container implements it.
type Deps interface {
	ResolveType(t reflect.Type) (any, error)
}

// Use resolves a dependency of type T.
func Use[T any](d Deps) (T, error) {
	var zero T
	if d == nil {
		return zero, serrors.New(serrors.FailedPrecondition, "native: no dependencies to resolve "+reflect.TypeFor[T]().String())
	}
	v, err := d.ResolveType(reflect.TypeFor[T]())
	if err != nil {
		return zero, err
	}
	t, ok := v.(T)
	if !ok {
		return zero, serrors.New(serrors.Internal, fmt.Sprintf("native: dependency is %T, not %s", v, reflect.TypeFor[T]()))
	}
	return t, nil
}

// OperationFunc is the Go handler of an operation or atomic command. It runs
// in the caller's transaction (tx.call) or in one the host opens; ctx carries
// that transaction (composables.UseTx) and the tenant.
type OperationFunc[I, O any] func(ctx context.Context, tx Tx, in I) (O, error)

// CommandFunc is the Go handler of an ordinary command: no transaction is
// open; it opens its own with c.Transaction.
type CommandFunc[I, O any] func(ctx context.Context, c CommandContext, in I) (O, error)

// QueryFunc is the Go handler of a query: it reads in the caller's snapshot
// or transaction (tx.query, q.query) or a read-only snapshot of its own.
type QueryFunc[I, O any] func(ctx context.Context, q QueryContext, in I) (O, error)

// Definition is the untyped view of a native contract.
type Definition interface {
	ID() string
	Kind() Kind
	// Resource is the public contract published in the catalog and the kit.
	Resource() manifest.Resource
	// Permission is Spec.Permission.
	Permission() string
	// HasDefault reports whether a Go default implementation is declared.
	HasDefault() bool
	bind(d Deps) (*Implementation, bool, error)
}

// Def is a typed native contract.
type Def[I, O any] struct {
	id      string
	kind    Kind
	spec    Spec
	res     manifest.Resource
	factory func(Deps) (any, error)
}

var nativeIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-zA-Z0-9_]*)+$`)

func newDef[I, O any](id string, kind Kind, spec Spec, factory func(Deps) (any, error)) (*Def[I, O], error) {
	const op = serrors.Op("extruntime.native.New")
	if !nativeIDPattern.MatchString(id) {
		return nil, serrors.New(serrors.Invalid, fmt.Sprintf("native contract id %q is invalid", id)).WithOp(op)
	}
	in, out := spec.Input, spec.Output
	if in == nil {
		t, err := TypeOf[I]()
		if err != nil {
			return nil, serrors.Wrap(op, fmt.Errorf("%s input: %w", id, err))
		}
		in = &t
	}
	if out == nil {
		t, err := TypeOf[O]()
		if err != nil {
			return nil, serrors.Wrap(op, fmt.Errorf("%s output: %w", id, err))
		}
		out = &t
	}
	res := manifest.Resource{
		ID: id, Input: in, Output: out, Errors: append([]string(nil), spec.Errors...),
		Label: spec.Label, Description: spec.Description,
	}
	switch kind {
	case KindOperation:
		res.Kind = manifest.ResOperation
	case KindAtomic:
		res.Kind, res.Atomic = manifest.ResCommand, true
	case KindCommand:
		res.Kind = manifest.ResCommand
	case KindQuery:
		res.Kind = manifest.ResQuery
	default:
		return nil, serrors.New(serrors.Invalid, "unknown native kind "+string(kind)).WithOp(op)
	}
	res.ContractHash = manifest.ContractHashOf(res)
	return &Def[I, O]{id: id, kind: kind, spec: spec, res: res, factory: factory}, nil
}

func must[I, O any](d *Def[I, O], err error) *Def[I, O] {
	if err != nil {
		panic(err)
	}
	return d
}

func factoryOf[F any](impl func(Deps) (F, error)) func(Deps) (any, error) {
	if impl == nil {
		return nil
	}
	return func(d Deps) (any, error) { return impl(d) }
}

// Operation declares a native operation: reusable invariant logic other code
// (Go or packages, through tx.call) runs inside its own transaction. impl may
// be nil for a contract without a Go default. It panics on an invalid ID or
// input/output types TypeOf cannot describe; NewOperation returns the error.
func Operation[I, O any](id string, spec Spec, impl func(Deps) (OperationFunc[I, O], error)) *Def[I, O] {
	return must(NewOperation(id, spec, impl))
}

// NewOperation is Operation returning the error.
func NewOperation[I, O any](id string, spec Spec, impl func(Deps) (OperationFunc[I, O], error)) (*Def[I, O], error) {
	return newDef[I, O](id, KindOperation, spec, factoryOf(impl))
}

// AtomicCommand declares an atomic command: the whole handler runs in one
// transaction, and a replacement gets a Tx as well.
func AtomicCommand[I, O any](id string, spec Spec, impl func(Deps) (OperationFunc[I, O], error)) *Def[I, O] {
	return must(newDef[I, O](id, KindAtomic, spec, factoryOf(impl)))
}

// Command declares an ordinary command: the replaceable algorithm of an
// intent. Its Go default opens transactions through CommandContext.
func Command[I, O any](id string, spec Spec, impl func(Deps) (CommandFunc[I, O], error)) *Def[I, O] {
	return must(NewCommand(id, spec, impl))
}

// NewCommand is Command returning the error.
func NewCommand[I, O any](id string, spec Spec, impl func(Deps) (CommandFunc[I, O], error)) (*Def[I, O], error) {
	return newDef[I, O](id, KindCommand, spec, factoryOf(impl))
}

// Query declares a native query.
func Query[I, O any](id string, spec Spec, impl func(Deps) (QueryFunc[I, O], error)) *Def[I, O] {
	return must(newDef[I, O](id, KindQuery, spec, factoryOf(impl)))
}

func (d *Def[I, O]) ID() string                  { return d.id }
func (d *Def[I, O]) Kind() Kind                  { return d.kind }
func (d *Def[I, O]) Permission() string          { return d.spec.Permission }
func (d *Def[I, O]) HasDefault() bool            { return d.factory != nil }
func (d *Def[I, O]) Resource() manifest.Resource { return cloneResource(d.res) }

func cloneResource(r manifest.Resource) manifest.Resource {
	b, err := json.Marshal(r)
	if err != nil {
		return r
	}
	var out manifest.Resource
	if err := json.Unmarshal(b, &out); err != nil {
		return r
	}
	return out
}

func (d *Def[I, O]) bind(deps Deps) (*Implementation, bool, error) {
	if d.factory == nil {
		return nil, false, nil
	}
	f, err := d.factory(deps)
	if err != nil {
		return nil, false, serrors.Wrap("extruntime.native.bind", fmt.Errorf("%s: %w", d.id, err))
	}
	impl := &Implementation{def: d}
	switch fn := f.(type) {
	case OperationFunc[I, O]:
		if fn == nil {
			return nil, false, nil
		}
		impl.tx = func(ctx context.Context, tx Tx, raw json.RawMessage) (json.RawMessage, error) {
			return typed(raw, func(in I) (O, error) { return fn(ctx, tx, in) })
		}
	case CommandFunc[I, O]:
		if fn == nil {
			return nil, false, nil
		}
		impl.cmd = func(ctx context.Context, c CommandContext, raw json.RawMessage) (json.RawMessage, error) {
			return typed(raw, func(in I) (O, error) { return fn(ctx, c, in) })
		}
	case QueryFunc[I, O]:
		if fn == nil {
			return nil, false, nil
		}
		impl.query = func(ctx context.Context, q QueryContext, raw json.RawMessage) (json.RawMessage, error) {
			return typed(raw, func(in I) (O, error) { return fn(ctx, q, in) })
		}
	default:
		return nil, false, serrors.New(serrors.Internal, fmt.Sprintf("native %s: unexpected handler %T", d.id, f))
	}
	return impl, true, nil
}

func typed[I, O any](raw json.RawMessage, fn func(I) (O, error)) (json.RawMessage, error) {
	var in I
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, Validation(Issue{Code: "runtime.invalidValue", Params: map[string]any{"message": err.Error()}})
		}
	}
	out, err := fn(in)
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

// Call runs the operation or atomic command joined to tx (tx.call): through
// the active implementation, its guards and records.
func (d *Def[I, O]) Call(ctx context.Context, tx Tx, in I) (O, error) {
	return roundTrip[I, O](in, func(raw json.RawMessage) (json.RawMessage, error) { return tx.Call(ctx, d.id, raw) })
}

// Query runs the query in r's snapshot or transaction.
func (d *Def[I, O]) Query(ctx context.Context, r Reader, in I) (O, error) {
	return roundTrip[I, O](in, func(raw json.RawMessage) (json.RawMessage, error) { return r.Query(ctx, d.id, raw) })
}

// Invoke runs the command as a new invocation from an ordinary command
// (ctx.call); it is refused inside a transaction.
func (d *Def[I, O]) Invoke(ctx context.Context, c CommandContext, in I) (O, error) {
	return roundTrip[I, O](in, func(raw json.RawMessage) (json.RawMessage, error) { return c.Call(ctx, d.id, raw) })
}

// Dispatch is the entry point of Go callers (controllers, services): a root
// invocation through the application's dispatcher.
func (d *Def[I, O]) Dispatch(ctx context.Context, disp Dispatcher, in I) (O, error) {
	return roundTrip[I, O](in, func(raw json.RawMessage) (json.RawMessage, error) { return disp.Dispatch(ctx, d.id, raw) })
}

func roundTrip[I, O any](in I, fn func(json.RawMessage) (json.RawMessage, error)) (O, error) {
	var out O
	raw, err := json.Marshal(in)
	if err != nil {
		return out, serrors.Wrap("extruntime.native.marshal", err)
	}
	res, err := fn(raw)
	if err != nil {
		return out, err
	}
	if len(res) > 0 {
		if err := json.Unmarshal(res, &out); err != nil {
			return out, serrors.Wrap("extruntime.native.unmarshal", err)
		}
	}
	return out, nil
}

// Implementation is a bound Go default implementation.
type Implementation struct {
	def   Definition
	tx    func(ctx context.Context, tx Tx, in json.RawMessage) (json.RawMessage, error)
	cmd   func(ctx context.Context, c CommandContext, in json.RawMessage) (json.RawMessage, error)
	query func(ctx context.Context, q QueryContext, in json.RawMessage) (json.RawMessage, error)
}

// Definition is the contract the implementation belongs to.
func (i *Implementation) Definition() Definition { return i.def }

// RunTx runs an operation or atomic command handler.
func (i *Implementation) RunTx(ctx context.Context, tx Tx, in json.RawMessage) (json.RawMessage, error) {
	if i.tx == nil {
		return nil, kindMismatch(i.def, "a transactional")
	}
	return guarded(i.def, func() (json.RawMessage, error) { return i.tx(ctx, tx, in) })
}

// RunCommand runs an ordinary command handler.
func (i *Implementation) RunCommand(ctx context.Context, c CommandContext, in json.RawMessage) (json.RawMessage, error) {
	if i.cmd == nil {
		return nil, kindMismatch(i.def, "an ordinary command")
	}
	return guarded(i.def, func() (json.RawMessage, error) { return i.cmd(ctx, c, in) })
}

// RunQuery runs a query handler.
func (i *Implementation) RunQuery(ctx context.Context, q QueryContext, in json.RawMessage) (json.RawMessage, error) {
	if i.query == nil {
		return nil, kindMismatch(i.def, "a query")
	}
	return guarded(i.def, func() (json.RawMessage, error) { return i.query(ctx, q, in) })
}

// guarded runs a handler and turns a panic of a Go default into an internal
// error, so the host still rolls back, records and releases what the
// invocation holds.
func guarded(d Definition, run func() (json.RawMessage, error)) (json.RawMessage, error) {
	var (
		out json.RawMessage
		err error
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				out = nil
				err = pool.Internal("runtime.nativePanic", fmt.Sprintf("%s panicked: %v", d.ID(), r))
			}
		}()
		out, err = run()
	}()
	return out, err
}

func kindMismatch(d Definition, want string) error {
	return serrors.New(serrors.Internal, fmt.Sprintf("native %s is a %s, not %s", d.ID(), d.Kind(), want))
}
