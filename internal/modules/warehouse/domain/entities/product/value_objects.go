package product

type StatusEnum string

const (
	InStock       StatusEnum = "in_stock"
	InDevelopment StatusEnum = "in_development"
	Approved      StatusEnum = "approved"
)

func (s StatusEnum) IsValid() bool {
	return s == InStock || s == InDevelopment || s == Approved
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

func (p Status) Get() StatusEnum {
	return p.value
}

func (p Status) Set(value StatusEnum) error {
	if !value.IsValid() {
		return InvalidStatus
	}
	p.value = value
	return nil
}

func (p Status) String() string {
	return string(p.value)
}
