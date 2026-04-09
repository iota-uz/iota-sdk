// Package crud provides this package.
package crud

import "github.com/iota-uz/iota-sdk/pkg/composition"

// ProvideBuilder registers a typed CRUD builder in the composition container.
func ProvideBuilder[TEntity any](
	builder *composition.Builder,
	schema Schema[TEntity],
	opts ...BuilderOption[TEntity],
) Builder[TEntity] {
	registry := NewBuilder(schema, builder.Context().EventPublisher(), opts...)
	composition.Provide[Builder[TEntity]](builder, registry)
	return registry
}

// ProvideExistingBuilder exposes a prebuilt CRUD builder to composition.
func ProvideExistingBuilder[TEntity any](
	builder *composition.Builder,
	registry Builder[TEntity],
) Builder[TEntity] {
	composition.Provide[Builder[TEntity]](builder, registry)
	return registry
}
