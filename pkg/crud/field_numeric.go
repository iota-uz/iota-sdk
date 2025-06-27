package crud

import "math"

type IntField interface {
	Field

	Min() int64
	Max() int64
	Step() int64
	MultipleOf() int64
}

func NewIntField(
	name string,
	opts ...FieldOption,
) IntField {
	f := newField(
		name,
		IntFieldType,
		opts...,
	).(*field)

	return &intField{field: f}
}

type intField struct {
	*field
}

func (i *intField) Min() int64 {
	if val, ok := i.attrs[Min].(int64); ok {
		return val
	}
	return math.MinInt64
}

func (i *intField) Max() int64 {
	if val, ok := i.attrs[Max].(int64); ok {
		return val
	}
	return math.MaxInt64
}

func (i *intField) Step() int64 {
	if val, ok := i.attrs[Step].(int64); ok {
		return val
	}
	return 1
}

func (i *intField) MultipleOf() int64 {
	if val, ok := i.attrs[MultipleOf].(int64); ok {
		return val
	}
	return 1
}

func (i *intField) AsIntField() (IntField, error) {
	return i, nil
}

type FloatField interface {
	Field

	Min() float64
	Max() float64
	Precision() int
	Step() float64
}

func NewFloatField(
	name string,
	opts ...FieldOption,
) FloatField {
	f := newField(
		name,
		FloatFieldType,
		opts...,
	).(*field)

	return &floatField{field: f}
}

type floatField struct {
	*field
}

func (f *floatField) Min() float64 {
	if val, ok := f.attrs[Min].(float64); ok {
		return val
	}
	return -math.MaxFloat64
}

func (f *floatField) Max() float64 {
	if val, ok := f.attrs[Max].(float64); ok {
		return val
	}
	return math.MaxFloat64
}

func (f *floatField) Precision() int {
	if val, ok := f.attrs[Precision].(int); ok {
		return val
	}
	return 2
}

func (f *floatField) Step() float64 {
	if val, ok := f.attrs[Step].(float64); ok {
		return val
	}
	return 0.01
}

func (f *floatField) AsFloatField() (FloatField, error) {
	return f, nil
}
