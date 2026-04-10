package composition

import (
	"embed"
	"fmt"
	"reflect"

	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

// BuildContext is the read-only context threaded through every
// Component.Build call. It exposes the pooled dependencies every component
// needs (db, event bus, bundle, config, logger) and the active capability
// set. It deliberately does NOT expose the application.Application handle;
// components that need `app` (for example, to pass it to a controller
// constructor) resolve it through the container's auto-provided
// application.Application provider — either as a parameter of a
// ContributeControllersFunc constructor or via composition.Resolve from
// inside a ContributeControllers closure.
type BuildContext struct {
	app                application.Application // private; accessed only via auto-provided provider
	db                 *pgxpool.Pool
	eventPublisher     eventbus.EventBus
	logger             *logrus.Logger
	bundle             *i18n.Bundle
	config             *configuration.Configuration
	ActiveCapabilities map[Capability]struct{}
}

// NewBuildContext constructs the BuildContext from the application handle and
// the SDK configuration. The application handle is captured privately so
// Engine.Compile can auto-register application.Application as a provider at
// container instantiation time.
func NewBuildContext(app application.Application, config *configuration.Configuration) BuildContext {
	ctx := BuildContext{
		app:    app,
		config: config,
	}
	if app != nil {
		ctx.db = app.DB()
		ctx.eventPublisher = app.EventPublisher()
		ctx.bundle = app.Bundle()
	}
	if config != nil {
		ctx.logger = config.Logger()
	}
	return ctx
}

func (c BuildContext) DB() *pgxpool.Pool {
	return c.db
}

func (c BuildContext) EventPublisher() eventbus.EventBus {
	return c.eventPublisher
}

func (c BuildContext) Logger() *logrus.Logger {
	return c.logger
}

func (c BuildContext) Bundle() *i18n.Bundle {
	return c.bundle
}

func (c BuildContext) Config() *configuration.Configuration {
	return c.config
}

func (c BuildContext) HasCapability(capability Capability) bool {
	if len(c.ActiveCapabilities) == 0 {
		return true
	}
	_, ok := c.ActiveCapabilities[capability]
	return ok
}

func (c BuildContext) withCapabilities(capabilities []Capability) BuildContext {
	if len(capabilities) == 0 {
		c.ActiveCapabilities = nil
		return c
	}

	active := make(map[Capability]struct{}, len(capabilities))
	for _, capability := range capabilities {
		active[capability] = struct{}{}
	}
	c.ActiveCapabilities = active
	return c
}

type Builder struct {
	context    BuildContext
	descriptor Descriptor

	providers           []*providerEntry
	controllerFactories []namedFactory[[]application.Controller]
	navItemFactories    []namedFactory[[]types.NavigationItem]
	localeFactories     []namedFactory[[]*embed.FS]
	schemaFactories     []namedFactory[[]application.GraphSchema]
	appletFactories     []namedFactory[[]application.Applet]
	assetFactories      []namedFactory[[]*embed.FS]
	hashFSFactories     []namedFactory[[]*hashfs.FS]
	quickLinkFactories  []namedFactory[[]*spotlight.QuickLink]
	spotlightFactories  []namedFactory[[]spotlight.SearchProvider]
	spotlightAgent      *namedFactory[spotlight.Agent]
	middlewareFactories []namedFactory[[]mux.MiddlewareFunc]
	hookFactories       []namedFactory[[]Hook]

	// Removals are recorded here and processed after every builder has
	// contributed, so a downstream component can cleanly replace upstream
	// providers/controllers/hooks without needing to win a registration race.
	// See Container.applyRemovals and the override decision table in
	// Container.addBuilder.
	providerRemovals   []Key
	controllerRemovals []string // matched against Controller.Key()
	hookRemovals       []string // matched against Hook.Name

	eventHandlerSeq int // monotonic counter for unique event-handler hook names
}

type namedFactory[T any] struct {
	component string
	label     string
	factory   func(*Container) (T, error)
}

func newBuilder(context BuildContext, descriptor Descriptor) *Builder {
	return &Builder{
		context:    context,
		descriptor: descriptor,
	}
}

func (b *Builder) Context() BuildContext {
	return b.context
}

func (b *Builder) Descriptor() Descriptor {
	return b.descriptor
}

func Provide[T any](builder *Builder, provider any) {
	appendProvider[T](builder, "", typeOf[T](), provider)
}

// ProvideAs binds a concrete value (or factory) under both the concrete type
// `S` and the interface type `I`. Use it when consumers may depend on either
// the implementation pointer or the interface — eliminates the need for two
// separate Provide calls with the same value.
//
// Example:
//
//	composition.ProvideAs[crmServices.ClientService, *crmServices.ClientServiceImpl](
//	    builder, clientService,
//	)
//
// Both keys point at the same instance.
//
// Panics at registration time if `I` is not an interface or if `S` does not
// implement `I` — the mismatch used to only surface on the first resolve.
func ProvideAs[I any, S any](builder *Builder, provider any) {
	validateInterfaceAssignment[I, S]("ProvideAs")
	appendProvider[S](builder, "", typeOf[S](), provider)
	// Bridge the interface key to the concrete provider so we don't run the
	// factory twice. The bridge factory resolves the concrete and casts.
	concreteKey := keyFor(typeOf[S](), "")
	appendProvider[I](builder, "", typeOf[I](), func(container *Container) (I, error) {
		raw, err := container.resolveAny(concreteKey)
		if err != nil {
			var zero I
			return zero, err
		}
		typed, ok := raw.(I)
		if !ok {
			var zero I
			return zero, fmt.Errorf("composition: %s does not implement %s", concreteKey, typeOf[I]())
		}
		return typed, nil
	})
}

// ProvideDefault registers a provider that downstream components are allowed
// to replace. Semantically identical to Provide[T], except the entry is
// flagged overridable — if another component (typically a downstream host)
// later registers a non-default provider for the same key, Container.addBuilder
// silently drops this default and installs the override.
//
// Two ProvideDefault calls for the same key still error out (two components
// both claiming the default slot is ambiguous).
//
// Use ProvideDefault when a component ships a reasonable implementation but
// expects downstream consumers to swap in their own. Use plain Provide when
// the implementation is an invariant of the component.
func ProvideDefault[T any](builder *Builder, provider any) {
	entry := appendProvider[T](builder, "", typeOf[T](), provider)
	entry.overridable = true
}

// ProvideDefaultAs is ProvideDefault bridged to an interface key. Both the
// concrete (`S`) and interface (`I`) entries are marked overridable so a
// downstream consumer can replace either independently.
//
// Panics at registration time if `I` is not an interface or if `S` does not
// implement `I`.
func ProvideDefaultAs[I any, S any](builder *Builder, provider any) {
	validateInterfaceAssignment[I, S]("ProvideDefaultAs")
	concreteEntry := appendProvider[S](builder, "", typeOf[S](), provider)
	concreteEntry.overridable = true
	concreteKey := keyFor(typeOf[S](), "")
	ifaceEntry := appendProvider[I](builder, "", typeOf[I](), func(container *Container) (I, error) {
		raw, err := container.resolveAny(concreteKey)
		if err != nil {
			var zero I
			return zero, err
		}
		typed, ok := raw.(I)
		if !ok {
			var zero I
			return zero, fmt.Errorf("composition: %s does not implement %s", concreteKey, typeOf[I]())
		}
		return typed, nil
	})
	ifaceEntry.overridable = true
}

// RemoveProvider schedules the provider for type T to be deleted from the
// container before instantiation. Use this when an upstream component did
// NOT register its provider via ProvideDefault but you still need to replace
// it.
//
// Execution order: Container.addBuilder collects removals alongside normal
// registrations; Container.applyRemovals processes them after every builder
// has contributed but before any provider is resolved. That means:
//
//   - Upstream providers are visible during builder registration (so
//     downstream can see what's being overridden), then blown away before
//     instantiation.
//   - The downstream component is responsible for registering a replacement
//     provider; without one, downstream resolves return NOT_PROVIDED.
//   - Removal targeting a key that was never provided is a no-op.
//
// For this to work reliably, the downstream component must declare a
// Requires dependency on the upstream component so their builders run in
// topological order — otherwise "upstream after downstream" leaves the
// removal applied to nothing.
func RemoveProvider[T any](builder *Builder) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	builder.providerRemovals = append(builder.providerRemovals, KeyFor[T]())
}

