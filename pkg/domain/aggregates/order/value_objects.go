package order

import "fmt"

type StatusEnum string

const (
	Pending  StatusEnum = "pending"
	Complete StatusEnum = "complete"
)

func (s StatusEnum) IsValid() bool {
	return s == Pending || s == Complete
}

func NewStatus(value string) (*Status, error) {
	status := StatusEnum(value)
	if !status.IsValid() {
		return nil, fmt.Errorf("invalid status: %s", value)
	}
	return &Status{value: status}, nil
}

type Status struct {
	value StatusEnum
}

func (s Status) Is(value StatusEnum) bool {
	return s.value == value
}

func (s Status) Get() StatusEnum {
	return s.value
}

func (s Status) String() string {
	return string(s.value)
}

// --------------------------------------------

type TypeEnum string

const (
	TypeIn  TypeEnum = "in"
	TypeOut TypeEnum = "out"
)

func (t TypeEnum) IsValid() bool {
	return t == TypeIn || t == TypeOut
}

func NewType(value string) (*Type, error) {
	_type := TypeEnum(value)
	if !_type.IsValid() {
		return nil, fmt.Errorf("invalid type: %s", value)
	}
	return &Type{value: _type}, nil
}

type Type struct {
	value TypeEnum
}

func (t Type) Is(value TypeEnum) bool {
	return t.value == value
}

func (t Type) Get() TypeEnum {
	return t.value
}

func (t Type) String() string {
	return string(t.value)
}
