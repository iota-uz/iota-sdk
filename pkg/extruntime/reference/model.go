// Package reference is the live API reference of the extension platform: what
// is installed right now (the native contracts of the application and the
// manifests of the activated package versions), rendered for people and
// tools — a JSON model for the configurator, markdown for the Help Center and
// MCP, and the contract parts of the OpenAPI document.
//
// Everything is derived from the catalog and filtered per reader (see
// Visible). The reference never carries implementation source, template
// bodies, secrets or capability declarations: it describes contracts.
package reference

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/catalog"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// TypeDoc is a wire type with its signature, JSON Schema and a valid example.
type TypeDoc struct {
	Signature string          `json:"signature"`
	Schema    Schema          `json:"schema"`
	Example   json.RawMessage `json:"example"`
}

// SignalDoc is one workflow signal and its payload.
type SignalDoc struct {
	Name    string  `json:"name"`
	Payload TypeDoc `json:"payload"`
}

// FieldDoc is one field of an entity or overlay.
type FieldDoc struct {
	ID        string          `json:"id"`
	Label     string          `json:"label,omitempty"`
	Type      string          `json:"type"`
	Required  bool            `json:"required"`
	Unique    bool            `json:"unique,omitempty"`
	Indexed   bool            `json:"indexed,omitempty"`
	Default   json.RawMessage `json:"default,omitempty"`
	RenamedAs []string        `json:"renamedFrom,omitempty"`
	// Permissions are all required to read (and write) the field.
	Permissions []string `json:"permissions,omitempty"`
}

// ReadRuleDoc is one row-visibility rule of an entity.
type ReadRuleDoc struct {
	Permission string `json:"permission,omitempty"`
	AllRows    bool   `json:"allRows,omitempty"`
	// Filtered is true when the rule grants only the rows its filter selects.
	Filtered bool `json:"filtered,omitempty"`
}

// AccessDoc is who may read an entity. No rules: every subject of the tenant.
type AccessDoc struct {
	Read []ReadRuleDoc `json:"read,omitempty"`
}

