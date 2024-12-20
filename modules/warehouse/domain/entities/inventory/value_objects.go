package inventory

import "fmt"

type StatusEnum string
type TypeEnum string

const (
	Success    StatusEnum = "success"
	Incomplete StatusEnum = "incomplete"
	Failed     StatusEnum = "failed"
)

const (
	Full    TypeEnum = "full"
	Partial TypeEnum = "partial"
)

func (s StatusEnum) IsValid() bool {
	return s == Success || s == Incomplete || s == Failed
}

func (t TypeEnum) IsValid() bool {
	return t == Full || t == Partial
}

func NewStatus(value string) (Status, error) {
	status := StatusEnum(value)
	if !status.IsValid() {
		return Status{}, fmt.Errorf("invalid status: %s", value)
	}
	return Status{value: status}, nil
}

func NewType(value string) (Type, error) {
	typ := TypeEnum(value)
	if !typ.IsValid() {
		return Type{}, fmt.Errorf("invalid status: %s", value)
	}
	return Type{value: typ}, nil
}

type Status struct {
	value StatusEnum
}

func (s Status) Get() StatusEnum {
	return s.value
}

func (s Status) String() string {
	return string(s.value)
}

type Type struct {
	value TypeEnum
}

func (t Type) Get() TypeEnum {
	return t.value
}

func (t Type) String() string {
	return string(t.value)
}
