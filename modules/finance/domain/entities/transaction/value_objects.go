package transaction

import "fmt"

type Type string

const (
	Deposit    Type = "DEPOSIT"
	Withdrawal Type = "WITHDRAWAL"
	Transfer   Type = "TRANSFER"
)

func (s Type) IsValid() bool {
	switch s {
	case Deposit, Withdrawal, Transfer:
		return true
	}
	return false
}

func NewType(value string) (Type, error) {
	t := Type(value)
	if !t.IsValid() {
		return "", fmt.Errorf("invalid type: %s", value)
	}
	return t, nil
}
