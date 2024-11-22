package transaction

import "fmt"

type TypeEnum string

const (
	IncomeType   TypeEnum = "income"
	ExpenseType  TypeEnum = "expense"
	TransferType TypeEnum = "transfer"
)

func (s TypeEnum) IsValid() bool {
	switch s {
	case IncomeType, ExpenseType, TransferType:
		return true
	}
	return false
}

func NewType(value string) (Type, error) {
	t := TypeEnum(value)
	if !t.IsValid() {
		return Type{}, fmt.Errorf("invalid type: %s", value)
	}
	return Type{value: t}, nil
}

type Type struct {
	value TypeEnum
}

func (p Type) Get() TypeEnum {
	return p.value
}

func (p Type) String() string {
	return string(p.value)
}
