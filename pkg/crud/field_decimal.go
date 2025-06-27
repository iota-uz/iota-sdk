package crud

type DecimalField interface {
	Field

	Precision() int
	Scale() int
	Min() string
	Max() string
}

func NewDecimalField(
	name string,
	opts ...FieldOption,
) DecimalField {
	f := newField(
		name,
		DecimalFieldType,
		opts...,
	).(*field)

	return &decimalField{field: f}
}

type decimalField struct {
	*field
}

func (d *decimalField) Precision() int {
	if val, ok := d.attrs[Precision].(int); ok {
		return val
	}
	return 10
}

func (d *decimalField) Scale() int {
	if val, ok := d.attrs[Scale].(int); ok {
		return val
	}
	return 2
}

func (d *decimalField) Min() string {
	if val, ok := d.attrs[Min].(string); ok {
		return val
	}
	return ""
}

func (d *decimalField) Max() string {
	if val, ok := d.attrs[Max].(string); ok {
		return val
	}
	return ""
}

func (d *decimalField) AsDecimalField() (DecimalField, error) {
	return d, nil
}