// RemoveController schedules every controller whose Key() equals `key` to
// be filtered out of the final container during materialize. Safe to call
// even when no such controller is registered (no-op).
func RemoveController(builder *Builder, key string) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if key == "" {
		panic("composition: RemoveController: key must not be empty")
	}
	builder.controllerRemovals = append(builder.controllerRemovals, key)
}

// RemoveHook schedules every hook whose Name equals `name` to be filtered
// out during materialize. Safe to call even when no such hook is registered.
func RemoveHook(builder *Builder, name string) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if name == "" {
		panic("composition: RemoveHook: name must not be empty")
	}
	builder.hookRemovals = append(builder.hookRemovals, name)
}

// validateInterfaceAssignment enforces that a ProvideAs-family helper is
// called with a real interface type `I` and a `S` that satisfies it. Called
// at registration time so the mismatch surfaces as a loud panic instead of
// a latent resolution failure.
func validateInterfaceAssignment[I any, S any](helperName string) {
	ifaceType := typeOf[I]()
	if ifaceType.Kind() != reflect.Interface {
		panic(fmt.Sprintf(
			"composition: %s target must be an interface, got %s (%s)",
			helperName, ifaceType, ifaceType.Kind(),
		))
	}
	concreteType := typeOf[S]()
	if !concreteType.Implements(ifaceType) {
		panic(fmt.Sprintf(
			"composition: %s: %s does not implement %s",
			helperName, concreteType, ifaceType,
		))
	}
}

