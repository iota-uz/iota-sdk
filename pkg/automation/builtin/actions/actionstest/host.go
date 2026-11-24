// Package actionstest builds the command host of a native set for tests of automation actions: the
// host the extension runtime's wiring builds without Node (natives run their Go defaults).
package actionstest

import (
	"context"
	"errors"
	"reflect"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
)

type noBindings struct{}

func (noBindings) Resolve(context.Context, string) (host.Binding, error) {
	return host.Binding{}, host.ErrNoBinding
}

// Host is the command host of set over db: native commands run their Go defaults with deps; nativeCtx
// (optional) is host.Config.NativeContext. It returns the host and its catalog (workflow.Config.Catalog).
func Host(db *pgxpool.Pool, set *native.Set, deps native.Deps, nativeCtx func(context.Context, host.NativeInvocation) context.Context) (*host.Host, *catalog.Catalog, error) {
	cat := catalog.New(db, catalog.Natives(set))
	h, err := host.New(host.Config{
		DB: db, Bindings: noBindings{}, Catalog: cat, Data: data.NewStore(cat, set.EntityRegistry(), nil),
		Natives: native.Bind(set, deps), NativeContext: nativeCtx,
	})
	return h, cat, err
}

// NoDeps resolves nothing: actions that need no dependencies.
type NoDeps struct{}

// ResolveType implements native.Deps.
func (NoDeps) ResolveType(t reflect.Type) (any, error) {
	return nil, errors.New("actionstest: no dependency " + t.String())
}
