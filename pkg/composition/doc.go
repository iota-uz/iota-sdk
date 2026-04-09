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
// Inside Build, a component calls composition.Provide and the Contribute*
// helpers to register everything it owns. It may also call composition.Use to
// declare dependencies on types provided by other components. No values are
// resolved during Build; the method only populates the builder's registry.
//
// # Providers
//
// composition.Provide[T] registers a typed factory function for T. Factories
// are invoked lazily the first time T is requested within a Container. The same
// resolved value is reused for the lifetime of that Container, so providers are
// effectively singletons per compilation.
//
// # Resolvers
//
// composition.Use[T]() returns a Resolver[T] handle. Holding a Resolver does
// not trigger resolution; it is a declaration of intent. Call
// composition.Resolve[T](container) at actual call time (inside a factory or
// hook) to obtain the concrete value from the container.
//
// # Contribution helpers
//
// The Contribute* family of functions attach well-known extension points to the
// builder without requiring callers to know the underlying registry key:
//
//   - ContributeControllers   - HTTP handler registrars
//   - ContributeNavItems      - sidebar / navigation entries
//   - ContributeLocales       - i18n locale bundles
//   - ContributeHooks         - startup / shutdown hooks
//   - ContributeApplets       - embedded React applet descriptors
//
// # Capabilities
//
// Engine.Compile accepts zero or more capability tags that gate which parts of
// the graph materialize. A component may choose to contribute controllers only
// when CapabilityAPI is present, run migrations only when CapabilityMigrate is
// present, and so on. The defined capabilities are:
//
//   - CapabilityAPI      - HTTP server is active
//   - CapabilityWorker   - background worker process
//   - CapabilityMigrate  - database migration run
//   - CapabilitySeed     - database seed run
//   - CapabilityCLI      - CLI command execution
//
// # Lifecycle
//
// Engine.Compile(ctx, capabilities...) resolves the component graph and returns
// a Container. Engine.Start invokes registered startup hooks in dependency
// order; Engine.Stop invokes shutdown hooks in reverse order.
package composition
