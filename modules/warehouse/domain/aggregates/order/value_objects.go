package order

import "fmt"

type Status string

const (
	Pending  Status = "pending"
	Complete Status = "complete"
)

func (s Status) IsValid() bool {
	return s == Pending || s == Complete
}

func NewStatus(value string) (Status, error) {
	status := Status(value)
	if !status.IsValid() {
		return "", fmt.Errorf("invalid status: %s", value)
	}
	return status, nil
}

// --------------------------------------------

type Type string

const (
	TypeIn  Type = "in"
	TypeOut Type = "out"
)

func (t Type) IsValid() bool {
	return t == TypeIn || t == TypeOut
}

func NewType(value string) (Type, error) {
	t := Type(value)
	if !t.IsValid() {
		return "", fmt.Errorf("invalid type: %s", value)
	}
	return t, nil
}
