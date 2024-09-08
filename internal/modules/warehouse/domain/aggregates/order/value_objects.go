package order

type StatusEnum string

const (
	Pending  StatusEnum = "pending"
	Complete StatusEnum = "complete"
)

func (s StatusEnum) IsValid() bool {
	return s == Pending || s == Complete
}

func NewStatus(value string) (*Status, error) {
	s := &Status{}
	if err := s.Set(StatusEnum(value)); err != nil {
		return nil, err
	}
	return s, nil
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

func (s Status) Set(value StatusEnum) error {
	if value.IsValid() {
		return ErrInvalidStatus
	}
	s.value = value
	return nil
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
	t := &Type{}
	if err := t.Set(TypeEnum(value)); err != nil {
		return nil, err
	}
	return t, nil
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

func (t Type) Set(value TypeEnum) error {
	if !value.IsValid() {
		return ErrInvalidType
	}
	t.value = value
	return nil
}

func (t Type) String() string {
	return string(t.value)
}
