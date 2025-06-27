package crud

type UUIDField interface {
	Field

	Version() int
}

func NewUUIDField(
	name string,
	opts ...FieldOption,
) UUIDField {
	f := newField(
		name,
		UUIDFieldType,
		opts...,
	).(*field)

	return &uuidField{field: f}
}

type uuidField struct {
	*field
}

func (u *uuidField) Version() int {
	if val, ok := u.attrs[UUIDVersion].(int); ok {
		return val
	}
	return 4
}

func (u *uuidField) AsUUIDField() (UUIDField, error) {
	return u, nil
}
