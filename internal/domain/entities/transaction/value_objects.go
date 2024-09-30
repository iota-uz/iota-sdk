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
	s := Type{}
	if err := s.Set(TypeEnum(value)); err != nil {
		return Type{}, err
	}
	return s, nil
}

type Type struct {
	value TypeEnum
}

func (p Type) Get() TypeEnum {
	return p.value
}

func (p Type) Set(value TypeEnum) error {
	if !value.IsValid() {
		return InvalidType
	}
	p.value = value
	return nil
}

func (p Type) String() string {
	return string(p.value)
}
