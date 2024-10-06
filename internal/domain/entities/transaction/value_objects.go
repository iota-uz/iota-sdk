package transaction

type TypeEnum string

// income, expense, transfer
const (
	Income   TypeEnum = "income"
	Expense  TypeEnum = "expense"
	Transfer TypeEnum = "transfer"
)

func (s TypeEnum) IsValid() bool {
	switch s {
	case Income, Expense, Transfer:
		return true
	}
	return false
}

func NewType(value string) (Type, error) {
	t := TypeEnum(value)
	if !t.IsValid() {
		return Type{}, InvalidType
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
