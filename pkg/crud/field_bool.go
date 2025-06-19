package crud

type BoolField interface {
	Field

	DefaultValue() bool
	TrueLabel() string
	FalseLabel() string
}

func NewBoolField(
	name string,
	opts ...FieldOption,
) BoolField {
	f := newField(
		name,
		BoolFieldType,
		opts...,
	).(*field)

	return &boolField{field: f}
}

type boolField struct {
	*field
}

func (b *boolField) DefaultValue() bool {
	if val, ok := b.attrs[DefaultValue].(bool); ok {
		return val
	}
	return false
}

func (b *boolField) TrueLabel() string {
	if val, ok := b.attrs[TrueLabel].(string); ok {
		return val
	}
	return ""
}

func (b *boolField) FalseLabel() string {
	if val, ok := b.attrs[FalseLabel].(string); ok {
		return val
	}
	return ""
}

func (b *boolField) AsBoolField() (BoolField, error) {
	return b, nil
}