// appendProvider registers a provider on the builder and returns the new
// entry so callers can set flags like `overridable` without having to thread
// them through every call site.
func appendProvider[T any](builder *Builder, name string, keyType reflect.Type, provider any) *providerEntry {
	if builder == nil {
		panic("composition: builder is nil")
	}
	factory, producedType := providerFactory[T](provider)
	entry := &providerEntry{
		key:           keyFor(keyType, name),
		componentName: builder.descriptor.Name,
		capabilities:  append([]Capability(nil), builder.descriptor.Capabilities...),
		displayName:   keyFor(producedType, name).DisplayName(),
		factory:       factory,
	}
	builder.providers = append(builder.providers, entry)
	return entry
}

func ContributeControllers(builder *Builder, factory func(*Container) ([]application.Controller, error)) {
	appendFactory(builder, "controllers", factory, &builder.controllerFactories)
}

func ContributeNavItems(builder *Builder, factory func(*Container) ([]types.NavigationItem, error)) {
	appendFactory(builder, "nav-items", factory, &builder.navItemFactories)
}

func ContributeLocales(builder *Builder, factory func(*Container) ([]*embed.FS, error)) {
	appendFactory(builder, "locales", factory, &builder.localeFactories)
}

func ContributeSchemas(builder *Builder, factory func(*Container) ([]application.GraphSchema, error)) {
	appendFactory(builder, "schemas", factory, &builder.schemaFactories)
}

func ContributeApplets(builder *Builder, factory func(*Container) ([]application.Applet, error)) {
	appendFactory(builder, "applets", factory, &builder.appletFactories)
}

func ContributeAssets(builder *Builder, factory func(*Container) ([]*embed.FS, error)) {
	appendFactory(builder, "assets", factory, &builder.assetFactories)
}

func ContributeHashFS(builder *Builder, factory func(*Container) ([]*hashfs.FS, error)) {
	appendFactory(builder, "hashfs", factory, &builder.hashFSFactories)
}

