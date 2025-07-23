package crud

import (
	"fmt"

	"github.com/google/uuid"
)

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

func (u *uuidField) Value(value any) FieldValue {
	if value == nil {
		return &fieldValue{
			field: u.field,
			value: nil,
		}
	}

	// Handle UUID type directly
	if uuidVal, ok := value.(uuid.UUID); ok {
		return &fieldValue{
			field: u.field,
			value: uuidVal,
		}
	}

	// Handle [16]uint8 from database scans
	if byteArray, ok := value.([16]uint8); ok {
		uuidVal, err := uuid.FromBytes(byteArray[:])
		if err != nil {
			panic(fmt.Sprintf(
				"invalid UUID bytes for field %q: %v",
				u.name, err,
			))
		}
		return &fieldValue{
			field: u.field,
			value: uuidVal,
		}
	}

	// Handle []uint8 from database scans (alternative format)
	if byteSlice, ok := value.([]uint8); ok {
		if len(byteSlice) == 16 {
			uuidVal, err := uuid.FromBytes(byteSlice)
			if err != nil {
				panic(fmt.Sprintf(
					"invalid UUID bytes for field %q: %v",
					u.name, err,
				))
			}
			return &fieldValue{
				field: u.field,
				value: uuidVal,
			}
		}
	}

	// Invalid type
	panic(fmt.Sprintf(
		"invalid type for UUID field %q: expected uuid.UUID or [16]uint8, got %T",
		u.name, value,
	))
}
