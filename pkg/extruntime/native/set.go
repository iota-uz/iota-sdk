package native

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Set is the one value that carries every native (Go-owned) resource of an
// application: the native entities the data store reads, the native lists
// packages extend with columns, the native events with their versioned
// payload types, the Go-implemented operations, commands and queries with
// their Go defaults, the native routes a screen binding may replace, and any
// other plain declaration (slots, pages, permissions, ...).
//
// Each module contributes its resources from its registration code (a
// Contributor); the application composes the contributions once (Compose)
// and hands the Set to the extension runtime (runtime.Config.Natives). The
// catalog, the developer kit (cmd/extruntime-catalog), the data store, the
// UI and the invocation host all read this value; there is no process-wide
// registry. A Set only grows: resources cannot be removed or replaced.
type Set struct {
	mu        sync.RWMutex
	kinds     map[string]string
	entities  map[string]data.NativeEntity
	contracts map[string]Definition
	events    map[string]EventDefinition
	lists     map[string]struct{}
	routes    map[string]string
	routeSpec map[string]RouteSpec
	declared  map[string]manifest.Resource
}

// Item is one native resource a module contributes: a contract (*Def), an
// event (*EventDef), Entity, List, Route or Declare.
type Item interface {
	addTo(s *Set) error
}

// Contributor is the native registration of one module: it adds the module's
// resources to the application's Set.
type Contributor func(s *Set) error

// NewSet returns an empty Set.
func NewSet() *Set {
	return &Set{
		kinds: map[string]string{}, entities: map[string]data.NativeEntity{}, contracts: map[string]Definition{},
		events: map[string]EventDefinition{}, lists: map[string]struct{}{}, routes: map[string]string{}, routeSpec: map[string]RouteSpec{},
		declared: map[string]manifest.Resource{},
	}
}

// Compose builds the Set of an application from the registrations of its
// modules, in order.
func Compose(contributors ...Contributor) (*Set, error) {
	s := NewSet()
	for _, c := range contributors {
		if c == nil {
			continue
		}
		if err := c(s); err != nil {
			return nil, serrors.Wrap("extruntime.native.Compose", err)
		}
	}
	return s, nil
}

// Add adds resources. An ID is unique across every kind; adding the same
// contract or event value twice is a no-op, a different resource under a
// taken ID is an error. On error nothing after the failing item is added.
func (s *Set) Add(items ...Item) error {
	for _, it := range items {
		if it == nil {
			continue
		}
		if err := it.addTo(s); err != nil {
			return serrors.Wrap("extruntime.native.Set.Add", err)
		}
	}
	return nil
}

var setIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-zA-Z0-9_]+)+$`)

// claim reserves id for kind; same reports an identical re-registration.
// Callers hold s.mu.
func (s *Set) claim(id, kind string, same func() bool) (bool, error) {
	if !setIDPattern.MatchString(id) {
		return false, serrors.New(serrors.Invalid, fmt.Sprintf("native %s id %q is invalid", kind, id))
	}
	prev, taken := s.kinds[id]
	if !taken {
		s.kinds[id] = kind
		return false, nil
	}
	if prev == kind && same != nil && same() {
		return true, nil
	}
	return false, serrors.New(serrors.AlreadyExists, fmt.Sprintf("native resource %s registered twice (%s, %s)", id, prev, kind))
}

func (d *Def[I, O]) addTo(s *Set) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dup, err := s.claim(d.id, "contract", func() bool { return s.contracts[d.id] == Definition(d) })
	if err != nil || dup {
		return err
	}
	s.contracts[d.id] = d
	return nil
}

type entityItem struct{ e data.NativeEntity }

// Entity contributes a native entity: a table packages read through the data
// store (writes go through published operations).
func Entity(e data.NativeEntity) Item { return entityItem{e} }

func (it entityItem) addTo(s *Set) error {
	if err := data.NewNativeRegistry().Register(it.e); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.claim(it.e.ID, "entity", nil); err != nil {
		return err
	}
	s.entities[it.e.ID] = it.e
	return nil
}

type listItem string

// List contributes a native list (for example "crm.lists.clients") packages
// may extend with columns. It is published in the kit of the namespace the
// list ID extends.
func List(id string) Item { return listItem(id) }

func (it listItem) addTo(s *Set) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.claim(string(it), "list", nil); err != nil {
		return err
	}
	s.lists[string(it)] = struct{}{}
	return nil
}

// RouteSpec types a native route beyond its path.
type RouteSpec struct {
	// Params types the path parameters (an object with one property per
	// ":name" segment). A package page for the route gets them decoded.
	Params *manifest.Type
	// Permission is the RBAC permission the native screen requires; a package
	// page replacing the route inherits it.
	Permission string
}

type routeItem struct {
	id, path string
	spec     RouteSpec
}

// RouteWith is Route for a page with typed path parameters and a permission.
func RouteWith(id, path string, spec RouteSpec) Item { return routeItem{id, path, spec} }

// Route contributes a native page route: published as a route contract (so
// packages can link to it and a screen binding can replace the page) and
// handed to the UI slice as a replaceable native route.
func Route(id, path string) Item { return routeItem{id: id, path: path} }

func (it routeItem) addTo(s *Set) error {
	if !strings.HasPrefix(it.path, "/") {
		return serrors.New(serrors.Invalid, fmt.Sprintf("native route %s: path %q must start with /", it.id, it.path))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.claim(it.id, "route", nil); err != nil {
		return err
	}
	s.routes[it.id] = it.path
	s.routeSpec[it.id] = it.spec
	return nil
}

type declItem struct{ r manifest.Resource }

// Declare contributes any other public declaration as a plain catalog
// resource: a slot (manifest.ResSlot with props), a page, a permission, a
// navigation node, an issue or a domain error. Entities, events, contracts,
// lists and routes have their own items, which carry what their consumers
// need besides the resource.
func Declare(r manifest.Resource) Item { return declItem{r} }

func (it declItem) addTo(s *Set) error {
	switch it.r.Kind {
	case "":
		return serrors.New(serrors.Invalid, "native resource "+it.r.ID+" has no kind")
	case manifest.ResEntity, manifest.ResOverlay, manifest.ResEvent, manifest.ResCommand,
		manifest.ResOperation, manifest.ResQuery, manifest.ResRoute:
		return serrors.New(serrors.Invalid, fmt.Sprintf("native %s %s: use native.%s", it.r.Kind, it.r.ID, itemFor(it.r.Kind)))
	case manifest.ResWorkflow, manifest.ResNotice, manifest.ResPermission, manifest.ResIssue,
		manifest.ResDomainError, manifest.ResColumn, manifest.ResPage, manifest.ResNavigation,
		manifest.ResSlot, manifest.ResCell, manifest.ResShell, manifest.ResContactResolver,
		manifest.ResDeliveryPolicy:
		// plain declarations
	}
	r := cloneResource(it.r)
	r.ContractHash = manifest.ContractHashOf(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.claim(r.ID, "declaration", nil); err != nil {
		return err
	}
	s.declared[r.ID] = r
	return nil
}

func itemFor(k manifest.ResourceKind) string {
	if k == manifest.ResEntity || k == manifest.ResOverlay {
		return "Entity"
	}
	if k == manifest.ResEvent {
		return "Event"
	}
	if k == manifest.ResRoute {
		return "Route"
	}
	return "Operation/Command/Query"
}

// Contract returns a Go-implemented contract (operation, command, query).
func (s *Set) Contract(id string) (Definition, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.contracts[id]
	return d, ok
}

// Contracts lists the contracts sorted by ID.
func (s *Set) Contracts() []Definition {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	out := slices.Collect(maps.Values(s.contracts))
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// Event returns a native event declaration.
func (s *Set) Event(id string) (EventDefinition, bool) {
	if s == nil {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.events[id]
	return e, ok
}

// Entities lists the native entities sorted by ID.
func (s *Set) Entities() []data.NativeEntity {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	out := slices.Collect(maps.Values(s.entities))
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// EntityRegistry is the data layer's index of the native entities (the data
// store, the schema compiler and the management service take it).
func (s *Set) EntityRegistry() *data.NativeRegistry {
	return data.NewNativeRegistry(s.Entities()...)
}

// Lists lists the native list IDs, sorted.
func (s *Set) Lists() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	out := slices.Collect(maps.Keys(s.lists))
	s.mu.RUnlock()
	sort.Strings(out)
	return out
}

// Routes maps every native route ID to its path pattern.
func (s *Set) Routes() map[string]string {
	if s == nil {
		return map[string]string{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return maps.Clone(s.routes)
}

// Resources lists every published resource sorted by ID, with contract
// hashes: entities, events, contracts, routes and declarations. Lists are
// kit references, not resources (Lists).
func (s *Set) Resources() []manifest.Resource {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	out := make([]manifest.Resource, 0, len(s.kinds))
	for _, e := range s.entities {
		r := e.Resource()
		r.ContractHash = manifest.ContractHashOf(r)
		out = append(out, r)
	}
	for _, e := range s.events {
		out = append(out, e.Resource())
	}
	for _, d := range s.contracts {
		out = append(out, d.Resource())
	}
	for id, path := range s.routes {
		spec := s.routeSpec[id]
		r := manifest.Resource{ID: id, Kind: manifest.ResRoute, Path: path, Params: spec.Params, Permission: spec.Permission}
		r.ContractHash = manifest.ContractHashOf(r)
		out = append(out, r)
	}
	for _, r := range s.declared {
		out = append(out, cloneResource(r))
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Resource returns one published resource.
func (s *Set) Resource(id string) (manifest.Resource, bool) {
	if s == nil {
		return manifest.Resource{}, false
	}
	s.mu.RLock()
	kind, ok := s.kinds[id]
	s.mu.RUnlock()
	if !ok || kind == "list" {
		return manifest.Resource{}, false
	}
	for _, r := range s.Resources() {
		if r.ID == id {
			return r, true
		}
	}
	return manifest.Resource{}, false
}

// ResourcesOf lists the published resources of one kind (for example the
// slot declarations, manifest.ResSlot), sorted by ID.
func (s *Set) ResourcesOf(kind manifest.ResourceKind) []manifest.Resource {
	var out []manifest.Resource
	for _, r := range s.Resources() {
		if r.Kind == kind {
			out = append(out, r)
		}
	}
	return out
}
