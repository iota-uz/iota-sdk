package catalog

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Registry collects node types, trigger sources and connectors and builds the catalog of a tenant.
// Registration happens at composition; a duplicate node type, event key or connector kind is an error.
type Registry interface {
	RegisterNode(NodeType) error
	RegisterTriggers(TriggerSource) error
	RegisterConnector(ConnectorContribution) error
	// ForTenant is the catalog of a tenant. Without AsAuthor it is the system view (the engine, the
	// check when a graph is enabled): nothing is hidden by permission and payloads are whole.
	ForTenant(ctx context.Context, tenantID uuid.UUID, opts ...ViewOption) (View, error)
	// Node is the executable node of a type, whatever the availability.
	Node(nodeType string) (NodeType, bool)
}

// RegistryOption configures a registry.
type RegistryOption func(*registry)

// WithBehaviors is the behavior set the catalog is checked and viewed with (default: graph's).
func WithBehaviors(b *graph.Behaviors) RegistryOption {
	return func(r *registry) { r.behaviors = b }
}

// WithSubject declares the expression context of a subject entity. An entity events or nodes name and
// no option declares has an empty context.
func WithSubject(entity string, s graph.SubjectSpec) RegistryOption {
	return func(r *registry) { r.subjects[entity] = s }
}

// WithMixedSubject sets graph.Catalog.MixedSubject.
func WithMixedSubject(entity string) RegistryOption {
	return func(r *registry) { r.mixed = entity }
}

// ViewOption configures ForTenant.
type ViewOption func(*viewOptions)

type viewOptions struct {
	author *spi.Subject
	locale string
}

// AsAuthor builds the catalog an author sees: node types and events without the author's permission
// are hidden and sensitive payload fields the author may not read are removed from the schema.
func AsAuthor(s spi.Subject) ViewOption {
	return func(o *viewOptions) { o.author = &s }
}

// WithLocale localizes titles and descriptions of nodes and events that support it.
func WithLocale(locale string) ViewOption {
	return func(o *viewOptions) { o.locale = locale }
}

// View is the catalog of a tenant.
type View struct {
	Catalog graph.Catalog
	// Unavailable lists node types and event keys left out, with the reason
	// ("connection_unavailable", "permission").
	Unavailable map[string]string
	// UnavailableEvents and UnavailableNodes are the same by namespace: an event and a node type may share a
	// key, and the reason of one is not the other's. Unavailable holds both (a node type's reason wins a
	// shared key); a rule that is about one of them reads its own map.
	UnavailableEvents map[string]string
	UnavailableNodes  map[string]string
	// NodeRequires and EventRequires are the connector kinds each node type and event of the catalog
	// works with (any one of them).
	NodeRequires  map[string][]ConnectorKind
	EventRequires map[string][]ConnectorKind
	// Connections are the tenant's working connections by kind, preferred first.
	Connections map[ConnectorKind][]ConnectionRef
	// TenantOnlyEvents are the events (TenantConnectionTriggers) and TenantOnlyNodes the node types
	// (TenantConnectionNode) that only the tenant's own connections serve; their keys are apart, since an
	// event and a node type may share one.
	TenantOnlyEvents map[string]bool
	TenantOnlyNodes  map[string]bool
}

// OwnConnections are the refs of connected that are the tenant's own connections: no platform credential.
func OwnConnections(connected map[ConnectorKind][]ConnectionRef) map[ConnectorKind][]ConnectionRef {
	return ownRefs(connected)
}

// ownRefs are the refs of connected that are the tenant's own connections: no platform credential.
func ownRefs(connected map[ConnectorKind][]ConnectionRef) map[ConnectorKind][]ConnectionRef {
	out := make(map[ConnectorKind][]ConnectionRef, len(connected))
	for k, refs := range connected {
		for _, r := range refs {
			if !strings.HasPrefix(r.ID, PlatformPrefix) {
				out[k] = append(out[k], r)
			}
		}
	}
	return out
}

// JSON is the view for the editor and the API: graph.View with NodeView.Requires filled.
func (v View) JSON(b *graph.Behaviors) graph.CatalogView {
	cv := graph.View(v.Catalog, b)
	for i := range cv.Nodes {
		kinds := v.NodeRequires[cv.Nodes[i].Type]
		if len(kinds) == 0 {
			continue
		}
		req := make([]string, len(kinds))
		for j, k := range kinds {
			req[j] = string(k)
		}
		cv.Nodes[i].Requires = req
	}
	return cv
}

