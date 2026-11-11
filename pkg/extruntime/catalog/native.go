// Package catalog implements spi.Catalog over the native (Go-registered)
// contracts and the manifests of activated package versions.
package catalog

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/build"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var nativeIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-zA-Z0-9_]+)+$`)

// NativeSet holds the contracts that Go registers: the native entity
// descriptors of the data layer plus native commands, operations, events,
// queries, pages and slots. They are public by contract; the Go
// implementations behind them are never exported.
type NativeSet struct {
	entities *data.NativeRegistry

	mu    sync.RWMutex
	extra map[string]manifest.Resource
	lists map[string]bool
}

// NewNativeSet wraps the native entity registry (may be nil).
func NewNativeSet(entities *data.NativeRegistry) *NativeSet {
	return &NativeSet{entities: entities, extra: map[string]manifest.Resource{}, lists: map[string]bool{}}
}

// Register adds a native contract. IDs are unique across entities and
// registered contracts.
func (n *NativeSet) Register(r manifest.Resource) error {
	const op = serrors.Op("extruntime.catalog.NativeSet.Register")
	if !nativeIDPattern.MatchString(r.ID) {
		return serrors.New(serrors.Invalid, fmt.Sprintf("native resource id %q is invalid", r.ID)).WithOp(op)
	}
	switch r.Kind {
	case manifest.ResEntity, manifest.ResOverlay:
		return serrors.New(serrors.Invalid, "native entities are registered through the data.NativeRegistry").WithOp(op)
	case "":
		return serrors.New(serrors.Invalid, "native resource "+r.ID+" has no kind").WithOp(op)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	if _, dup := n.extra[r.ID]; dup {
		return serrors.New(serrors.AlreadyExists, "native resource "+r.ID+" registered twice").WithOp(op)
	}
	if n.entities != nil {
		if _, dup := n.entities.Get(r.ID); dup {
			return serrors.New(serrors.AlreadyExists, "native resource "+r.ID+" registered twice").WithOp(op)
		}
	}
	r.ContractHash = manifest.ContractHashOf(r)
	n.extra[r.ID] = r
	return nil
}

// RegisterList declares a native list (for example "crm.lists.clients") that
// packages may extend with columns. It is published in the kit as a
// reference, `column(crm.Clients, ...)`.
func (n *NativeSet) RegisterList(id string) error {
	const op = serrors.Op("extruntime.catalog.NativeSet.RegisterList")
	if !nativeIDPattern.MatchString(id) {
		return serrors.New(serrors.Invalid, fmt.Sprintf("native list id %q is invalid", id)).WithOp(op)
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	n.lists[id] = true
	return nil
}

// Resources lists every native resource sorted by ID, with contract hashes.
func (n *NativeSet) Resources() []manifest.Resource {
	if n == nil {
		return nil
	}
	var out []manifest.Resource
	if n.entities != nil {
		for _, e := range n.entities.List() {
			r := e.Resource()
			r.ContractHash = manifest.ContractHashOf(r)
			out = append(out, r)
		}
	}
	n.mu.RLock()
	for _, r := range n.extra {
		out = append(out, r)
	}
	n.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get returns one native resource.
func (n *NativeSet) Get(id string) (manifest.Resource, bool) {
	if n == nil {
		return manifest.Resource{}, false
	}
	if n.entities != nil {
		if e, ok := n.entities.Get(id); ok {
			r := e.Resource()
			r.ContractHash = manifest.ContractHashOf(r)
			return r, true
		}
	}
	n.mu.RLock()
	defer n.mu.RUnlock()
	r, ok := n.extra[id]
	return r, ok
}

// Namespace is the kit namespace of a native resource: the first ID segment
// ("core.user" -> "core").
func Namespace(id string) string {
	ns, _, _ := strings.Cut(id, ".")
	return ns
}

// Sources groups the native resources by namespace for kit generation.
func (n *NativeSet) Sources() []build.KitSource {
	byNS := map[string][]manifest.Resource{}
	for _, r := range n.Resources() {
		ns := Namespace(r.ID)
		byNS[ns] = append(byNS[ns], r)
	}
	listsByNS := map[string][]string{}
	n.mu.RLock()
	for l := range n.lists {
		ns := Namespace(l)
		listsByNS[ns] = append(listsByNS[ns], l)
		if _, ok := byNS[ns]; !ok {
			byNS[ns] = nil
		}
	}
	n.mu.RUnlock()
	out := make([]build.KitSource, 0, len(byNS))
	for ns, res := range byNS {
		lists := listsByNS[ns]
		sort.Strings(lists)
		out = append(out, build.KitSource{Namespace: ns, Resources: res, Lists: lists})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Namespace < out[j].Namespace })
	return out
}

// Reserved reports whether a package namespace lies under a native root
// ("core", "crm"): packages may not declare resources there.
func (n *NativeSet) Reserved(namespace string) bool {
	root := Namespace(namespace)
	for _, r := range n.Resources() {
		if Namespace(r.ID) == root {
			return true
		}
	}
	return false
}

// DefaultNatives builds the native set an application publishes: the native
// entity descriptors, the native events (events.Default), the Go-implemented
// commands, operations and queries (contracts; nil: native.Default), extra
// contracts (native routes, application-specific resources) and the native
// lists (empty: the CRM clients list).
func DefaultNatives(entities *data.NativeRegistry, contracts *native.Set, lists []string, extra ...manifest.Resource) (*NativeSet, error) {
	const op = serrors.Op("extruntime.catalog.DefaultNatives")
	if contracts == nil {
		contracts = native.Default
	}
	set := NewNativeSet(entities)
	all := append([]manifest.Resource{}, events.Default.Resources()...)
	all = append(all, contracts.Resources()...)
	all = append(all, extra...)
	for _, r := range all {
		if err := set.Register(r); err != nil {
			return nil, serrors.Wrap(op, err)
		}
	}
	if len(lists) == 0 {
		lists = []string{data.NativeClientList}
	}
	for _, l := range lists {
		if err := set.RegisterList(l); err != nil {
			return nil, serrors.Wrap(op, err)
		}
	}
	return set, nil
}

// Defaults is every native resource of the process without application
// configuration: data.DefaultNatives entities, native events, the contracts
// of native.Default (declared by the modules linked into the binary) and the
// default native lists. An offline kit is generated from
// Defaults().Sources(); a running application adds its configured native
// routes and extra contracts (runtime.Config).
func Defaults() (*NativeSet, error) {
	return DefaultNatives(data.DefaultNatives(), nil, nil)
}
