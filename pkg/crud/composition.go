// Package crud provides this package.
package crud

import "github.com/iota-uz/iota-sdk/pkg/composition"

// ProvideBuilder registers a typed CRUD builder in both composition and the
// legacy application service container for callers that still use app.Service.
func ProvideBuilder[TEntity any](
	builder *composition.Builder,
	schema Schema[TEntity],
	opts ...BuilderOption[TEntity],
) Builder[TEntity] {
	registry := NewBuilder(schema, builder.Context().App.EventPublisher(), opts...)
	builder.Context().App.RegisterServices(registry)
	composition.Provide[Builder[TEntity]](builder, registry)
	return registry
}

// ProvideExistingBuilder exposes a prebuilt CRUD builder to composition while
// keeping the legacy application service lookup path available.
func ProvideExistingBuilder[TEntity any](
	builder *composition.Builder,
	registry Builder[TEntity],
) Builder[TEntity] {
	builder.Context().App.RegisterServices(registry)
	composition.Provide[Builder[TEntity]](builder, registry)
	return registry
}
