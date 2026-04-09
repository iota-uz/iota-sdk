package composition

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var (
	ErrNotProvided   = errors.New("composition: not provided")
	ErrCycleDetected = errors.New("composition: cycle detected")
)

func IsNotProvided(err error) bool {
	return errors.Is(err, ErrNotProvided)
}

type ResolutionError struct {
	cause error
	path  []string
}

func (e *ResolutionError) Error() string {
	switch {
	case errors.Is(e.cause, ErrNotProvided):
		return fmt.Sprintf("%s (NOT PROVIDED)", formatPath(e.path))
	case errors.Is(e.cause, ErrCycleDetected):
		return fmt.Sprintf("%s (CYCLE)", formatPath(e.path))
	default:
		return fmt.Sprintf("%s: %v", formatPath(e.path), e.cause)
	}
}

func (e *ResolutionError) Unwrap() error {
	return e.cause
}

type Engine struct {
	components []registeredComponent
}

type registeredComponent struct {
	component  Component
	descriptor Descriptor
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Register(components ...Component) error {
	seen := make(map[string]struct{}, len(e.components)+len(components))
	for _, existing := range e.components {
		seen[existing.descriptor.Name] = struct{}{}
	}

	for _, component := range components {
		if component == nil {
			continue
		}
		descriptor := normalizeDescriptor(component)
		if _, ok := seen[descriptor.Name]; ok {
			return fmt.Errorf("composition: duplicate component %q", descriptor.Name)
		}
		seen[descriptor.Name] = struct{}{}
		e.components = append(e.components, registeredComponent{
			component:  component,
			descriptor: descriptor,
		})
	}
	return nil
}

func (e *Engine) Compile(ctx BuildContext, capabilities ...Capability) (*Container, error) {
	ordered, err := e.orderComponents()
	if err != nil {
		return nil, err
	}

	activeCapabilities := normalizeCapabilities(capabilities)
	ctx = ctx.withCapabilities(activeCapabilities)

	builders := make([]*Builder, 0, len(ordered))
	for _, component := range ordered {
		builder := newBuilder(ctx, component.descriptor)
		if err := component.component.Build(builder); err != nil {
			return nil, fmt.Errorf("build component %q: %w", component.descriptor.Name, err)
		}
		builders = append(builders, builder)
	}

	container := newContainer(ctx, activeCapabilities)
	for _, builder := range builders {
		if !componentActive(builder.descriptor, activeCapabilities) {
			continue
		}
		if err := container.addBuilder(builder); err != nil {
			return nil, err
		}
	}
	if err := container.instantiateAll(); err != nil {
		return nil, err
	}
	if err := container.materialize(); err != nil {
		return nil, err
	}
	// Materialize is done; controllers + nav items + locales + applets are
	// all attached. Publish the container to the application as the runtime
	// source so app.Controllers()/app.NavItems()/etc. read from it.
	if ctx.app != nil {
		if binder, ok := ctx.app.(application.RuntimeBinder); ok {
			if err := binder.AttachRuntimeSource(container); err != nil {
				return nil, err
			}
		}
	}
	return container, nil
}

func (e *Engine) Run(ctx context.Context, buildCtx BuildContext, capabilities ...Capability) (*Container, error) {
	container, err := e.Compile(buildCtx, capabilities...)
	if err != nil {
		return nil, err
	}
	if err := e.Start(ctx, container); err != nil {
		return nil, err
	}
	return container, nil
}

func (e *Engine) Start(ctx context.Context, container *Container) error {
	return Start(ctx, container)
}

func (e *Engine) Stop(ctx context.Context, container *Container) error {
	return Stop(ctx, container)
}

func Start(ctx context.Context, container *Container) error {
	if container == nil || container.started {
		return nil
	}

	started := make([]Hook, 0, len(container.hooks))
	for _, hook := range container.hooks {
		if hook.Start == nil {
			started = append(started, hook)
			continue
		}
		if err := hook.Start(ctx, container); err != nil {
			for i := len(started) - 1; i >= 0; i-- {
				if started[i].Stop != nil {
					_ = started[i].Stop(ctx, container)
				}
			}
			return fmt.Errorf("start hook %q: %w", hook.Name, err)
		}
		started = append(started, hook)
	}

	container.started = true
	return nil
}

func Stop(ctx context.Context, container *Container) error {
	if container == nil || !container.started {
		return nil
	}

	var stopErr error
	for i := len(container.hooks) - 1; i >= 0; i-- {
		hook := container.hooks[i]
		if hook.Stop == nil {
			continue
		}
		if err := hook.Stop(ctx, container); err != nil {
			stopErr = errors.Join(stopErr, fmt.Errorf("stop hook %q: %w", hook.Name, err))
		}
	}
	container.started = false
	return stopErr
}

func (e *Engine) orderComponents() ([]registeredComponent, error) {
	components := make(map[string]registeredComponent, len(e.components))
	for _, component := range e.components {
		components[component.descriptor.Name] = component
	}

	state := make(map[string]providerState, len(e.components))
	stack := make([]string, 0, len(e.components))
	ordered := make([]registeredComponent, 0, len(e.components))

	var visit func(name string) error
	visit = func(name string) error {
		component, ok := components[name]
		if !ok {
			return fmt.Errorf("composition: component %q not registered", name)
		}
		switch state[name] {
		case providerPending:
			// Continue into dependency resolution below.
		case providerResolved:
			return nil
		case providerResolving:
			return fmt.Errorf("composition: component cycle: %s", strings.Join(append(append([]string(nil), stack...), name), " -> "))
		}

		state[name] = providerResolving
		stack = append(stack, name)
		for _, dependency := range component.descriptor.Requires {
			if _, ok := components[dependency]; !ok {
				return fmt.Errorf("composition: component %q requires %q", name, dependency)
			}
			if err := visit(dependency); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = providerResolved
		ordered = append(ordered, component)
		return nil
	}

	for _, component := range e.components {
		if err := visit(component.descriptor.Name); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}

type Container struct {
	context            BuildContext
	activeCapabilities []Capability
	providers          map[Key]*providerEntry
	providerOrder      []*providerEntry
	resolutionPath     []string

	controllerFactories   []namedFactory[[]application.Controller]
	navItemFactories      []namedFactory[[]types.NavigationItem]
	localeFactories       []namedFactory[[]*embed.FS]
	schemaFactories       []namedFactory[[]application.GraphSchema]
	appletFactories       []namedFactory[[]application.Applet]
	assetFactories        []namedFactory[[]*embed.FS]
	hashFSFactories       []namedFactory[[]*hashfs.FS]
	quickLinkFactories    []namedFactory[[]*spotlight.QuickLink]
	spotlightFactories    []namedFactory[[]spotlight.SearchProvider]
	spotlightAgentFactory *namedFactory[spotlight.Agent]
	middlewareFactories   []namedFactory[[]mux.MiddlewareFunc]
	hookFactories         []namedFactory[[]Hook]

	controllers        []application.Controller
	navItems           []types.NavigationItem
	locales            []*embed.FS
	graphSchemas       []application.GraphSchema
	applets            []application.Applet
	assets             []*embed.FS
	hashFSAssets       []*hashfs.FS
	quickLinks         []*spotlight.QuickLink
	spotlightProviders []spotlight.SearchProvider
	spotlightAgent     spotlight.Agent
	middleware         []mux.MiddlewareFunc
	hooks              []Hook
	started            bool
}

func newContainer(context BuildContext, activeCapabilities []Capability) *Container {
	return &Container{
		context:            context,
		activeCapabilities: append([]Capability(nil), activeCapabilities...),
		providers:          make(map[Key]*providerEntry),
	}
}

func (c *Container) HasCapability(capability Capability) bool {
	return c.context.HasCapability(capability)
}

func (c *Container) Controllers() []application.Controller {
	return append([]application.Controller(nil), c.controllers...)
}

func (c *Container) NavItems() []types.NavigationItem {
	return append([]types.NavigationItem(nil), c.navItems...)
}

func (c *Container) LocaleFiles() []*embed.FS {
	return append([]*embed.FS(nil), c.locales...)
}

func (c *Container) GraphSchemas() []application.GraphSchema {
	return append([]application.GraphSchema(nil), c.graphSchemas...)
}

func (c *Container) Applets() []application.Applet {
	return append([]application.Applet(nil), c.applets...)
}

func (c *Container) Assets() []*embed.FS {
	return append([]*embed.FS(nil), c.assets...)
}

func (c *Container) HashFSAssets() []*hashfs.FS {
	return append([]*hashfs.FS(nil), c.hashFSAssets...)
}

func (c *Container) QuickLinks() []*spotlight.QuickLink {
	return append([]*spotlight.QuickLink(nil), c.quickLinks...)
}

func (c *Container) SpotlightProviders() []spotlight.SearchProvider {
	return append([]spotlight.SearchProvider(nil), c.spotlightProviders...)
}

func (c *Container) SpotlightAgent() spotlight.Agent {
	return c.spotlightAgent
}

func (c *Container) Middleware() []mux.MiddlewareFunc {
	return append([]mux.MiddlewareFunc(nil), c.middleware...)
}

func (c *Container) Hooks() []Hook {
	return append([]Hook(nil), c.hooks...)
}

func (c *Container) AppendHooks(hooks ...Hook) {
	if c == nil {
		return
	}
	for _, hook := range hooks {
		if hook.Name == "" {
			continue
		}
		c.hooks = append(c.hooks, hook)
	}
}

func (c *Container) AppendControllers(controllers ...application.Controller) {
	if c == nil || len(controllers) == 0 {
		return
	}
	filtered := make([]application.Controller, 0, len(controllers))
	for _, controller := range controllers {
		if controller == nil {
			continue
		}
		filtered = append(filtered, controller)
	}
	if len(filtered) == 0 {
		return
	}
	c.controllers = append(c.controllers, filtered...)
}

func (c *Container) AppendHashFSAssets(fs ...*hashfs.FS) {
	if c == nil || len(fs) == 0 {
		return
	}
	filtered := make([]*hashfs.FS, 0, len(fs))
	for _, asset := range fs {
		if asset == nil {
			continue
		}
		filtered = append(filtered, asset)
	}
	if len(filtered) == 0 {
		return
	}
	c.hashFSAssets = append(c.hashFSAssets, filtered...)
}

func (c *Container) AppendMiddleware(middleware ...mux.MiddlewareFunc) {
	if c == nil || len(middleware) == 0 {
		return
	}
	c.middleware = append(c.middleware, middleware...)
}

func Resolve[T any](container *Container) (T, error) {
	return ResolveKey[T](container, KeyFor[T]())
}

// ResolveOptional returns the provided value, a presence flag, and any non-
// NOT_PROVIDED error. Use it when a component gracefully degrades in the
// absence of a provider.
func ResolveOptional[T any](container *Container) (T, bool, error) {
	value, err := Resolve[T](container)
	if err == nil {
		return value, true, nil
	}
	if IsNotProvided(err) {
		var zero T
		return zero, false, nil
	}
	var zero T
	return zero, false, err
}

func ResolveKey[T any](container *Container, key Key) (T, error) {
	if container == nil {
		var zero T
		return zero, fmt.Errorf("composition: container is nil")
	}
	value, err := container.resolveAny(key)
	if err != nil {
		var zero T
		return zero, err
	}
	typed, ok := value.(T)
	if ok {
		return typed, nil
	}
	var zero T
	return zero, fmt.Errorf("composition: resolved %s as %T", key, value)
}

func (c *Container) resolveAny(key Key) (any, error) {
	return c.resolveKeyWithPath(key, append([]string(nil), c.resolutionPath...))
}

func (c *Container) resolveKeyWithPath(key Key, path []string) (any, error) {
	entry, ok := c.providers[key]
	if !ok {
		missingPath := append([]string(nil), path...)
		if len(missingPath) == 0 {
			missingPath = []string{key.DisplayName()}
		} else {
			missingPath = append(missingPath, key.DisplayName())
		}
		return nil, &ResolutionError{
			cause: ErrNotProvided,
			path:  missingPath,
		}
	}
	return c.resolveEntry(entry, path)
}

func (c *Container) resolveEntry(entry *providerEntry, path []string) (any, error) {
	switch entry.state {
	case providerPending:
		// Resolve the provider below.
	case providerResolved:
		return entry.value, nil
	case providerResolving:
		cyclePath := append([]string(nil), path...)
		cyclePath = append(cyclePath, entry.displayName)
		return nil, &ResolutionError{
			cause: ErrCycleDetected,
			path:  cyclePath,
		}
	}

	currentPath := append([]string(nil), path...)
	if len(currentPath) == 0 {
		currentPath = []string{entry.componentName, entry.displayName}
	} else {
		currentPath = append(currentPath, entry.displayName)
	}

	entry.state = providerResolving
	previousPath := c.resolutionPath
	c.resolutionPath = currentPath
	value, err := entry.factory(c)
	c.resolutionPath = previousPath
	if err != nil {
		entry.state = providerPending
		if _, ok := err.(*ResolutionError); ok {
			return nil, err
		}
		return nil, &ResolutionError{
			cause: err,
			path:  currentPath,
		}
	}

	entry.value = value
	entry.state = providerResolved
	return value, nil
}

func (c *Container) addBuilder(builder *Builder) error {
	for _, provider := range builder.providers {
		if _, exists := c.providers[provider.key]; exists {
			return fmt.Errorf("composition: duplicate provider %s", provider.key)
		}
		c.providers[provider.key] = provider
		c.providerOrder = append(c.providerOrder, provider)
	}

	c.controllerFactories = append(c.controllerFactories, builder.controllerFactories...)
	c.navItemFactories = append(c.navItemFactories, builder.navItemFactories...)
	c.localeFactories = append(c.localeFactories, builder.localeFactories...)
	c.schemaFactories = append(c.schemaFactories, builder.schemaFactories...)
	c.appletFactories = append(c.appletFactories, builder.appletFactories...)
	c.assetFactories = append(c.assetFactories, builder.assetFactories...)
	c.hashFSFactories = append(c.hashFSFactories, builder.hashFSFactories...)
	c.quickLinkFactories = append(c.quickLinkFactories, builder.quickLinkFactories...)
	c.spotlightFactories = append(c.spotlightFactories, builder.spotlightFactories...)
	if builder.spotlightAgent != nil {
		if c.spotlightAgentFactory != nil {
			return fmt.Errorf("composition: duplicate spotlight agent contribution from %q", builder.descriptor.Name)
		}
		c.spotlightAgentFactory = builder.spotlightAgent
	}
	c.middlewareFactories = append(c.middlewareFactories, builder.middlewareFactories...)
	c.hookFactories = append(c.hookFactories, builder.hookFactories...)
	return nil
}

func (c *Container) instantiateAll() error {
	for _, provider := range c.providerOrder {
		if _, err := c.resolveEntry(provider, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *Container) materialize() error {
	if err := collectInto(c, c.controllerFactories, &c.controllers); err != nil {
		return err
	}
	if err := collectInto(c, c.navItemFactories, &c.navItems); err != nil {
		return err
	}
	if err := collectInto(c, c.localeFactories, &c.locales); err != nil {
		return err
	}
	if err := collectInto(c, c.schemaFactories, &c.graphSchemas); err != nil {
		return err
	}
	if err := collectInto(c, c.appletFactories, &c.applets); err != nil {
		return err
	}
	if err := collectInto(c, c.assetFactories, &c.assets); err != nil {
		return err
	}
	if err := collectInto(c, c.hashFSFactories, &c.hashFSAssets); err != nil {
		return err
	}
	if err := collectInto(c, c.quickLinkFactories, &c.quickLinks); err != nil {
		return err
	}
	if err := collectInto(c, c.spotlightFactories, &c.spotlightProviders); err != nil {
		return err
	}
	if err := collectOneInto(c, c.spotlightAgentFactory, &c.spotlightAgent); err != nil {
		return err
	}
	if err := collectInto(c, c.middlewareFactories, &c.middleware); err != nil {
		return err
	}
	if err := collectInto(c, c.hookFactories, &c.hooks); err != nil {
		return err
	}
	return nil
}

func collectOneInto[T any](container *Container, factory *namedFactory[T], target *T) error {
	if factory == nil {
		return nil
	}
	previousPath := container.resolutionPath
	container.resolutionPath = []string{factory.component, factory.label}
	value, err := factory.factory(container)
	container.resolutionPath = previousPath
	if err != nil {
		return fmt.Errorf("composition: %s contribution for %q: %w", factory.label, factory.component, err)
	}
	*target = value
	return nil
}

func collectInto[T any](container *Container, factories []namedFactory[[]T], target *[]T) error {
	for _, entry := range factories {
		previousPath := container.resolutionPath
		container.resolutionPath = []string{entry.component, entry.label}
		values, err := entry.factory(container)
		container.resolutionPath = previousPath
		if err != nil {
			return fmt.Errorf("composition: %s contribution for %q: %w", entry.label, entry.component, err)
		}
		*target = append(*target, values...)
	}
	return nil
}

func componentActive(descriptor Descriptor, active []Capability) bool {
	if len(active) == 0 || len(descriptor.Capabilities) == 0 {
		return true
	}
	for _, candidate := range descriptor.Capabilities {
		if slices.Contains(active, candidate) {
			return true
		}
	}
	return false
}

func normalizeDescriptor(component Component) Descriptor {
	descriptor := component.Descriptor()
	descriptor.Name = strings.TrimSpace(descriptor.Name)
	if descriptor.Name == "" {
		descriptor.Name = shortTypeName(typeOfComponent(component))
	}
	descriptor.Capabilities = normalizeCapabilities(descriptor.Capabilities)
	descriptor.Requires = normalizeStrings(descriptor.Requires)
	return descriptor
}

func normalizeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func typeOfComponent(component Component) reflect.Type {
	return reflect.TypeOf(component)
}

func formatPath(path []string) string {
	return strings.Join(path, " -> ")
}
