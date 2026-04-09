package composition

import (
	"context"
	"embed"
	"fmt"
	"reflect"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

type BuildContext struct {
	App                application.Application
	ActiveCapabilities map[Capability]struct{}
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

type Hook struct {
	Name  string
	Start func(context.Context, *Container) error
	Stop  func(context.Context, *Container) error
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
	spotlightFactories  []namedFactory[[]spotlight.SearchProvider]
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

func ProvideNamed[T any](builder *Builder, name string, provider any) {
	appendProvider[T](builder, name, typeOf[T](), provider)
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

func ContributeSpotlightProviders(builder *Builder, factory func(*Container) ([]spotlight.SearchProvider, error)) {
	appendFactory(builder, "spotlight", factory, &builder.spotlightFactories)
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
