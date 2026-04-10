// Package composition provides the component graph, container, and lifecycle
// orchestration used to assemble SDK applications.
//
// # Mental model
//
// An application is a graph of Components. Each Component declares what it
// provides and what it needs. The Engine compiles that graph into a Container
// and drives the hook lifecycle.
//
// # Components
//
// A Component implements a single method:
//
//	Build(builder *Builder) error
//
// Inside Build a component calls composition.Provide (and its ProvideFunc,
// ProvideFuncAs, ProvideAs siblings) together with the Add* and Contribute*
// helpers to register everything it owns. No values are resolved during
// Build; the method only populates the builder's registry.
//
// # Providers
//
// composition.Provide[T] registers either an eager value or a closure factory
// for T. Factories are invoked lazily the first time T is requested within a
// Container; the same resolved value is reused for the lifetime of that
// Container, so providers are effectively singletons per compilation.
//
// Wide-dependency constructors (services with many typed arguments) should
// prefer the reflection injector:
//
//	composition.ProvideFunc(builder, services.NewPaymentService)
//
// The injector inspects the constructor's signature, resolves each parameter
// from the container by type, and registers the provider under the return
// type. ProvideFuncAs[I] additionally bridges the concrete return type to an
// interface key I so consumers can depend on the interface. Both helpers
// reject variadic constructors at registration time to prevent silent option
// drops — wrap variadic functions in a non-variadic adapter.
//
// # Static contributions
//
// Components that only need to attach constant data (locale bundles, nav
// items, quick links, hashfs assets) should use the Add* helpers:
//
//	composition.AddLocales(builder, &LocaleFiles)
//	composition.AddNavItems(builder, NavItems...)
//	composition.AddQuickLinks(builder, spotlight.NewQuickLink(...))
//	composition.AddHashFS(builder, assets.HashFS)
//
// These are zero-closure equivalents of the corresponding Contribute* factories
// and make the Build method read as a declarative manifest.
//
// # Contribution helpers
//
// For contributions that need to resolve services from the container, the
// Contribute* family attaches extension points without requiring callers to
// know the underlying registry key:
//
//   - ContributeControllers / ContributeControllersFunc
//   - ContributeNavItems
//   - ContributeLocales
//   - ContributeHooks
//   - ContributeApplets
//   - ContributeEventHandler / ContributeEventHandlerFunc
//
// The *Func variants accept a reflection-injected constructor whose parameter
// types are resolved from the container at materialization time.
//
// # Auto-providers
//
// Engine.Compile registers providers for the core services already available
// on the build context: *pgxpool.Pool, eventbus.EventBus, *i18n.Bundle,
// *logrus.Logger, application.Application, spotlight.Service,
// application.Huber, and *configuration.Configuration. Components can take
// these as typed parameters in ProvideFunc / ContributeControllersFunc
// constructors, or call composition.Resolve from inside a Contribute*
// closure. User-registered providers for the same key take precedence over
// the auto-provided value.
//
// # Capabilities
//
// Engine.Compile accepts zero or more capability tags that gate which parts of
// the graph materialize. A component may choose to contribute controllers only
// when CapabilityAPI is present, or worker-only hooks when CapabilityWorker is
// present. The defined capabilities are:
//
//   - CapabilityAPI      - HTTP server is active
//   - CapabilityWorker   - background worker process
//
// # Lifecycle
//
// Engine.Compile(buildCtx, capabilities...) resolves the component graph and
// returns a Container. The package-level Start(ctx, container) function runs
// every registered hook's Start closure in registration order; each Start may
// return a StopFn that Stop(ctx, container) invokes in reverse order during
// shutdown.
//
// Hooks themselves register via ContributeHooks with the signature
// Start(ctx) (StopFn, error). The Start closure captures any local state it
// needs to clean up and returns the StopFn that will later tear that state
// down. Start and Stop are not safe for concurrent use; callers must serialize
// lifecycle operations.
package composition