type nodeEntry struct {
	node     NodeType
	requires []ConnectorKind
}

type eventEntry struct {
	source   TriggerSource
	spec     graph.EventSpec
	requires []ConnectorKind
}

type registry struct {
	mu         sync.RWMutex
	avail      Availability
	behaviors  *graph.Behaviors
	subjects   map[string]graph.SubjectSpec
	mixed      string
	nodes      map[string]nodeEntry
	events     map[string]eventEntry
	connectors map[ConnectorKind]bool
}

// NewRegistry returns an empty registry over the tenants' availability (nil: no connector is ever
// connected).
func NewRegistry(a Availability, o ...RegistryOption) Registry {
	r := &registry{
		avail: a, subjects: map[string]graph.SubjectSpec{}, nodes: map[string]nodeEntry{},
		events: map[string]eventEntry{}, connectors: map[ConnectorKind]bool{},
	}
	for _, opt := range o {
		opt(r)
	}
	if r.behaviors == nil {
		r.behaviors = graph.DefaultBehaviors()
	}
	return r
}

func (r *registry) RegisterNode(n NodeType) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.checkNode(n, map[string]bool{}); err != nil {
		return serrors.Wrap("automation.catalog.RegisterNode", err)
	}
	r.addNode(n, "")
	return nil
}

func (r *registry) RegisterTriggers(s TriggerSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.checkTriggers(s, map[string]bool{}); err != nil {
		return serrors.Wrap("automation.catalog.RegisterTriggers", err)
	}
	r.addTriggers(s, "")
	return nil
}

