package crud_v2

import "github.com/iota-uz/iota-sdk/pkg/eventbus"

type BuilderOption[TEntity any] func(b *builder[TEntity])

type Builder[TEntity any] interface {
	Schema() Schema[TEntity]
	Repository() Repository[TEntity]
	Service() Service[TEntity]
}

func WithRepository[TEntity any](repository Repository[TEntity]) BuilderOption[TEntity] {
	return func(b *builder[TEntity]) {
		b.repository = repository
	}
}

func WithService[TEntity any](service Service[TEntity]) BuilderOption[TEntity] {
	return func(b *builder[TEntity]) {
		b.service = service
	}
}

func NewBuilder[TEntity any](
	schema Schema[TEntity],
	publisher eventbus.EventBus,
	opts ...BuilderOption[TEntity],
) Builder[TEntity] {
	b := &builder[TEntity]{
		schema:    schema,
		publisher: publisher,
	}

	for _, opt := range opts {
		opt(b)
	}

	if b.repository == nil {
		b.repository = DefaultRepository[TEntity](schema)
	}
	if b.service == nil {
		b.service = DefaultService[TEntity](schema, b.repository, b.publisher)
	}

	return b
}

type builder[TEntity any] struct {
	schema     Schema[TEntity]
	repository Repository[TEntity]
	service    Service[TEntity]
	publisher  eventbus.EventBus
}

func (b *builder[TEntity]) Schema() Schema[TEntity] {
	return b.schema
}

func (b *builder[TEntity]) Repository() Repository[TEntity] {
	return b.repository
}

func (b *builder[TEntity]) Service() Service[TEntity] {
	return b.service
}