// NavDoc is the navigation entry of a page.
type NavDoc struct {
	Label     string `json:"label,omitempty"`
	Group     string `json:"group,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Parent    string `json:"parent,omitempty"`
	Icon      string `json:"icon,omitempty"`
	Order     int    `json:"order,omitempty"`
}

// IDLabel is an identified, labelled navigation member.
type IDLabel struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

// NavigationDoc is what a navigation resource contributes.
type NavigationDoc struct {
	Groups     []IDLabel `json:"groups,omitempty"`
	Workspaces []IDLabel `json:"workspaces,omitempty"`
	Palette    []IDLabel `json:"palette,omitempty"`
	// Patches are the refs the package hides, renames or moves.
	Patches []string `json:"patches,omitempty"`
}

// GuardDoc is an additive check on a target.
type GuardDoc struct {
	ID      string `json:"id"`
	Target  string `json:"target"`
	Phase   string `json:"phase"`
	Package string `json:"package"`
	Version string `json:"version,omitempty"`
}

// Item is one documented resource.
type Item struct {
	ID          string                `json:"id"`
	Kind        manifest.ResourceKind `json:"kind"`
	Label       string                `json:"label,omitempty"`
	Description string                `json:"description,omitempty"`
	// Package and Version identify the package that declares the resource;
	// empty for a native (Go-registered) contract.
	Package string `json:"package,omitempty"`
	Version string `json:"version,omitempty"`
	Native  bool   `json:"native,omitempty"`
	// Permission is the RBAC permission needed to call or open the resource.
	Permission string `json:"permission,omitempty"`

	Atomic  bool        `json:"atomic,omitempty"`
	Input   *TypeDoc    `json:"input,omitempty"`
	Output  *TypeDoc    `json:"output,omitempty"`
	Payload *TypeDoc    `json:"payload,omitempty"`
	Params  *TypeDoc    `json:"params,omitempty"`
	Props   *TypeDoc    `json:"props,omitempty"`
	Errors  []string    `json:"errors,omitempty"`
	Subject string      `json:"subject,omitempty"`
	Signals []SignalDoc `json:"signals,omitempty"`

	// entity / overlay
	KeyType string     `json:"keyType,omitempty"`
	Base    string     `json:"base,omitempty"`
	Fields  []FieldDoc `json:"fields,omitempty"`
	Access  *AccessDoc `json:"access,omitempty"`
	Record  *TypeDoc   `json:"record,omitempty"`

	// route / page / slot / cell / column
	Path       string         `json:"path,omitempty"`
	Route      string         `json:"route,omitempty"`
	Target     string         `json:"target,omitempty"`
	Order      int            `json:"order,omitempty"`
	Nav        *NavDoc        `json:"nav,omitempty"`
	Navigation *NavigationDoc `json:"navigation,omitempty"`
	List       string         `json:"list,omitempty"`
	Overlay    string         `json:"overlay,omitempty"`
	Field      string         `json:"field,omitempty"`
	Filterable bool           `json:"filterable,omitempty"`
	Sortable   bool           `json:"sortable,omitempty"`
	Exportable bool           `json:"exportable,omitempty"`

	// Channels the declaring package ships templates for (notice only).
	Channels []string `json:"channels,omitempty"`
	// Guards run on this target; Subscribers are the subscriptions to this event.
	Guards      []GuardDoc `json:"guards,omitempty"`
	Subscribers []string   `json:"subscribers,omitempty"`
	// ExtendedBy lists the overlays that attach to this entity.
	ExtendedBy []string `json:"extendedBy,omitempty"`

	res    manifest.Resource
	labels manifest.Label
	// refKey picks the sample key of a ref; the record of an entity is
	// rebuilt from the fields a reader may see.
	refKey func(string) string
}

// Resource is the contract as declared. It is unexported data of the model:
// the visibility rule and the OpenAPI builder read it, JSON does not carry it.
func (i Item) Resource() manifest.Resource { return i.res }

// EndpointDoc is an HTTP endpoint of a package.
type EndpointDoc struct {
	ID     string `json:"id"`
	Method string `json:"method"`
	// Path is the manifest path; URL is where it is served.
	Path    string `json:"path"`
	URL     string `json:"url"`
	Public  bool   `json:"public"`
	Package string `json:"package"`
	Version string `json:"version"`
}

// ScheduleDoc is a cron schedule of a package.
type ScheduleDoc struct {
	ID       string `json:"id"`
	Cron     string `json:"cron"`
	Timezone string `json:"timezone,omitempty"`
}

// SubscriptionDoc is an event subscription of a package.
type SubscriptionDoc struct {
	ID    string `json:"id"`
	Event string `json:"event"`
}

// PackageRef names the active package version behind a namespace.
type PackageRef struct {
	Namespace string `json:"namespace"`
	Version   string `json:"version"`
}

// Namespace is the reference of one namespace: a package (its active version)
// or a native root ("core", "crm").
type Namespace struct {
	Name string `json:"name"`
	// Package is set for a package namespace.
	Package *PackageRef `json:"package,omitempty"`
	// Native is true when the namespace has native contracts.
	Native        bool              `json:"native,omitempty"`
	Items         []Item            `json:"items"`
	Endpoints     []EndpointDoc     `json:"endpoints,omitempty"`
	Schedules     []ScheduleDoc     `json:"schedules,omitempty"`
	Subscriptions []SubscriptionDoc `json:"subscriptions,omitempty"`
	// Guards are the guards this namespace's package declares, on any target.
	Guards []GuardDoc `json:"guards,omitempty"`
}

// Model is the reference of everything a reader may see.
type Model struct {
	Namespaces []Namespace `json:"namespaces"`
	// Full is true when the reader holds Extensions.Read and sees everything.
	Full bool `json:"full"`
}

// Namespace returns the named namespace.
func (m *Model) Namespace(name string) (Namespace, bool) {
	if m == nil {
		return Namespace{}, false
	}
	for _, ns := range m.Namespaces {
		if ns.Name == name {
			return ns, true
		}
	}
	return Namespace{}, false
}

// Lookup finds an item by resource ID.
func (m *Model) Lookup(id string) (Item, bool) {
	if m == nil {
		return Item{}, false
	}
	for _, ns := range m.Namespaces {
		for _, it := range ns.Items {
			if it.ID == id {
				return it, true
			}
		}
	}
	return Item{}, false
}

// Source supplies what the reference is built from; *catalog.Catalog is one.
type Source interface {
	// Active lists the activated package versions.
	Active(ctx context.Context) ([]catalog.ActiveVersion, error)
	// Natives is the application's native contract set (may be nil).
	Natives() *catalog.NativeSet
}

// generationSource is implemented by sources that can say when they changed
// (*catalog.Catalog), so the unfiltered model is cached between activations.
type generationSource interface{ Generation() uint64 }

// Builder builds the reference from a Source.
type Builder struct {
	src Source
	// cache of the unfiltered model for one catalog generation.
	cacheGen   uint64
	cacheModel *full
	cacheOK    bool
	mu         sync.Mutex
}

// New builds the reference over the catalog.
func New(src Source) *Builder { return &Builder{src: src} }

// Natives is a Source of native contracts only, for tools that run without a
// database (the offline OpenAPI export).
func Natives(n *catalog.NativeSet) Source { return nativeOnly{n} }

type nativeOnly struct{ n *catalog.NativeSet }

func (s nativeOnly) Active(context.Context) ([]catalog.ActiveVersion, error) { return nil, nil }
func (s nativeOnly) Natives() *catalog.NativeSet                             { return s.n }

// full is the unfiltered model: every namespace and item with raw labels.
type full struct {
	namespaces []Namespace
	resources  map[string]manifest.Resource
}

// Model builds the reference for the reader: sub's permissions decide what is
// visible and sub.Locale picks the labels. The locale argument, when set,
// wins over sub.Locale.
func (b *Builder) Model(ctx context.Context, sub spi.Subject, locale string) (*Model, error) {
	const op = serrors.Op("extruntime.reference.Builder.Model")
	f, err := b.snapshot(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if locale == "" {
		locale = sub.Locale
	}
	return f.view(sub, locale), nil
}

// FullModel is the model an administrator sees (every namespace, every
// resource), in locale. Offline exports use it.
func (b *Builder) FullModel(ctx context.Context, locale string) (*Model, error) {
	const op = serrors.Op("extruntime.reference.Builder.FullModel")
	f, err := b.snapshot(ctx)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	return f.viewAll(locale), nil
}

func (b *Builder) snapshot(ctx context.Context) (*full, error) {
	gs, cacheable := b.src.(generationSource)
	var gen uint64
	if cacheable {
		gen = gs.Generation()
		b.mu.Lock()
		if b.cacheOK && b.cacheGen == gen {
			f := b.cacheModel
			b.mu.Unlock()
			return f, nil
		}
		b.mu.Unlock()
	}
	active, err := b.src.Active(ctx)
	if err != nil {
		return nil, err
	}
	f := assemble(active, b.src.Natives())
	if cacheable {
		b.mu.Lock()
		b.cacheGen, b.cacheModel, b.cacheOK = gen, f, true
		b.mu.Unlock()
	}
	return f, nil
}

// kindOrder is the order of resource kinds within a namespace page.
var kindOrder = map[manifest.ResourceKind]int{
	manifest.ResEntity: 0, manifest.ResOverlay: 1, manifest.ResCommand: 2, manifest.ResOperation: 3,
	manifest.ResQuery: 4, manifest.ResEvent: 5, manifest.ResWorkflow: 6, manifest.ResNotice: 7,
	manifest.ResIssue: 8, manifest.ResDomainError: 9, manifest.ResPermission: 10, manifest.ResRoute: 11,
	manifest.ResPage: 12, manifest.ResNavigation: 13, manifest.ResSlot: 14, manifest.ResCell: 15,
	manifest.ResColumn: 16, manifest.ResShell: 17, manifest.ResContactResolver: 18, manifest.ResDeliveryPolicy: 19,
}

type declared struct {
	res     manifest.Resource
	pkg     string
	version string
	native  bool
}

func assemble(active []catalog.ActiveVersion, natives *catalog.NativeSet) *full {
	f := &full{resources: map[string]manifest.Resource{}}
	nativeResources := natives.Resources()
	decls := make([]declared, 0, len(nativeResources))
	// Natives win over a package declaring the same ID, as in the catalog.
	for _, r := range nativeResources {
		// A native contract's permission lives on its definition, not in the
		// published resource; the host enforces it from there.
		if def, ok := natives.Set().Contract(r.ID); ok && r.Permission == "" {
			r.Permission = def.Permission()
		}
		decls = append(decls, declared{res: r, native: true})
		f.resources[r.ID] = r
	}
	for _, av := range active {
		for _, r := range av.Manifest.Resources {
			if _, taken := f.resources[r.ID]; taken {
				continue
			}
			decls = append(decls, declared{res: r, pkg: av.Namespace, version: av.Version})
			f.resources[r.ID] = r
		}
	}
	// Samples of a ref carry the key type of the entity it points at.
	refKey := func(entity string) string {
		switch f.resources[entity].KeyType {
		case "int64":
			return "1"
		case "string":
			return "key-1"
		}
		return ""
	}

	byNS := map[string]*Namespace{}
	ns := func(name string) *Namespace {
		n := byNS[name]
		if n == nil {
			n = &Namespace{Name: name, Items: []Item{}}
			byNS[name] = n
		}
		return n
	}
	for _, d := range decls {
		name := d.pkg
		if d.native {
			name = catalog.Namespace(d.res.ID)
		}
		n := ns(name)
		n.Native = n.Native || d.native
		n.Items = append(n.Items, newItem(d.res, d.pkg, d.version, d.native, refKey))
	}
	var guards []GuardDoc
	extended := map[string][]string{}
	for _, d := range decls {
		if d.res.Kind == manifest.ResOverlay && d.res.Base != "" {
			extended[d.res.Base] = append(extended[d.res.Base], d.res.ID)
		}
	}
	subscribers := map[string][]string{}
	templates := map[string][]string{}
	for _, av := range active {
		n := ns(av.Namespace)
		n.Package = &PackageRef{Namespace: av.Namespace, Version: av.Version}
		for _, e := range av.Manifest.Endpoints {
			n.Endpoints = append(n.Endpoints, EndpointDoc{
				ID: e.ID, Method: strings.ToUpper(e.Method), Path: e.Path, URL: EndpointURL(av.Namespace, e.Path),
				Public: e.Public, Package: av.Namespace, Version: av.Version,
			})
		}
		for _, s := range av.Manifest.Schedules {
			n.Schedules = append(n.Schedules, ScheduleDoc{ID: s.ID, Cron: s.Cron, Timezone: s.Timezone})
		}
		for _, s := range av.Manifest.Subscriptions {
			n.Subscriptions = append(n.Subscriptions, SubscriptionDoc{ID: s.ID, Event: s.Event})
			subscribers[s.Event] = append(subscribers[s.Event], s.ID)
		}
		for _, g := range av.Manifest.Guards {
			gd := GuardDoc{ID: g.ID, Target: g.Target, Phase: string(g.Phase), Package: av.Namespace, Version: av.Version}
			n.Guards = append(n.Guards, gd)
			guards = append(guards, gd)
		}
		for _, t := range av.Manifest.Templates {
			templates[t.Notice] = append(templates[t.Notice], t.Channels...)
		}
	}
	names := make([]string, 0, len(byNS))
	for name := range byNS {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		n := byNS[name]
		for i := range n.Items {
			it := &n.Items[i]
			for _, g := range guards {
				if g.Target == it.ID {
					it.Guards = append(it.Guards, g)
				}
			}
			it.ExtendedBy = dedupeSorted(extended[it.ID])
			it.Subscribers = dedupeSorted(subscribers[it.ID])
			it.Channels = dedupeSorted(templates[it.ID])
		}
		slices.SortStableFunc(n.Items, func(a, b Item) int {
			return cmp.Or(cmp.Compare(kindOrder[a.Kind], kindOrder[b.Kind]), strings.Compare(a.ID, b.ID))
		})
		slices.SortFunc(n.Endpoints, func(a, b EndpointDoc) int {
			return cmp.Or(strings.Compare(a.Path, b.Path), strings.Compare(a.Method, b.Method))
		})
		slices.SortFunc(n.Schedules, func(a, b ScheduleDoc) int { return strings.Compare(a.ID, b.ID) })
		slices.SortFunc(n.Subscriptions, func(a, b SubscriptionDoc) int { return strings.Compare(a.ID, b.ID) })
		slices.SortFunc(n.Guards, func(a, b GuardDoc) int { return strings.Compare(a.ID, b.ID) })
		f.namespaces = append(f.namespaces, *n)
	}
	return f
}

func dedupeSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
	return slices.Compact(out)
}

// EndpointURL is where an endpoint of a package is served.
func EndpointURL(namespace, path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "/_ext/endpoints/" + namespace + path
}

func typeDoc(t *manifest.Type, refKey func(string) string) *TypeDoc {
	if t == nil {
		return nil
	}
	return &TypeDoc{Signature: Signature(*t), Schema: TypeSchema(*t), Example: ExampleWith(*t, refKey)}
}

func newItem(r manifest.Resource, pkg, version string, native bool, refKey func(string) string) Item {
	it := Item{
		ID: r.ID, Kind: r.Kind, Description: r.Description, Package: pkg, Version: version, Native: native,
		Permission: r.Permission, Atomic: r.Atomic, Errors: append([]string(nil), r.Errors...),
		Subject: r.Subject, KeyType: r.KeyType, Base: r.Base,
		Path: r.Path, Route: r.Route, Target: r.Target, Order: r.Order,
		List: r.List, Overlay: r.Overlay, Field: r.Field,
		Filterable: r.Filterable, Sortable: r.Sortable, Exportable: r.Exportable,
		Input: typeDoc(r.Input, refKey), Output: typeDoc(r.Output, refKey), Payload: typeDoc(r.Payload, refKey),
		Params: typeDoc(r.Params, refKey), Props: typeDoc(r.Props, refKey),
		res: r, labels: r.Label,
	}
	if len(r.Signals) > 0 {
		names := make([]string, 0, len(r.Signals))
		for name := range r.Signals {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			t := r.Signals[name]
			it.Signals = append(it.Signals, SignalDoc{Name: name, Payload: *typeDoc(&t, refKey)})
		}
	}
	if r.Kind == manifest.ResEntity || r.Kind == manifest.ResOverlay {
		fillEntity(&it, r, refKey)
	}
	if r.Nav != nil {
		it.Nav = &NavDoc{Group: r.Nav.Group, Workspace: r.Nav.Workspace, Parent: r.Nav.Parent, Icon: r.Nav.Icon, Order: r.Nav.Order}
	}
	if r.Kind == manifest.ResNavigation {
		it.Navigation = &NavigationDoc{}
		for _, g := range r.Groups {
			it.Navigation.Groups = append(it.Navigation.Groups, IDLabel{ID: g.ID})
		}
		for _, w := range r.Workspaces {
			it.Navigation.Workspaces = append(it.Navigation.Workspaces, IDLabel{ID: w.ID})
		}
		for _, p := range r.Palette {
			it.Navigation.Palette = append(it.Navigation.Palette, IDLabel{ID: p.ID})
		}
		for _, p := range r.Patches {
			it.Navigation.Patches = append(it.Navigation.Patches, p.Ref)
		}
	}
	return it
}

func fillEntity(it *Item, r manifest.Resource, refKey func(string) string) {
	for _, f := range r.Fields {
		fd := FieldDoc{
			ID: f.ID, Type: Signature(f.Type), Required: !f.Type.Optional, Unique: f.Unique, Indexed: f.Index,
			Default: f.Default, RenamedAs: f.RenamedFrom,
		}
		if r.Access != nil {
			fd.Permissions = append([]string(nil), r.Access.Fields[f.ID]...)
		}
		it.Fields = append(it.Fields, fd)
	}
	if r.Access != nil {
		ad := &AccessDoc{}
		for _, rule := range r.Access.Read {
			ad.Read = append(ad.Read, ReadRuleDoc{Permission: rule.Permission, AllRows: rule.All, Filtered: !rule.All && rule.Rows != nil})
		}
		it.Access = ad
	}
	it.refKey = refKey
	it.Record = recordDoc(r.Fields, refKey)
}

// recordDoc documents the wire record of an entity holding fields: its key and
// the fields. It is built from the fields a reader may see, so the record
// never names a field the field list omits.
func recordDoc(fields []manifest.Field, refKey func(string) string) *TypeDoc {
	record := manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
		{Name: "key", Type: manifest.Type{Kind: manifest.KindString}},
	}}
	for _, f := range fields {
		record.Properties = append(record.Properties, manifest.Property{Name: f.ID, Type: f.Type})
	}
	return typeDoc(&record, refKey)
}

// pickLabel chooses the text for locale: the exact tag, its language, English,
// then the lexically first available.
func pickLabel(l manifest.Label, locale string) string {
	if len(l) == 0 {
		return ""
	}
	locale = strings.ToLower(strings.ReplaceAll(locale, "_", "-"))
	if v := l[locale]; v != "" {
		return v
	}
	if lang, _, ok := strings.Cut(locale, "-"); ok {
		if v := l[lang]; v != "" {
			return v
		}
	}
	if v := l["en"]; v != "" {
		return v
	}
	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if l[k] != "" {
			return l[k]
		}
	}
	return ""
}
