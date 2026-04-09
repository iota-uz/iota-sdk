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
// components that need `app` (to pass to controller constructors for
// middleware wiring) call RequireApplication(container) from inside a
// ContributeControllers closure, which resolves it through the container's
// auto-provided application.Application provider.
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
func ProvideAs[I any, S any](builder *Builder, provider any) {
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

func appendProvider[T any](builder *Builder, name string, keyType reflect.Type, provider any) {
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
}

type providerState uint8

const (
	providerPending providerState = iota
	providerResolving
	providerResolved
)
