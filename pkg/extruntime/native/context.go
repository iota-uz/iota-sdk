package native

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Reader runs queries in the snapshot or transaction of its context.
type Reader interface {
	// Query runs a query contract (native or package) with the caller's
	// rights, reading through the caller's snapshot or transaction.
	Query(ctx context.Context, query string, input json.RawMessage) (json.RawMessage, error)
}

// Tx is the Go view of a host-owned transaction: the handler of an operation
// or atomic command, or the callback of CommandContext.Transaction.
type Tx interface {
	Reader
	// DB is the transaction. The handler's ctx carries it too
	// (composables.UseTx), so repositories join it.
	DB() pgx.Tx
	// Call runs an operation or atomic command joined to this transaction
	// through its active implementation (tx.call). Ordinary commands are
	// refused.
	Call(ctx context.Context, contract string, input json.RawMessage) (json.RawMessage, error)
	// AfterCommit registers fn to run after the transaction commits (never
	// after a rollback). It fails in a transaction owned by the caller of the
	// runtime (a subscription delivery), whose commit the host cannot see.
	AfterCommit(fn func(ctx context.Context)) error
	// Authorize fails with a denied error unless the acting subject holds the
	// RBAC permission.
	Authorize(permission string) error
}

// QueryContext is the read-only context of a query handler.
type QueryContext interface {
	Reader
	// DB is the snapshot (or the caller's transaction for tx.query). Query
	// handlers must not write.
	DB() pgx.Tx
	Authorize(permission string) error
}

// CommandContext is the context of an ordinary command handler.
type CommandContext interface {
	Reader
	// Transaction runs fn in a new transaction and commits it when fn
	// returns nil. Transactions are not nested.
	Transaction(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error
	// Call runs a command as a new invocation (ctx.call). It is refused while
	// a transaction is open.
	Call(ctx context.Context, command string, input json.RawMessage) (json.RawMessage, error)
	Authorize(permission string) error
}

// Dispatcher is the single entry point of native contracts for Go code
// (controllers, services, jobs): every published entry of a native command
// goes through it, so a package replacement and the guards of packages apply
// to Go callers too.
type Dispatcher interface {
	Dispatch(ctx context.Context, contract string, input json.RawMessage) (json.RawMessage, error)
}

// Implementations binds the Go defaults of a contract set to one
// application's dependencies. Each default is instantiated on first use.
type Implementations struct {
	set  *Set
	deps Deps

	mu    sync.Mutex
	bound map[string]*bound
}

type bound struct {
	once sync.Once
	impl *Implementation
	has  bool
	err  error
}

// Bind binds the Go defaults of the contracts of set to deps (nil set: no
// contracts).
func Bind(set *Set, deps Deps) *Implementations {
	if set == nil {
		set = NewSet()
	}
	return &Implementations{set: set, deps: deps, bound: map[string]*bound{}}
}

// Set is the contract set the implementations belong to.
func (i *Implementations) Set() *Set { return i.set }

// Definition returns the native contract with id.
func (i *Implementations) Definition(id string) (Definition, bool) {
	if i == nil {
		return nil, false
	}
	return i.set.Contract(id)
}

// Lookup returns the Go default of id; ok is false when id is not a native
// contract or declares no Go default.
func (i *Implementations) Lookup(id string) (*Implementation, bool, error) {
	if i == nil {
		return nil, false, nil
	}
	d, ok := i.set.Contract(id)
	if !ok || !d.HasDefault() {
		return nil, false, nil
	}
	i.mu.Lock()
	b := i.bound[id]
	if b == nil {
		b = &bound{}
		i.bound[id] = b
	}
	i.mu.Unlock()
	b.once.Do(func() { b.impl, b.has, b.err = d.bind(i.deps) })
	if b.err != nil {
		return nil, false, b.err
	}
	return b.impl, b.has, nil
}

// ErrNoImplementation is returned for a native contract without a Go default
// when nothing else implements it.
var ErrNoImplementation = serrors.New(serrors.NotFound, "native contract has no implementation")

// StaticDeps resolves dependencies from fixed values (the first value
// assignable to the requested type), for callers without a container.
func StaticDeps(values ...any) Deps { return staticDeps(values) }

type staticDeps []any

func (s staticDeps) ResolveType(t reflect.Type) (any, error) {
	for _, v := range s {
		if v != nil && reflect.TypeOf(v).AssignableTo(t) {
			return v, nil
		}
	}
	return nil, serrors.New(serrors.NotFound, "native: no dependency of type "+t.String())
}