func ContributeQuickLinks(builder *Builder, factory func(*Container) ([]*spotlight.QuickLink, error)) {
	appendFactory(builder, "quick-links", factory, &builder.quickLinkFactories)
}

func ContributeSpotlightProviders(builder *Builder, factory func(*Container) ([]spotlight.SearchProvider, error)) {
	appendFactory(builder, "spotlight", factory, &builder.spotlightFactories)
}

func ContributeSpotlightAgent(builder *Builder, factory func(*Container) (spotlight.Agent, error)) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if factory == nil {
		return
	}
	builder.spotlightAgent = &namedFactory[spotlight.Agent]{
		component: builder.descriptor.Name,
		label:     "spotlight-agent",
		factory:   factory,
	}
}

func ContributeMiddleware(builder *Builder, factory func(*Container) ([]mux.MiddlewareFunc, error)) {
	appendFactory(builder, "middleware", factory, &builder.middlewareFactories)
}

func ContributeHooks(builder *Builder, factory func(*Container) ([]Hook, error)) {
	appendFactory(builder, "hooks", factory, &builder.hookFactories)
}

func appendFactory[T any](builder *Builder, label string, factory func(*Container) (T, error), target *[]namedFactory[T]) {
	if builder == nil {
		panic("composition: builder is nil")
	}
	if factory == nil {
		return
	}
	*target = append(*target, namedFactory[T]{
		component: builder.descriptor.Name,
		label:     label,
		factory:   factory,
	})
}

func providerFactory[T any](provider any) (func(*Container) (any, error), reflect.Type) {
	targetType := typeOf[T]()
	if provider == nil {
		panic("composition: provider is nil")
	}

	value := reflect.ValueOf(provider)
	if value.Kind() != reflect.Func {
		producedType := value.Type()
		if !producedType.AssignableTo(targetType) && !producedType.Implements(targetType) {
			panic(fmt.Sprintf("composition: provider value %s does not satisfy %s", producedType, targetType))
		}
		return func(*Container) (any, error) {
			return provider, nil
		}, producedType
	}

	fnType := value.Type()
	containerType := reflect.TypeOf(&Container{})
	errorType := reflect.TypeOf((*error)(nil)).Elem()

	if fnType.NumIn() > 1 {
		panic("composition: provider functions accept at most one argument")
	}
	needsContainer := false
	if fnType.NumIn() == 1 {
		if fnType.In(0) != containerType {
			panic(fmt.Sprintf("composition: provider argument must be *composition.Container, got %s", fnType.In(0)))
		}
		needsContainer = true
	}
	if fnType.NumOut() != 1 && fnType.NumOut() != 2 {
		panic("composition: provider functions must return one value or value,error")
	}
	if fnType.NumOut() == 2 && !fnType.Out(1).Implements(errorType) {
		panic("composition: provider second return value must be error")
	}

	producedType := fnType.Out(0)
	if !producedType.AssignableTo(targetType) && !producedType.Implements(targetType) {
		panic(fmt.Sprintf("composition: provider result %s does not satisfy %s", producedType, targetType))
	}

	return func(container *Container) (any, error) {
		args := make([]reflect.Value, 0, 1)
		if needsContainer {
			args = append(args, reflect.ValueOf(container))
		}
		results := value.Call(args)
		if len(results) == 2 && !results[1].IsNil() {
			return nil, results[1].Interface().(error)
		}
		return results[0].Interface(), nil
	}, producedType
}

type providerEntry struct {
	key           Key
	componentName string
	capabilities  []Capability
	displayName   string
	factory       func(*Container) (any, error)
	state         providerState
	value         any
	// overridable signals that this entry was registered via
	// ProvideDefault[As]. Container.addBuilder silently replaces an
	// overridable entry with a non-overridable one for the same key.
	overridable bool
}

type providerState uint8

const (
	providerPending providerState = iota
	providerResolving
	providerResolved
)
