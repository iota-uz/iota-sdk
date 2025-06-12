package crud

type SchemaOption[TEntity any] func(s *schema[TEntity])
type Validator[TEntity any] func(entity TEntity) error

type Schema[TEntity any] interface {
	Name() string
	Fields() Fields
	Mapper() Mapper[TEntity]
	Validators() []Validator[TEntity]
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
