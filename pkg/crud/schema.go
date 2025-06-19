package crud

import "context"

type SchemaOption[TEntity any] func(s *schema[TEntity])
type Validator[TEntity any] func(entity TEntity) error

type Hook[TEntity any] func(ctx context.Context, entity TEntity) (TEntity, error)

type Hooks[TEntity any] interface {
	OnCreate() Hook[TEntity]
	OnUpdate() Hook[TEntity]
	OnDelete() Hook[TEntity]
}

type Schema[TEntity any] interface {
	Name() string
	Fields() Fields
	Mapper() Mapper[TEntity]
	Validators() []Validator[TEntity]
	Hooks() Hooks[TEntity]
}

func WithValidators[TEntity any](validators []Validator[TEntity]) SchemaOption[TEntity] {
	return func(s *schema[TEntity]) {
		s.validators = append(s.validators, validators...)
	}
}

func WithValidator[TEntity any](validator Validator[TEntity]) SchemaOption[TEntity] {
	return func(s *schema[TEntity]) {
		s.validators = append(s.validators, validator)
	}
}

func WithCreateHook[TEntity any](hook Hook[TEntity]) SchemaOption[TEntity] {
	return func(s *schema[TEntity]) {
		s.hooks.createHook = hook
	}
}

func WithUpdateHook[TEntity any](hook Hook[TEntity]) SchemaOption[TEntity] {
	return func(s *schema[TEntity]) {
		s.hooks.updateHook = hook
	}
}

func WithDeleteHook[TEntity any](hook Hook[TEntity]) SchemaOption[TEntity] {
	return func(s *schema[TEntity]) {
		s.hooks.deleteHook = hook
	}
}

func NewSchema[TEntity any](
	name string,
	fields Fields,
	mapper Mapper[TEntity],
	opts ...SchemaOption[TEntity],
) Schema[TEntity] {
	s := &schema[TEntity]{
		name:       name,
		fields:     fields,
		mapper:     mapper,
		validators: make([]Validator[TEntity], 0),
		hooks: &hooks[TEntity]{
			createHook: func(ctx context.Context, entity TEntity) (TEntity, error) {
				return entity, nil
			},
			updateHook: func(ctx context.Context, entity TEntity) (TEntity, error) {
				return entity, nil
			},
			deleteHook: func(ctx context.Context, entity TEntity) (TEntity, error) {
				return entity, nil
			},
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

type schema[TEntity any] struct {
	name       string
	fields     Fields
	mapper     Mapper[TEntity]
	validators []Validator[TEntity]
	hooks      *hooks[TEntity]
}

func (s *schema[TEntity]) Name() string {
	return s.name
}

func (s *schema[TEntity]) Fields() Fields {
	return s.fields
}

func (s *schema[TEntity]) Mapper() Mapper[TEntity] {
	return s.mapper
}

func (s *schema[TEntity]) Validators() []Validator[TEntity] {
	return s.validators
}

func (s *schema[TEntity]) Hooks() Hooks[TEntity] {
	return s.hooks
}

type hooks[TEntity any] struct {
	createHook Hook[TEntity]
	updateHook Hook[TEntity]
	deleteHook Hook[TEntity]
}

func (h *hooks[TEntity]) OnCreate() Hook[TEntity] {
	return h.createHook
}

func (h *hooks[TEntity]) OnUpdate() Hook[TEntity] {
	return h.updateHook
}

func (h *hooks[TEntity]) OnDelete() Hook[TEntity] {
	return h.deleteHook
}