func (r *registry) RegisterConnector(c ConnectorContribution) error {
	const op = serrors.Op("automation.catalog.RegisterConnector")
	if c == nil {
		return serrors.New(serrors.Invalid, "a nil connector").WithOp(op)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	kind := c.Kind()
	if kind == "" {
		return serrors.New(serrors.Invalid, "a connector without a kind").WithOp(op)
	}
	if r.connectors[kind] {
		return serrors.New(serrors.Invalid, fmt.Sprintf("connector %s is registered twice", kind)).WithOp(op)
	}
	// Check everything first, so a failing connector adds nothing.
	nodes, sources := c.Nodes(), c.Triggers()
	seenNodes, seenEvents := map[string]bool{}, map[string]bool{}
	for _, n := range nodes {
		if err := r.checkNode(n, seenNodes); err != nil {
			return serrors.Wrap(op, fmt.Errorf("connector %s: %w", kind, err))
		}
	}
	for _, s := range sources {
		if err := r.checkTriggers(s, seenEvents); err != nil {
			return serrors.Wrap(op, fmt.Errorf("connector %s: %w", kind, err))
		}
	}
	r.connectors[kind] = true
	for _, n := range nodes {
		r.addNode(n, kind)
	}
	for _, s := range sources {
		r.addTriggers(s, kind)
	}
	return nil
}

func (r *registry) checkNode(n NodeType, seen map[string]bool) error {
	if n == nil {
		return fmt.Errorf("a nil node type")
	}
	spec := n.Spec()
	typ := spec.Type
	switch {
	case typ == "":
		return fmt.Errorf("a node type without a type")
	case r.nodes[typ].node != nil || seen[typ]:
		return fmt.Errorf("node type %s is registered twice", typ)
	case spec.Params == nil:
		return fmt.Errorf("node type %s: no params spec", typ)
	}
	if !slices.Contains(r.behaviors.Supported(), spec.Behavior) {
		return fmt.Errorf("node type %s: behavior %q is not supported", typ, spec.Behavior)
	}
	if declaresConnection(spec.Params) {
		return fmt.Errorf("node type %s: %s is a reserved param", typ, ConnectionParam)
	}
	if err := graph.CheckDryRun(spec); err != nil {
		return err
	}
	if err := spec.Output.Validate(); err != nil {
		return fmt.Errorf("node type %s: output: %w", typ, err)
	}
	seen[typ] = true
	return nil
}

func (r *registry) checkTriggers(s TriggerSource, seen map[string]bool) error {
	if s == nil {
		return fmt.Errorf("a nil trigger source")
	}
	if f, ok := s.(failing); ok {
		if err := f.buildError(); err != nil {
			return err
		}
	}
	for _, e := range s.Events() {
		switch {
		case e.Key == "":
			return fmt.Errorf("an event without a key")
		case r.events[e.Key].source != nil || seen[e.Key]:
			return fmt.Errorf("event %s is registered twice", e.Key)
		}
		if err := e.Payload.Validate(); err != nil {
			return fmt.Errorf("event %s: payload: %w", e.Key, err)
		}
		if _, ok := e.Payload.Get("kind"); ok {
			return fmt.Errorf("event %s: payload: kind is the event's own field", e.Key)
		}
		// by_self is the engine's own field of inbound events (an optional bool it sets): a source may
		// declare it only as that.
		if f, ok := e.Payload.Get(BySelfField); ok && f.String() != expr.OptionalOf(expr.BoolField).String() {
			return fmt.Errorf("event %s: payload: %s is the engine's optional bool of inbound events", e.Key, BySelfField)
		}
		if declaresConnection(e.TriggerParams) {
			return fmt.Errorf("event %s: %s is a reserved param", e.Key, ConnectionParam)
		}
		if _, ok := e.Match[ConnectionParam]; ok {
			return fmt.Errorf("event %s: %s is a reserved param", e.Key, ConnectionParam)
		}
		if pt, ok := s.(PermissionedTriggers); ok {
			if access, ok := pt.Access(e.Key); ok {
				for path := range access.Sensitive {
					if err := sensitivePath(e.Payload, path); err != nil {
						return fmt.Errorf("event %s: sensitive %s: %w", e.Key, path, err)
					}
				}
			}
		}
		seen[e.Key] = true
	}
	return nil
}

func (r *registry) addNode(n NodeType, kind ConnectorKind) {
	r.nodes[n.Spec().Type] = nodeEntry{node: n, requires: requirements(n.Requires(), kind)}
}

func (r *registry) addTriggers(s TriggerSource, kind ConnectorKind) {
	req := requirements(s.Requires(), kind)
	for _, e := range s.Events() {
		if len(req) > 0 {
			e = withBySelf(e)
		}
		r.events[e.Key] = eventEntry{source: s, spec: e, requires: req}
	}
}

// BySelfField is the payload field the engine sets on an inbound event of a connector: true for a
// message the connector sent itself (its own bot, its own mailbox).
const BySelfField = "by_self"

// withBySelf declares by_self on an inbound event that does not declare it.
func withBySelf(e graph.EventSpec) graph.EventSpec {
	if _, ok := e.Payload.Get(BySelfField); ok {
		return e
	}
	e.Payload = append(e.Payload.Clone(), expr.Prop{Name: BySelfField, Field: expr.OptionalOf(expr.BoolField)})
	return e
}

// requirements are the declared kinds and the contributing connector's kind, each once.
func requirements(declared []ConnectorKind, kind ConnectorKind) []ConnectorKind {
	out := make([]ConnectorKind, 0, len(declared)+1)
	seen := map[ConnectorKind]bool{}
	for _, k := range append(append([]ConnectorKind{}, declared...), kind) {
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

func (r *registry) Node(nodeType string) (NodeType, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.nodes[nodeType]
	return e.node, ok
}

func (r *registry) ForTenant(ctx context.Context, tenantID uuid.UUID, opts ...ViewOption) (View, error) {
	const op = serrors.Op("automation.catalog.ForTenant")
	var o viewOptions
	for _, opt := range opts {
		opt(&o)
	}
	connected := map[ConnectorKind][]ConnectionRef{}
	if r.avail != nil {
		c, err := r.avail.Connected(ctx, tenantID)
		if err != nil {
			return View{}, serrors.Wrap(op, err)
		}
		for k, refs := range c {
			if len(refs) > 0 {
				connected[k] = append([]ConnectionRef(nil), refs...)
			}
		}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	v := View{
		Catalog: graph.Catalog{
			Events: map[string]graph.EventSpec{}, Nodes: map[string]graph.NodeSpec{},
			Subjects: map[string]graph.SubjectSpec{}, MixedSubject: r.mixed,
		},
		Unavailable:       map[string]string{},
		UnavailableEvents: map[string]string{},
		UnavailableNodes:  map[string]string{},
		NodeRequires:      map[string][]ConnectorKind{},
		EventRequires:     map[string][]ConnectorKind{},
		Connections:       connected,
		TenantOnlyEvents:  map[string]bool{},
		TenantOnlyNodes:   map[string]bool{},
	}
	own := ownRefs(connected)
	reason := func(requires []ConnectorKind, permission string, from map[ConnectorKind][]ConnectionRef) string {
		if o.author != nil && !o.author.Has(permission) {
			return ReasonPermission
		}
		if len(requires) == 0 {
			return ""
		}
		for _, k := range requires {
			if len(from[k]) > 0 {
				return ""
			}
		}
		return ReasonConnectionUnavailable
	}

	for key, e := range r.events {
		var access EventAccess
		if pt, ok := e.source.(PermissionedTriggers); ok {
			access, _ = pt.Access(key)
		}
		from := connected
		if tc, ok := e.source.(TenantConnectionTriggers); ok && tc.TenantConnectionOnly(key) {
			from = own
			v.TenantOnlyEvents[key] = true
		}
		if why := reason(e.requires, access.ReadPermission, from); why != "" {
			v.Unavailable[key] = why
			v.UnavailableEvents[key] = why
			delete(v.TenantOnlyEvents, key)
			continue
		}
		spec := copyEvent(e.spec)
		if lt, ok := e.source.(LocalizedTriggers); ok && o.locale != "" {
			spec = lt.Localize(spec, o.locale)
			spec.Key = key
		}
		if o.author != nil {
			var hidden []string
			spec.Payload, hidden = project(spec.Payload, access.Sensitive, *o.author)
			dropMatches(&spec, hidden)
		}
		if len(e.requires) > 0 {
			base := spec.TriggerParams
			if base == nil && len(spec.Match) > 0 {
				p, err := graph.MatchParams(spec.Match)
				if err != nil {
					return View{}, serrors.Wrap(op, fmt.Errorf("event %s: %w", key, err))
				}
				base = p
			}
			spec.TriggerParams = withConnectionParam(base)
			v.EventRequires[key] = append([]ConnectorKind(nil), e.requires...)
		}
		v.Catalog.Events[key] = spec
	}

	for typ, e := range r.nodes {
		perm := ""
		if pn, ok := e.node.(PermissionedNode); ok {
			perm = pn.Permission()
			if perm == "" {
				perm = "\x00" // a permissioned node without a permission is nobody's
			}
		}
		if o.author == nil {
			perm = ""
		}
		from := connected
		if tc, ok := e.node.(TenantConnectionNode); ok && tc.TenantConnectionOnly() {
			from = own
			v.TenantOnlyNodes[typ] = true
		}
		if why := reason(e.requires, perm, from); why != "" {
			v.Unavailable[typ] = why
			v.UnavailableNodes[typ] = why
			delete(v.TenantOnlyNodes, typ)
			continue
		}
		spec := e.node.Spec()
		if ln, ok := e.node.(LocalizedNode); ok && o.locale != "" {
			spec = ln.Localize(spec, o.locale)
		}
		spec.Type = typ
		if len(e.requires) > 0 {
			spec.Params = withConnectionParam(spec.Params)
			v.NodeRequires[typ] = append([]ConnectorKind(nil), e.requires...)
		}
		v.Catalog.Nodes[typ] = spec
	}

	// A node that waits for an event the tenant cannot use is unavailable for the same reason.
	for _, typ := range sortedKeys(v.Catalog.Nodes) {
		n := v.Catalog.Nodes[typ]
		if n.Wait == nil || n.Wait.Event.Value == "" {
			continue
		}
		if _, ok := v.Catalog.Events[n.Wait.Event.Value]; ok {
			continue
		}
		why := v.UnavailableEvents[n.Wait.Event.Value]
		if why == "" {
			why = ReasonConnectionUnavailable
		}
		v.Unavailable[typ] = why
		v.UnavailableNodes[typ] = why
		delete(v.Catalog.Nodes, typ)
		delete(v.NodeRequires, typ)
	}

	for entity, s := range r.subjects {
		v.Catalog.Subjects[entity] = s
	}
	addSubject := func(entity string) {
		if entity == "" || entity == graph.SubjectConversation || entity == graph.SubjectNone {
			return
		}
		if _, ok := v.Catalog.Subjects[entity]; !ok {
			v.Catalog.Subjects[entity] = graph.SubjectSpec{}
		}
	}
	for _, e := range v.Catalog.Events {
		addSubject(e.Subject)
	}
	for _, n := range v.Catalog.Nodes {
		for _, s := range n.Subjects {
			addSubject(s)
		}
	}
	addSubject(r.mixed)

	if err := v.Catalog.Check(r.behaviors); err != nil {
		return View{}, serrors.Wrap(op, err)
	}
	v.Catalog.Revision = graph.Revision(v.Catalog, r.behaviors)
	return v, nil
}

func copyEvent(e graph.EventSpec) graph.EventSpec {
	e.Payload = e.Payload.Clone()
	if e.Match != nil {
		m := make(map[string]graph.MatchField, len(e.Match))
		for k, f := range e.Match {
			m[k] = f
		}
		e.Match = m
	}
	return e
}

// sensitivePath checks that a sensitive path names a field of the payload and that no dyn field is on
// the way: a field inside dyn cannot be removed from the schema, so the projection would not hide it.
func sensitivePath(payload expr.Fields, path string) error {
	parts := strings.Split(path, ".")
	f := expr.ObjectOf(payload)
	for i, part := range parts {
		for f.Kind == expr.Optional || f.Kind == expr.List || f.Kind == expr.Map {
			if f.Elem == nil {
				return fmt.Errorf("not a field of the payload")
			}
			f = *f.Elem
		}
		if f.Kind == expr.Dyn {
			return fmt.Errorf("%s is dyn: a field inside it cannot be hidden", strings.Join(parts[:i], "."))
		}
		next, ok := f.Prop(part)
		if !ok {
			return fmt.Errorf("not a field of the payload")
		}
		f = next
	}
	return nil
}

// dropMatches removes match fields that read a hidden payload field: matching by it would reveal it.
func dropMatches(spec *graph.EventSpec, hidden []string) {
	for name, m := range spec.Match {
		for _, h := range hidden {
			if m.Payload == h || strings.HasPrefix(m.Payload, h+".") {
				delete(spec.Match, name)
				break
			}
		}
	}
	if len(spec.Match) == 0 && spec.Match != nil {
		spec.Match = nil
	}
}

// project removes from a payload the sensitive fields the author may not read (a field needs all of
// its permissions) and returns the hidden paths.
func project(payload expr.Fields, sensitive map[string][]string, author spi.Subject) (expr.Fields, []string) {
	var hidden []string
	paths := make([]string, 0, len(sensitive))
	for p := range sensitive {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		allowed := true
		for _, perm := range sensitive[p] {
			if !author.Has(perm) {
				allowed = false
				break
			}
		}
		if !allowed {
			payload = removePath(payload, strings.Split(p, "."))
			hidden = append(hidden, p)
		}
	}
	return payload, hidden
}

// removePath removes the field at a dotted path; objects are looked into through optionals and the
// items of lists.
func removePath(fs expr.Fields, path []string) expr.Fields {
	if len(path) == 0 {
		return fs
	}
	out := make(expr.Fields, 0, len(fs))
	for _, p := range fs {
		if p.Name != path[0] {
			out = append(out, p)
			continue
		}
		if len(path) == 1 {
			continue
		}
		out = append(out, expr.Prop{Name: p.Name, Field: removeIn(p.Field, path[1:])})
	}
	return out
}

func removeIn(f expr.Field, path []string) expr.Field {
	switch f.Kind {
	case expr.Optional, expr.List, expr.Map:
		if f.Elem == nil {
			return f
		}
		e := removeIn(*f.Elem, path)
		f.Elem = &e
		return f
	case expr.Object:
		f.Props = []expr.Prop(removePath(expr.Fields(f.Props), path))
		return f
	case expr.String, expr.Int, expr.Double, expr.Bool, expr.Dyn, expr.Enum, expr.Timestamp:
		return f // no fields inside
	}
	return f
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
